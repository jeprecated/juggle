---
title: Working directory flag (--workdir)
priority: medium
---

## Goal

Add `--workdir path/to/repo` so juggle spawns the agent with that directory as its working directory instead of cwd. Useful for CI, cron, and orchestration scripts.

## Acceptance Criteria

- `--workdir DIR` sets the agent's working directory
- Directory must exist, error if not
- Affects agent spawning only — juggle itself stays in its original cwd
- Watch directory paths are relative to workdir if not absolute
- @file resolution still uses juggle's cwd (not workdir) for prompt files
- Passed through as RunOptions.WorkingDir (field already exists)
- Tests verify agent runs in specified directory

## Implementation Notes

- WorkingDir field already exists on RunOptions — just need to wire the flag through
- Set cmd.Dir in provider Run() implementations

## Completion Summary

- Added `WorkDir string` field to `Config` struct
- Added `workdir string` to `flags` struct and registered `--workdir` cobra flag
- Wired `flags.workdir` into `cfg.WorkDir` in the `run()` cobra handler
- Added validation in `Run()`: errors if WorkDir is set but directory doesn't exist
- Added relative watch path resolution in `Run()`: if `--watch` is relative and `--workdir` is set, joins them
- Wired `cfg.WorkDir` → `RunOptions.WorkingDir` in `buildRunOptions()` (providers already set `cmd.Dir` from this field)

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/juggle_test.go` (modified)
- `internal/cli/watch_test.go` (modified)
