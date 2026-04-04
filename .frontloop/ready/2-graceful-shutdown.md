---
title: Graceful SIGINT/SIGTERM shutdown
priority: high
---

## Goal

Trap SIGINT/SIGTERM so the first signal finishes the current iteration cleanly, prints a run summary, and exits. A second signal force-kills immediately. Prevents orphaned agent processes and mid-write corruption.

## Acceptance Criteria

- First SIGINT/SIGTERM sets a "shutting down" flag
- Current in-flight iteration is allowed to complete (up to --timeout)
- No new iteration is started after the flag is set
- Run summary (iterations completed, tokens, duration) printed on clean exit
- Second signal force-kills the process immediately
- Agent child process is also terminated on force-kill
- Works in both RunLoop and RunWatch
- Exit code 130 for signal-interrupted runs
- Tests verify flag prevents next iteration from starting

## Implementation Notes

- Use signal.NotifyContext or a channel-based signal handler
- Check shutdown flag at the top of each iteration loop
- On force-kill, send SIGTERM to child process group
