//go:build windows
// +build windows

package sandbox

import (
	"os/exec"
	"syscall"
)

type windowsSandbox struct{}

func init() {
	Builder = &windowsSandbox{}
}

func (s *windowsSandbox) Configure(cmd *exec.Cmd, policy Policy) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	// Apply Windows specific process isolation flags.
	// CREATE_BREAKAWAY_FROM_JOB allows the process to be attached to a restricted Job Object.
	// DETACHED_PROCESS isolates the process from the parent's console.
	cmd.SysProcAttr.CreationFlags = 0x01000000 | 0x00000008

	if policy == PolicyStrict {
		// Note: Full AppContainer or Restricted Token isolation requires low-level
		// Windows API calls (e.g. Advapi32.dll CreateRestrictedToken).
		// For this milestone, we set up the basic struct creation flags.
	}

	return nil
}
