//go:build !windows

package provider

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcessGroup configures cmd to run in its own process group.
// This ensures killProcessGroup can terminate all children, not just the parent.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGTERM to the process group of cmd. It waits for done
// to close (signalling that cmd.Wait() has returned) or for grace to expire,
// then sends SIGKILL if the process group is still alive.
//
// When Setpgid was set via setProcessGroup, the process group ID equals
// cmd.Process.Pid, so Kill(-pgid, signal) reaches every child process.
func killProcessGroup(cmd *exec.Cmd, grace time.Duration, done <-chan struct{}) {
	if cmd.Process == nil {
		return
	}
	pgid := cmd.Process.Pid

	// Send SIGTERM to every process in the group
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	// Wait for the process to exit (done closes when cmd.Wait() returns)
	// or fall back to SIGKILL after the grace period.
	select {
	case <-done:
		// Process exited cleanly after SIGTERM — no SIGKILL needed
	case <-time.After(grace):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
