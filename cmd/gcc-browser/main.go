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
	fmt.Println("[GCC Orchestrator] Orchestration mock ready. Ensure `gcc-daemon` is available in PATH.")
}
