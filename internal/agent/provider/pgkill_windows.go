//go:build windows

package provider

import (
	"os/exec"
	"time"
)

// setProcessGroup is a no-op on Windows.
//
// Windows uses Job Objects to manage process trees, which requires a different
// mechanism (CreateJobObject / AssignProcessToJobObject). That is not yet
// implemented. On Windows, timeout kills only the parent process; child
// processes may outlive the agent run.
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup is a no-op on Windows.
//
// See setProcessGroup for the Windows limitation. The done channel is drained
// so callers do not block indefinitely.
func killProcessGroup(cmd *exec.Cmd, grace time.Duration, done <-chan struct{}) {
	if cmd.Process != nil {
		cmd.Process.Kill() //nolint:errcheck
	}
	select {
	case <-done:
	case <-time.After(grace):
	}
}
