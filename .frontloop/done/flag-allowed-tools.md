---
title: Expose --allowed-tools / --disallowed-tools flags
priority: medium
---

## Goal

Add flags to restrict which tools the agent can use during iterations. Important safety measure for overnight runs — e.g., allow only Bash and Read, block Write.

## Acceptance Criteria

- `--allowed-tools Bash,Read,Grep` restricts agent to these tools only
- `--disallowed-tools Write,Edit` blocks specific tools
- Using both simultaneously is an error
- Passed to Claude Code as `--allowedTools` / `--disallowedTools`
- Mapped for other providers or ignored with verbose warning
- Tests verify flags passed to agent command

## Implementation Notes

- Add AllowedTools and DisallowedTools string fields to RunOptions
- Validate mutual exclusivity in Run() before entering loop

## Completion Summary

- Added `AllowedTools []string` and `DisallowedTools []string` to `provider.RunOptions`
- Added `AllowedTools []string` and `DisallowedTools []string` to `cli.Config`
- Added `--allowed-tools` and `--disallowed-tools` CLI flags (StringSlice, comma-separated)
- Wired flags through `run()` → `Config` → `buildRunOptions()` → `RunOptions`
- Validated mutual exclusivity in `Run()` before entering loop
- Extracted `claudeHeadlessArgs(opts RunOptions) []string` from `runHeadless` for testability
- Claude provider passes `--allowedTools` / `--disallowedTools` to Claude CLI when set
- OpenCode provider logs a verbose warning to stderr and ignores the flags

### Files Changed

- `internal/agent/provider/provider.go` (modified)
- `internal/agent/provider/claude.go` (modified)
- `internal/agent/provider/opencode.go` (modified)
- `internal/agent/provider/provider_test.go` (modified)
- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/juggle_test.go` (modified)
