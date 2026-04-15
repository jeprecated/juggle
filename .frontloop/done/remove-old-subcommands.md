---
title: Remove old watch and serve subcommands
priority: 1400
---

## Goal

Remove the old `juggle watch` and `juggle serve` subcommands now that `juggle queue` replaces them both.

## Context

See ADR at `docs/adr-001-loop-queue.md`. Once `juggle queue` is complete and bare `juggle` shows help, the old commands are dead code.

Depends on tasks 1000, 1100, 1200, and 1300 being completed first.

## Acceptance Criteria

- `juggle watch` → "unknown command" error
- `juggle serve` → "unknown command" error
- `watchCmd` variable and its cobra registration are removed
- `serveCmd` variable and its cobra registration are removed
- `watchFlags` struct is removed (replaced by `queueFlags`)
- `serveSpecificFlags` struct is removed
- Old `runWatchSubcmd()` handler is removed
- Old `runServeCmd()` handler is removed
- All code that was only reachable from watch/serve but not from queue is removed
- All tests updated: references to `juggle watch` → `juggle queue --watch`, references to `juggle serve` → `juggle queue --serve`
- `go build ./...` succeeds
- All tests pass

## Implementation Notes

- Be careful not to remove `RunWatch()` or `RunServe()` internal functions if `queue` still delegates to them. Only remove the cobra command definitions and their handlers.
- Check for any test helpers that construct Config with `Watch` field — those should still work since queue uses the same Config struct
- The `detectShellGlobExpansion` function may no longer be needed if queue takes `--watch` as a flag (not positional arg). Evaluate and remove if dead code.
- Update completion tests that reference `juggle watch` and `juggle serve`

## Files to Change

- `internal/cli/juggle.go` — remove `watchCmd`, `watchFlags`, `runWatchSubcmd`
- `internal/cli/serve.go` — remove `serveCmd`, `serveSpecificFlags`, `runServeCmd` (keep `RunServe` if queue delegates to it)
- `internal/cli/juggle_test.go` — update test invocations
- `internal/cli/serve_test.go` — update or remove serve-specific tests
- `internal/cli/help_test.go` — update help output expectations
- `internal/cli/nushell_test.go` — update completion tests
- `internal/cli/config.go` — remove `watchFlags` references if not already done
- `internal/cli/config_test.go` — update config tests

## Completion Summary

- Removed `watchCmd`, `serveCmd` cobra commands and their registrations
- Removed `watchFlags` struct (replaced by `queueFlags`), `serveSpecificFlags` struct
- Removed `runWatchSubcmd()` and `runServeCmd()` handlers
- Removed `detectShellGlobExpansion()` (dead code after queue uses --watch flag)
- Kept `RunWatch()`, `RunServe()`, `parseServeAddr()`, `newServeHandler()` as queue delegates to them
- Renamed `EveryImmediate` to `Now` in Config struct
- Updated all tests: watch/serve references → queue equivalents
- Added `queue_test.go` with comprehensive queue subcommand tests
- Updated help, completion, and config tests

### Files Changed

- `internal/cli/juggle.go` (modified) — removed watch/serve commands, added queue command
- `internal/cli/serve.go` (modified) — removed serveCmd, kept helper functions
- `internal/cli/watch.go` (modified) — EveryImmediate → Now
- `internal/cli/watch_glob.go` (modified) — removed detectShellGlobExpansion
- `internal/cli/config.go` (modified) — updated flag references
- `internal/cli/help.go` (modified) — added Queue Mode group
- `internal/cli/juggle_test.go` (modified) — updated for loop/queue
- `internal/cli/serve_test.go` (modified) — updated for queue
- `internal/cli/help_test.go` (modified) — updated for queue
- `internal/cli/nushell_test.go` (modified) — updated completions
- `internal/cli/config_test.go` (modified) — updated flag references
- `internal/cli/loop_test.go` (modified) — added bare juggle tests
- `internal/cli/watch_glob_test.go` (modified) — removed glob detection tests
- `internal/cli/queue_test.go` (new) — queue subcommand tests
