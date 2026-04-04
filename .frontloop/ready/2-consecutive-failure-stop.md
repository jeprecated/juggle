---
title: Consecutive failure stop
priority: high
---

## Goal

After N consecutive non-zero exit codes from the agent (default 3), stop the loop with a clear diagnostic instead of burning remaining iterations on a broken config or stuck agent.

## Acceptance Criteria

- `--max-failures N` flag (default 3) sets the consecutive failure threshold
- Counter resets to 0 on any successful (exit 0) iteration
- On threshold breach, exit with clear message: "stopping: N consecutive failures"
- Rate-limited iterations don't count as failures (they're retried)
- Works in both RunLoop and RunWatch
- `--max-failures 0` disables the check
- Tests verify counter reset on success and threshold breach

## Implementation Notes

- Simple counter variable in the loop, reset on success
- Check after result processing, before delay/fuzz sleep
