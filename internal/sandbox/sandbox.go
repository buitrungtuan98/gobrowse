package sandbox

import (
	"os/exec"
)

// Policy defines the restriction level applied to a process.
type Policy int

const (
	// PolicyStrict restricts all network access and limits filesystem access (used for Render/JS).
	PolicyStrict Policy = iota
	// PolicyNetwork limits filesystem access but allows network sockets (used for Network proxy).
	PolicyNetwork
)

// Sandbox defines the cross-platform interface for executing secure isolated child processes.
type Sandbox interface {
	// Configure modifies the exec.Cmd attributes to inject OS-specific isolation rules.
	Configure(cmd *exec.Cmd, policy Policy) error
}

// OSBuilder creates the platform-specific sandbox implementation.
// This is fulfilled in the OS-specific files (sandbox_linux.go, sandbox_windows.go).
var Builder Sandbox
