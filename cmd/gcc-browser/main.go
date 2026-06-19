package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-chromium-core/gcc"
	"github.com/go-chromium-core/gcc/api"
	"github.com/go-chromium-core/gcc/internal/ipc"
	"github.com/go-chromium-core/gcc/internal/sandbox"
	"github.com/go-chromium-core/gcc/pkg/render"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Orchestrator manages the IPC lifecycle.
type Orchestrator struct {
	daemonPath    string
	activeDaemons map[string]string // Tracks role -> target address (ip:port)
}

func NewOrchestrator() *Orchestrator {
	// Find the daemon binary path next to the current executable
	ex, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to find executable path: %v", err)
	}
	dir := filepath.Dir(ex)
	return &Orchestrator{
		daemonPath:    filepath.Join(dir, "gcc-daemon"),
		activeDaemons: make(map[string]string),
	}
}

// GetDaemonAddress retrieves the dynamically assigned localhost port address for a pooled daemon role
func (o *Orchestrator) GetDaemonAddress(role string) string {
	return o.activeDaemons[role]
}

// SpawnProcess starts a child daemon and returns the gRPC connection
func (o *Orchestrator) SpawnProcess(role string) (*grpc.ClientConn, error) {
	return o.SpawnProcessWithArgs(role, nil)
}

func (o *Orchestrator) SpawnProcessWithArgs(role string, extraArgs []string) (*grpc.ClientConn, error) {
	args := []string{"--role", role, "--port", "0"}
	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	cmd := exec.Command(o.daemonPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout: %w", err)
	}

	cmd.Stderr = os.Stderr

	// Determine sandbox policy based on the child daemon role
	policy := sandbox.PolicyStrict
	if role == "network" {
		policy = sandbox.PolicyNetwork
	}

	// Apply OS-level sandbox containerization before starting the process
	if sandbox.Builder != nil {
		if err := sandbox.Builder.Configure(cmd, policy); err != nil {
			return nil, fmt.Errorf("failed to configure sandbox: %w", err)
		}
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start daemon: %w", err)
	}

	// Read the random port assigned by the OS from stdout
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read port from daemon: %w", err)
	}

	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "PORT:") {
		return nil, fmt.Errorf("unexpected output from daemon: %s", line)
	}

	port := strings.TrimPrefix(line, "PORT:")
	target := fmt.Sprintf("127.0.0.1:%s", port)

	o.activeDaemons[role] = target

	log.Printf("[Orchestrator] Connected to %s process at %s", role, target)

	// In a real implementation we would manage process lifecycles properly and kill on exit.
	// We'll run the stdout consumer in a goroutine to prevent the pipe from blocking.
	go func() {
		bufio.NewReader(stdout).WriteTo(os.Stdout)
		cmd.Wait()
	}()

	conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial daemon: %w", err)
	}

	return conn, nil
}

func startMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logo.png" {
			imgData, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAADIAAAAyCAYAAAAeP4ixAAAABHNCSVQICAgIfAhkiAAAAH9JREFUaIHt0sENgCAQRMFt5wD6b4Y2bIAGMMLs0z0h380551z9L326e+7u/x6yK8jL8fExGvKyw1rXy8vLZL1er9fr9Xq9Xq/X6/V6vV6v1+v1er1er9fr9Xq9Xq/X6/V6vV6v1+v1er1er9fr9Xq9Xq/X6/V6vV6v1+v1er1er9fr9V67t2yq+hNnO3QAAAAASUVORK5CYII=")
			w.Header().Set("Content-Type", "image/png")
			w.Write(imgData)
			return
		}

		if r.URL.Path == "/styles.css" {
			css := `
				body { background-color: #EEEEEE; width: 800px; height: 600px; }
				#content { background-color: #FFFFFF; width: 600px; height: 400px; }
				.highlight { color: #FF0000; font-size: 24px; }
				img { width: 50px; height: 50px; display: block; }
			`
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte(css))
			return
		}

		html := `
		<html>
		  <head>
		    <link rel="stylesheet" href="/styles.css">
		  </head>
		  <body>
			<div id="content">
			  <p class="highlight">E2E Remote CSS Navigation Pipeline!</p>
			  <img src="/logo.png">
			</div>
		  </body>
		</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
}

// fetchAndParse handles the navigation pipeline for a specific tab
func fetchAndParse(ctx *BrowserContext, tab *Tab, ts *httptest.Server) {
	log.Printf("[Orchestrator] Fetching %s ...", tab.URL)
	var htmlBody io.Reader
	fetchUrl := tab.URL
	if !strings.HasPrefix(fetchUrl, "http://") && !strings.HasPrefix(fetchUrl, "https://") {
		fetchUrl = "http://" + fetchUrl
	}

	if ctx.NetworkAdapter != nil {
		resp, err := ctx.NetworkAdapter.Fetch(context.Background(), fetchUrl, gcc.FetchOptions{Method: "GET"})
		if err != nil {
			log.Fatalf("Fetch failed: %v", err)
		}

		buf := new(bytes.Buffer)
		io.Copy(buf, resp.Body)
		htmlBody = buf
	} else {
		htmlBody = bytes.NewReader([]byte(fmt.Sprintf(`<html><window><viewport id="content"><text>Fallback Tab: %s</text></viewport></window></html>`, tab.URL)))
	}

	log.Println("[Orchestrator] Parsing HTML structure...")
	if ctx.ParserAdapter != nil {
		var err error
		tab.DOM, err = ctx.ParserAdapter.ParseHTML(htmlBody)
		if err != nil {
			log.Fatalf("ParseHTML failed: %v", err)
		}

		cssCombined := ""
		var imageAssets map[string][]byte
		imageAssets = make(map[string][]byte)

		for _, res := range tab.DOM.Resources {
			resUrl := res
			if strings.HasPrefix(res, "/") {
				resUrl = ts.URL + res
			}

			if strings.HasSuffix(res, ".css") {
				if ctx.NetworkAdapter != nil {
					resResp, netErr := ctx.NetworkAdapter.Fetch(context.Background(), resUrl, gcc.FetchOptions{Method: "GET"})
					if netErr == nil {
						resBuf := new(bytes.Buffer)
						io.Copy(resBuf, resResp.Body)
						cssCombined += resBuf.String() + "\n"
					}
				}
			} else if strings.HasSuffix(res, ".png") || strings.HasSuffix(res, ".jpg") {
				if ctx.NetworkAdapter != nil {
					resResp, netErr := ctx.NetworkAdapter.Fetch(context.Background(), resUrl, gcc.FetchOptions{Method: "GET"})
					if netErr == nil {
						resBuf := new(bytes.Buffer)
						io.Copy(resBuf, resResp.Body)
						imageAssets[res] = resBuf.Bytes()
					}
				}
			}
		}

		if cssCombined == "" {
			cssCombined = "body { background-color: #E5E5E5; width: 800px; height: 600px; }"
		}

		tab.CSS, err = ctx.ParserAdapter.ParseCSS(bytes.NewReader([]byte(cssCombined)))
		if err != nil {
			log.Fatalf("ParseCSS failed: %v", err)
		}

		// Inject base64 images into DOM manually for this POC
		var injectImages func(node *gcc.DOMNode)
		injectImages = func(node *gcc.DOMNode) {
			if node == nil {
				return
			}
			if node.Type == "img" {
				for i, attr := range node.Attr {
					if src, ok := attr["src"]; ok {
						if imgData, exists := imageAssets[src]; exists {
							node.Attr[i]["_img_data"] = base64.StdEncoding.EncodeToString(imgData)
						}
					}
				}
			}
			for _, child := range node.Children {
				injectImages(child)
			}
		}
		injectImages(tab.DOM.Root)

	} else {
		log.Fatalf("Parser IPC daemon is missing, unable to boot pipeline.")
	}

	tab.IsDirty = true
}

func buildBrowserUI(ctx *BrowserContext, urlBuffer string, urlFocused bool) (*gcc.DOMTree, *gcc.CSSOMTree) {
	// Construct the browser Chrome (Tab bar) wrapping the active tab's content

	tabNodes := make([]*gcc.DOMNode, 0)
	for i, tab := range ctx.Tabs {
		class := "tab"
		if i == ctx.ActiveTab {
			class = "tab active"
		}
		tabNodes = append(tabNodes, &gcc.DOMNode{
			Type: "tab",
			Attr: []map[string]string{{"class": class}, {"data-id": fmt.Sprintf("%d", i)}},
			Data: tab.Title,
		})
	}

	activeTab := ctx.GetActiveTab()
	var viewportContent *gcc.DOMNode
	if activeTab != nil && activeTab.DOM != nil {
		viewportContent = activeTab.DOM.Root
	} else {
		viewportContent = &gcc.DOMNode{Type: "text", Data: "Loading..."}
	}

	dom := &gcc.DOMTree{
		Root: &gcc.DOMNode{
			Type: "window",
			Children: []*gcc.DOMNode{
				{
					Type:     "tab-bar",
					Attr:     []map[string]string{{"id": "tabs"}},
					Children: tabNodes,
				},
				{
					Type: "url-bar",
					Attr: []map[string]string{{"id": "url"}},
					Children: []*gcc.DOMNode{
						{Type: "text", Data: urlBuffer},
					},
				},
				{
					Type:     "viewport",
					Attr:     []map[string]string{{"id": "content"}},
					Children: []*gcc.DOMNode{viewportContent},
				},
			},
		},
	}

	cssCombined := `
		window { background-color: #E5E5E5; width: 800px; height: 600px; display: block; }
		#tabs { background-color: #CCCCCC; width: 800px; height: 40px; display: flex; flex-direction: row; }
		tab { background-color: #999999; width: 150px; height: 40px; }
		.active { background-color: #FFFFFF; width: 150px; height: 40px; }
		#url { background-color: #FFFFFF; width: 800px; height: 30px; display: block; border-bottom: 2px solid #333; }
		#content { background-color: #FFFFFF; width: 800px; height: 530px; display: block; }
	`

	css := &gcc.CSSOMTree{}
	if ctx.ParserAdapter != nil {
		var err error
		css, err = ctx.ParserAdapter.ParseCSS(bytes.NewReader([]byte(cssCombined)))
		if err != nil {
			log.Fatalf("ParseCSS failed: %v", err)
		}

		// Merge Active Tab CSS into Chrome CSS
		if activeTab != nil && activeTab.CSS != nil {
			css.Rules = append(css.Rules, activeTab.CSS.Rules...)
		}
	}

	return dom, css
}

// applyMutation recursively traverses the DOM to apply styles
func applyMutation(node *gcc.DOMNode, mutation ipc.DOMMutation) bool {
	if node == nil {
		return false
	}

	// Assuming Node.Type is the ID for this Mock
	// (In a real engine, we check node.Attr["id"])
	if node.Type == mutation.NodeID {
		// Ensure styles are applied as a class/id proxy or inline style
		found := false
		for _, attr := range node.Attr {
			if _, ok := attr["style"]; ok {
				// Naive style append for mock
				attr["style"] = attr["style"] + ";" + mutation.Property + ":" + mutation.Value
				found = true
				break
			}
		}
		if !found {
			node.Attr = append(node.Attr, map[string]string{"style": mutation.Property + ":" + mutation.Value})
		}
		return true // Applied
	}

	for _, child := range node.Children {
		if applyMutation(child, mutation) {
			return true
		}
	}
	return false
}
func main() {
	log.Println("[GCC Orchestrator] Booting Multi-Tab Pipeline...")

	ts := startMockServer()
	defer ts.Close()

	orchestrator := NewOrchestrator()
	if _, err := os.Stat(orchestrator.daemonPath); os.IsNotExist(err) {
		orchestrator.daemonPath = "gcc-daemon"
	}

	// 1. Spawn Shared Daemons (Stateless pool)
	netConn, _ := orchestrator.SpawnProcess("network")
	parserConn, _ := orchestrator.SpawnProcess("parser")
	defer func() {
		if netConn != nil {
			netConn.Close()
		}
		if parserConn != nil {
			parserConn.Close()
		}
	}()

	var netAdapter *ipc.NetworkIPCAdapter
	if netConn != nil {
		netAdapter = ipc.NewNetworkIPCAdapter(api.NewNetworkServiceClient(netConn))
	}
	var parserAdapter *ipc.ParserIPCAdapter
	if parserConn != nil {
		parserAdapter = ipc.NewParserIPCAdapter(api.NewParserServiceClient(parserConn))
	}

	// 2. Initialize Browser Context & Tabs
	browserCtx := NewBrowserContext(orchestrator, netAdapter, parserAdapter)

	tab1 := browserCtx.CreateTab(ts.URL, "Tab 1")
	tab2 := browserCtx.CreateTab(ts.URL+"/tab2", "Tab 2")

	// Fetch both tabs concurrently (Optimized resource loading)
	go fetchAndParse(browserCtx, tab1, ts)
	go fetchAndParse(browserCtx, tab2, ts)

	// 3. GUI Rendering Loop (Hardware Accelerated)
	log.Println("[Orchestrator] Initializing Hardware GPU Canvas...")
	canvas, err := render.NewOpenGLCanvas(800, 600, "Go-Chromium-Core (GCC)")
	if err != nil {
		log.Fatalf("Failed to initialize OpenGL canvas: %v", err)
	}
	defer canvas.Terminate()

	var chromeLayout *gcc.LayoutTree
	urlFocused := false
	urlBuffer := ""
	if activeTab := browserCtx.GetActiveTab(); activeTab != nil {
		urlBuffer = activeTab.URL
	}

	// Track Chrome UI updates
	needsChromeUpdate := true
	_ = needsChromeUpdate

	canvas.SetOnMouseClick(func(x, y float64) {
		if chromeLayout != nil {
			hit := render.HitTest(chromeLayout, x, y)
			if hit != nil && hit.Node != nil {
				log.Printf("[Orchestrator Event] Clicked Node: %s", hit.Node.Type)

				// Handle URL Bar Focus
				if hit.Node.Type == "url-bar" || hit.Node.Type == "text" && hit.Node.Data == urlBuffer {
					urlFocused = true
					needsChromeUpdate = true
					return
				} else {
					urlFocused = false
					needsChromeUpdate = true
				}

				// Handle Tab Switching
				if hit.Node.Type == "tab" {
					for _, attr := range hit.Node.Attr {
						if idStr, ok := attr["data-id"]; ok {
							var id int
							fmt.Sscanf(idStr, "%d", &id)
							if id >= 0 && id < len(browserCtx.Tabs) {
								browserCtx.ActiveTab = id
								urlBuffer = browserCtx.Tabs[id].URL // Reset URL buffer to new active tab
								needsChromeUpdate = true
								return
							}
						}
					}
				}
			}
		}
	})

	canvas.SetOnChar(func(char rune) {
		if urlFocused {
			urlBuffer += string(char)
			needsChromeUpdate = true
		}
	})

	canvas.SetOnKey(func(key int, action int) {
		if urlFocused && action == 1 { // 1 == Press (glfw.Press)
			if key == 259 && len(urlBuffer) > 0 { // 259 == Backspace (glfw.KeyBackspace)
				urlBuffer = urlBuffer[:len(urlBuffer)-1]
				needsChromeUpdate = true
			} else if key == 257 { // 257 == Enter (glfw.KeyEnter)
				urlFocused = false
				activeTab := browserCtx.GetActiveTab()
				if activeTab != nil {
					activeTab.URL = urlBuffer
					go fetchAndParse(browserCtx, activeTab, ts)
				}
				needsChromeUpdate = true
			}
		}
	})

	var localLayoutEngine gcc.RenderEngine = render.NewRenderStack()

	log.Println("[Orchestrator] Entering hardware rendering loop. Close window to exit.")
	for !canvas.ShouldClose() {
		activeTab := browserCtx.GetActiveTab()

		// 10.3 Phase 10: Poll JS Daemon for DOM Mutations
		if activeTab != nil && activeTab.JSAdapter != nil && activeTab.DOM != nil {
			mutations, err := activeTab.JSAdapter.PollMutations()
			if err == nil && len(mutations) > 0 {
				log.Printf("[Orchestrator] Received %d DOM mutations from JS Daemon", len(mutations))
				for _, mut := range mutations {
					if applyMutation(activeTab.DOM.Root, mut) {
						activeTab.IsDirty = true
						needsChromeUpdate = true
					}
				}
			}
		}

		// If tab layout is dirty or Chrome changed (active tab switched)
		// We rebuild the Chrome UI tree and re-layout
		chromeDom, chromeCss := buildBrowserUI(browserCtx, urlBuffer, urlFocused)

		// Compute layout for the entire browser window using the local Render Engine
		// In a full implementation, the inner viewport is computed by the IPC RenderAdapter,
		// and the Orchestrator composites the frames. For this POC, we compute the Chrome locally.
		layoutTree, err := localLayoutEngine.ComputeLayout(chromeDom, chromeCss)
		if err == nil && layoutTree != nil {
			localLayoutEngine.Paint(layoutTree, canvas)
			chromeLayout = layoutTree

			if activeTab != nil {
				activeTab.IsDirty = false
			}
		}
	}
}
