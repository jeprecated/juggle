---
title: Pass-through flags to agent CLI (--)
priority: medium
---

## Goal

Support `--` separator to pass arbitrary flags directly to the underlying agent CLI. Everything after `--` is appended to the agent command as-is. Useful for provider-specific flags juggle doesn't know about.

## Acceptance Criteria

- `juggle @task.md -- --max-turns 50 --allowedTools Bash,Read` passes `--max-turns 50 --allowedTools Bash,Read` to the agent CLI
- Works with all providers (claude, opencode, custom)
- Pass-through flags appear after juggle's own flags in the spawned command
- `--` with nothing after it is a no-op
- Pass-through flags documented in help text with examples
- Tests verify flags are appended to agent command

## Implementation Notes

- Cobra supports `--` natively via `cmd.ArgsAfterDash` or by setting `TraverseChildren`
- Store pass-through args in Config, pass to provider via RunOptions
- Add a `PassthroughArgs []string` field to RunOptions
- Each provider's Run() appends these to its command args
