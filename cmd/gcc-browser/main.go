package main

import (
	"bufio"
	"bytes"
	"context"
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
	daemonPath string
}

func NewOrchestrator() *Orchestrator {
	// Find the daemon binary path next to the current executable
	ex, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to find executable path: %v", err)
	}
	dir := filepath.Dir(ex)
	return &Orchestrator{
		daemonPath: filepath.Join(dir, "gcc-daemon"),
	}
}

// SpawnProcess starts a child daemon and returns the gRPC connection
func (o *Orchestrator) SpawnProcess(role string) (*grpc.ClientConn, error) {
	cmd := exec.Command(o.daemonPath, "--role", role, "--port", "0")

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
		html := `
		<html>
		  <body>
			<div id="content">
			  <p class="highlight">Hello Navigation Pipeline!</p>
			</div>
		  </body>
		</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
}

func main() {
	log.Println("[GCC Orchestrator] Booting Navigation Pipeline...")

	// 0. Spin up local mock HTTP server
	ts := startMockServer()
	defer ts.Close()

	// Orchestrator initialization
	orchestrator := NewOrchestrator()
	if _, err := os.Stat(orchestrator.daemonPath); os.IsNotExist(err) {
		orchestrator.daemonPath = "gcc-daemon"
	}

	// 1. Spawn Daemons
	netConn, _ := orchestrator.SpawnProcess("network")
	parserConn, _ := orchestrator.SpawnProcess("parser")
	renderConn, _ := orchestrator.SpawnProcess("renderer")
	jsConn, _ := orchestrator.SpawnProcess("javascript")

	defer func() {
		if netConn != nil {
			netConn.Close()
		}
		if parserConn != nil {
			parserConn.Close()
		}
		if renderConn != nil {
			renderConn.Close()
		}
		if jsConn != nil {
			jsConn.Close()
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

	var renderAdapter *ipc.RendererIPCAdapter
	if renderConn != nil {
		renderAdapter = ipc.NewRendererIPCAdapter(api.NewRendererServiceClient(renderConn))
	}

	var jsAdapter *ipc.JavascriptIPCAdapter
	if jsConn != nil {
		jsAdapter = ipc.NewJavascriptIPCAdapter(api.NewJavaScriptServiceClient(jsConn))
	}

	log.Println("[Orchestrator] All IPC Daemons Spawning Completed.")

	// 2. Fetch Document (Network)
	log.Printf("[Orchestrator] Fetching %s ...", ts.URL)
	var htmlBody io.Reader
	if netAdapter != nil {
		resp, err := netAdapter.Fetch(context.Background(), ts.URL, gcc.FetchOptions{Method: "GET"})
		if err != nil {
			log.Fatalf("Fetch failed: %v", err)
		}

		buf := new(bytes.Buffer)
		io.Copy(buf, resp.Body)
		htmlBody = buf
	} else {
		// Fallback for tests if daemon missing
		htmlBody = bytes.NewReader([]byte(`<html><window><viewport id="content"><text>Fallback Pipeline Mode</text></viewport></window></html>`))
	}

	// 3. Parse Document (Parser)
	log.Println("[Orchestrator] Parsing HTML structure...")
	var dom *gcc.DOMTree
	var css *gcc.CSSOMTree
	if parserAdapter != nil {
		// Mock a CSS inline fetch for this milestone
		cssBody := bytes.NewReader([]byte(`
			window { background-color: #E5E5E5; width: 800px; height: 600px; }
			#content { background-color: #FFFFFF; width: 700px; height: 500px; }
			.highlight { color: #FF0000; }
		`))

		var err error
		dom, err = parserAdapter.ParseHTML(htmlBody)
		if err != nil {
			log.Fatalf("ParseHTML failed: %v", err)
		}

		css, err = parserAdapter.ParseCSS(cssBody)
		if err != nil {
			log.Fatalf("ParseCSS failed: %v", err)
		}
	} else {
		dom, css = createMockUI()
	}

	// 4. GUI Rendering Loop (Hardware Accelerated)
	log.Println("[Orchestrator] Initializing Hardware GPU Canvas...")
	canvas, err := render.NewOpenGLCanvas(800, 600, "Go-Chromium-Core (GCC)")
	if err != nil {
		log.Fatalf("Failed to initialize OpenGL canvas: %v", err)
	}
	defer canvas.Terminate()

	var currentLayout *gcc.LayoutTree
	canvas.SetOnMouseClick(func(x, y float64) {
		if currentLayout != nil {
			hit := render.HitTest(currentLayout, x, y)
			if hit != nil && hit.Node != nil {
				log.Printf("[Orchestrator Event] Clicked Node: %s", hit.Node.Type)
				if jsAdapter != nil {
					jsAdapter.DispatchEvent(hit.Node.Type, "click", "{}")
				}
			}
		}
	})

	// Fallback to local render stack if IPC daemon missing
	var layoutEngine gcc.RenderEngine = render.NewRenderStack()
	if renderAdapter != nil {
		layoutEngine = renderAdapter
	}

	log.Println("[Orchestrator] Entering hardware rendering loop. Close window to exit.")
	for !canvas.ShouldClose() {
		layoutTree, err := layoutEngine.ComputeLayout(dom, css)
		if err == nil && layoutTree != nil {
			layoutEngine.Paint(layoutTree, canvas)
			currentLayout = layoutTree
		}
	}
}
