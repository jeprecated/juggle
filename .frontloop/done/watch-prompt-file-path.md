---
title: Include task file path in watch prompt
priority: medium
---

## Goal

Give the agent the local file path of the task file so it can read and update the file directly. Currently only the basename appears in the footer.

## Acceptance Criteria

- Task file's relative path (from working dir) appears as first line inside `<task>` block: `file: tasks/comments.json`
- Footer also uses the relative path instead of basename
- Falls back to absolute path if relative path can't be computed
- `BuildWatchPrompt` parameter renamed from `filename` to `taskRelPath`
- `runWatchTask` computes relative path instead of passing `filepath.Base(taskPath)`
- `JUGGLE_TASK_FILE` env var unchanged (already absolute)

## Design Decisions

- Path goes inside `<task>` block as first line (`file: {path}`), not as an attribute or separate section
- Relative path preferred over absolute for readability; absolute as fallback

## Completion Summary

- Renamed `BuildWatchPrompt` parameter `filename` → `taskRelPath`
- Updated `BuildWatchPrompt` to emit `file: {taskRelPath}` as first line inside `<task>` block
- Updated `BuildWatchPrompt` footer to use `taskRelPath` instead of basename
- Removed `filename` parameter from `runWatchTask`; function now computes relative path via `filepath.Rel(wd, taskFile)`, falling back to absolute
- Updated all `runWatchTask` call sites in `watch.go` and `watch_glob.go`
- Updated all test call sites in `watch_test.go`, `hooks_test.go`, `juggle_env_test.go`
- Updated `TestBuildWatchPrompt` and integration tests to assert new format

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/watch_glob.go` (modified)
- `internal/cli/juggle_test.go` (modified)
- `internal/cli/watch_test.go` (modified)
- `internal/cli/hooks_test.go` (modified)
- `internal/cli/juggle_env_test.go` (modified)
- `internal/integration_test/juggle_test.go` (modified)
