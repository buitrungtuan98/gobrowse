//go:build linux
// +build linux

package sandbox

import (
	"os/exec"
	"syscall"
)

type linuxSandbox struct{}

func init() {
	Builder = &linuxSandbox{}
}

func (s *linuxSandbox) Configure(cmd *exec.Cmd, policy Policy) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Apply Linux namespaces for process isolation
	// CLONE_NEWPID: Gives the child process an independent process ID tree.
	// CLONE_NEWNS: Gives the child process an independent mount namespace.
	cmd.SysProcAttr.Cloneflags = syscall.CLONE_NEWPID | syscall.CLONE_NEWNS

	if policy == PolicyStrict {
		// Strict sandboxing (Renderers & JS execution) completely removes network access
		// by detaching the network namespace.
		cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	}

	// Note: In a fully productionized environment, Seccomp BPF filters would be compiled
	// and attached here using `golang.org/x/sys/unix` to block explicit syscalls like `open()`.

	return nil
}
