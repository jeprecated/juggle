---
title: Process tree kill on timeout
priority: high
---

## Goal

When `--timeout` fires, kill the entire process tree (agent + subprocesses) using process groups, not just the parent PID. Prevents zombie children holding file locks or ports.

## Acceptance Criteria

- Agent process spawned with its own process group (`setpgid`)
- On timeout, `kill(-pgid, SIGTERM)` kills entire tree
- Fallback to SIGKILL after 5s grace period if SIGTERM doesn't work
- Works on Linux and macOS
- Tests verify child processes are cleaned up (may need integration test)

## Implementation Notes

- Set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` in provider Run()
- Kill with `syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)`
- Windows will need a different approach (Job Objects) — skip for now, document limitation

## Completion Summary

- Added `pgkill_unix.go` (`!windows` build tag): `setProcessGroup` sets `Setpgid=true`; `killProcessGroup` sends SIGTERM, waits on a `done` channel (closed when `cmd.Wait()` returns), then SIGKILL after 5s grace
- Added `pgkill_windows.go` (`windows` build tag): no-op stubs with documented limitation (Job Objects not implemented)
- Modified `claude.go` `runHeadless` and `runInteractive`: replaced `exec.CommandContext` with `exec.Command` + `setProcessGroup`; added goroutine watching `ctx.Done()` → `killProcessGroup`
- Modified `opencode.go` `runHeadless` and `runInteractive`: same pattern as claude.go
- Added 6 unit tests in `pgkill_test.go` covering: Setpgid attribute, nil process safety, SIGTERM termination, early return via done channel, SIGKILL fallback timing, and child process group kill

### Files Changed

- `internal/agent/provider/pgkill_unix.go` (new)
- `internal/agent/provider/pgkill_windows.go` (new)
- `internal/agent/provider/pgkill_test.go` (new)
- `internal/agent/provider/claude.go` (modified)
- `internal/agent/provider/opencode.go` (modified)
