---
title: Glob watch paths with git root workdir
priority: high
---

## Goal

Allow `--watch` to accept glob patterns (e.g. `**/.frontloop/ready`) so a single juggle instance can watch for tasks across multiple repositories. When a task file is picked up, detect the git/jj root of that file and use it as the agent's working directory. `--workers` controls how many repos may have an active agent concurrently.

## Acceptance Criteria

- `--watch '**/.frontloop/ready'` expands the glob and watches all matching directories
- New directories matching the glob are discovered as they appear (not just at startup)
- Agent workdir is set to the VCS root (git or jj) of the matched task file
- `--workers N` caps how many repos can run an agent at the same time
- Existing single-directory `--watch ./tasks/` behavior is unchanged

## Completion Summary

Added `internal/cli/watch_glob.go` with:
- `isGlobPattern(s string) bool` — detects `*`, `?`, `[` metacharacters
- `FindVCSRoot(dir string) string` — walks up to find `.git` or `.jj` marker
- `expandGlobDirs(basedir, pattern string) ([]string, error)` — expands `**` globs via `doublestar` library
- `claimFromDirs(dirs []string) (string, error)` — extends `workerCoordinator` for multi-dir claiming
- `runGlobWatch`, `runGlobWatchSerial`, `runGlobWatchWorkers`, `runGlobWorkerLoop` — full serial and parallel watch loops

Modified `internal/cli/watch.go`: `RunWatch` now checks `isGlobPattern` and delegates to `runGlobWatch` before the existing single-dir path.

Added `github.com/bmatcuk/doublestar/v4` dependency for `**` glob support.

Tests: 13 new tests in `watch_glob_test.go` covering all new functions and integration scenarios.
