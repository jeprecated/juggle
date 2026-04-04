//go:build !windows

package provider

import (
	"fmt"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestSetProcessGroup_SetsSetpgid(t *testing.T) {
	cmd := exec.Command("sleep", "1")
	setProcessGroup(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set, got nil")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Error("expected Setpgid=true after setProcessGroup")
	}
}

func TestKillProcessGroup_NilProcess_NoOp(t *testing.T) {
	cmd := exec.Command("sleep", "1")
	done := make(chan struct{})
	close(done)
	// Should not panic when process is nil (command never started)
	killProcessGroup(cmd, time.Second, done)
}

func TestKillProcessGroup_TerminatesProcess(t *testing.T) {
	cmd := exec.Command("sleep", "100")
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pgid := cmd.Process.Pid

	done := make(chan struct{})
	go func() {
		killProcessGroup(cmd, 2*time.Second, done)
	}()

	cmd.Wait() //nolint:errcheck
	close(done)

	// After cmd.Wait() reaps the process, the process group leader is gone
	if syscall.Kill(-pgid, 0) == nil {
		t.Errorf("process group %d still alive after kill", pgid)
	}
}

func TestKillProcessGroup_ReturnsEarlyWhenDoneClosed(t *testing.T) {
	// When cmd.Wait() returns (done closed), killProcessGroup should not block
	// for the full grace period.
	cmd := exec.Command("sleep", "100")
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	done := make(chan struct{})
	grace := 5 * time.Second

	start := time.Now()
	// Close done after a short delay (simulating cmd.Wait() returning after SIGTERM)
	go func() {
		time.Sleep(150 * time.Millisecond)
		close(done)
	}()

	killProcessGroup(cmd, grace, done) // synchronous call
	elapsed := time.Since(start)
	cmd.Wait() //nolint:errcheck

	// Should have returned when done was closed (~150ms), not after 5s grace
	if elapsed >= 2*time.Second {
		t.Errorf("expected early return via done channel, took %v (grace=%v)", elapsed, grace)
	}
}

func TestKillProcessGroup_FallsBackToSIGKILL(t *testing.T) {
	// When done is never closed within the grace period, SIGKILL path fires.
	cmd := exec.Command("sleep", "100")
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pgid := cmd.Process.Pid

	done := make(chan struct{}) // intentionally never closed during killProcessGroup
	grace := 200 * time.Millisecond

	start := time.Now()
	killProcessGroup(cmd, grace, done) // blocks synchronously for ~grace
	elapsed := time.Since(start)
	cmd.Wait() //nolint:errcheck
	close(done)

	// Should have waited approximately the grace period before returning via SIGKILL
	if elapsed < grace-50*time.Millisecond {
		t.Errorf("expected to wait ~grace before SIGKILL path, returned after %v", elapsed)
	}
	// Process group should be gone after SIGKILL + Wait
	if syscall.Kill(-pgid, 0) == nil {
		t.Errorf("process group %d still alive after SIGKILL", pgid)
	}
}

func TestKillProcessGroup_KillsEntireGroup(t *testing.T) {
	// Shell spawns a background child. Process group kill sends SIGTERM to both.
	// Verifies children (not just the parent) receive the signal.
	cmd := exec.Command("sh", "-c", "sleep 100 & echo $!; wait")
	setProcessGroup(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	var childPID int
	fmt.Fscan(stdout, &childPID) //nolint:errcheck

	time.Sleep(50 * time.Millisecond) // let child settle

	done := make(chan struct{})
	go func() {
		killProcessGroup(cmd, 500*time.Millisecond, done)
	}()

	cmd.Wait() //nolint:errcheck
	close(done)

	if childPID <= 0 {
		t.Skip("could not read child PID")
	}

	// Poll until init reaps the child zombie (usually within a few hundred ms)
	for i := 0; i < 20; i++ {
		if syscall.Kill(childPID, 0) != nil {
			return // child is gone — test passes
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("child process %d still alive after process group kill", childPID)
}
