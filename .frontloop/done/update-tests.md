---
title: Update tests for loop/queue consolidation
priority: 1600
---

## Goal

Update all existing tests to use the new `juggle loop` and `juggle queue` commands instead of the old bare `juggle`, `juggle watch`, and `juggle serve` invocations.

## Context

Final cleanup task. Depends on all previous tasks (1000-1500) being completed.

## Acceptance Criteria

- No test references `juggle watch` or `juggle serve` as CLI invocations
- No test calls `runWatchSubcmd()` or `runServeCmd()` directly
- Tests that tested bare `juggle "prompt"` now test `juggle loop "prompt"`
- Tests that tested `juggle watch ./tasks/ @rules.md` now test `juggle queue @rules.md --watch ./tasks/`
- Tests that tested serve HTTP handler now test via queue's `--serve`
- Shell completion tests updated for new subcommand names
- Help output tests updated for new help text
- Config file tests updated for new config keys (no `every_immediate`, `now` instead)
- Watch glob tests updated for `--watch` flag syntax
- Queue tests do NOT use `-n`/`--iterations` (removed from queue)
- `go test ./...` — all tests pass
- `devbox run build` — builds cleanly

## Implementation Notes

- Watch tests (`watch_test.go`, `watch_glob_test.go`): update to construct queue Config instead of watch Config
- Serve tests (`serve_test.go`): update HTTP handler tests for trigger-based flow (WriteTrigger, not file writes)
- Help tests (`help_test.go`): update expected output
- Cobra structure tests (`juggle_test.go`): verify `loop` and `queue` are subcommands of root, verify flag ownership (no `-n` on queue, no `--watch` on loop)
- Completion tests (`nushell_test.go`, `powershell_test.go`): update subcommand lists
- Integration tests (`internal/integration_test/`): check for any test scripts using old syntax

## Files to Change

- `internal/cli/juggle_test.go`
- `internal/cli/watch_test.go`
- `internal/cli/watch_glob_test.go`
- `internal/cli/serve_test.go`
- `internal/cli/help_test.go`
- `internal/cli/config_test.go`
- `internal/cli/nushell_test.go`
- `internal/cli/powershell_test.go`
- `internal/cli/complete_test.go`
- Any integration test files

## Completion Summary

- All test files were updated during tasks 1000-1500 as part of the source refactoring
- Verified: no test references `runWatchSubcmd()`, `runServeCmd()`, or `every_immediate`
- Verified: `nushell_test.go` defensively asserts old subcommands are absent
- Verified: queue tests do not use `-n`/`--iterations`
- `go test ./...` — all tests pass
- `go build` — builds cleanly

### Files Changed

- No additional changes needed — tests were already updated by prior tasks
