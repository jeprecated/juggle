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
