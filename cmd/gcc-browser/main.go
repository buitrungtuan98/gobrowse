package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

func main() {
	log.Println("[GCC Orchestrator] Booting...")

	// Orchestrator initialization
	orchestrator := NewOrchestrator()

	// Override path to gcc-daemon if not built in place
	if _, err := os.Stat(orchestrator.daemonPath); os.IsNotExist(err) {
		orchestrator.daemonPath = "gcc-daemon"
	}

	// Attempt to spawn the JS runtime daemon
	log.Println("[Orchestrator] Spawning JS Runtime...")
	jsConn, err := orchestrator.SpawnProcess("javascript")
	if err != nil {
		log.Printf("[Orchestrator] Failed to spawn JS Runtime (Did you compile gcc-daemon?): %v", err)
	} else {
		defer jsConn.Close()
		log.Println("[Orchestrator] Successfully established IPC with JS Runtime.")
	}

	time.Sleep(1 * time.Second)

	// Phase 5: Hardware Paint GUI Initialization
	log.Println("[Orchestrator] Initializing Hardware GPU Canvas...")

	canvas, err := render.NewOpenGLCanvas(800, 600, "Go-Chromium-Core (GCC)")
	if err != nil {
		log.Fatalf("Failed to initialize OpenGL canvas: %v", err)
	}
	defer canvas.Terminate()

	// Build the mock UI tree
	uiDom, uiCss := createMockUI()

	// Create a local Layout Engine to compute the UI Geometry
	layoutEngine := render.NewRenderStack()

	log.Println("[Orchestrator] Entering hardware rendering loop. Close window to exit.")

	// Phase 6.2/6.3: JS Event Bridge
	var jsAdapter *ipc.JavascriptIPCAdapter
	if jsConn != nil {
		jsAdapter = ipc.NewJavascriptIPCAdapter(api.NewJavaScriptServiceClient(jsConn))
	}

	// Register hit-testing event listener
	var currentLayout *gcc.LayoutTree
	canvas.SetOnMouseClick(func(x, y float64) {
		if currentLayout != nil {
			hit := render.HitTest(currentLayout, x, y)
			if hit != nil && hit.Node != nil {
				log.Printf("[Orchestrator Event] Clicked Node: %s", hit.Node.Type)

				// Dispatch event over gRPC to Javascript VM
				if jsAdapter != nil {
					nodeID := hit.Node.Type // Simplified for mock
					err := jsAdapter.DispatchEvent(nodeID, "click", "{}")
					if err != nil {
						log.Printf("[Orchestrator Event] Failed to dispatch to JS: %v", err)
					}
				}
			}
		}
	})

	for !canvas.ShouldClose() {
		// Calculate UI dimensions
		layoutTree, err := layoutEngine.ComputeLayout(uiDom, uiCss)
		if err == nil && layoutTree != nil {
			// Paint the layout onto the hardware canvas
			layoutEngine.Paint(layoutTree, canvas)
			currentLayout = layoutTree
		}
	}

}
