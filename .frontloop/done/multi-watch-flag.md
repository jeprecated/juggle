---
title: Multi-value --watch flag
priority: medium
---

## Goal

Allow `--watch` to be specified multiple times so unrelated directories can be watched together with a shared worker pool.

## Acceptance Criteria

- `flags.watch` becomes `[]string` via `StringArrayVar`; `Config.Watch` becomes `[]string`
- `--watch dir1 --watch dir2` works, mixing literal dirs and glob patterns
- Workers pull from all watched dirs via shared `workerCoordinator.claimFromDirs`
- Globs are re-expanded each poll cycle to pick up new directories
- Each non-absolute watch path resolves against `--workdir`
- `--workers requires --watch` validates `len(cfg.Watch) > 0`
- Dashboard auto-enabled for multi-watch (same as glob watch)
- `juggle.toml` accepts `watch` as string or list of strings
- Existing single `--watch` behavior unchanged (single dir and glob paths still work)

## Design Decisions

- `StringArrayVar` (repeatable flag), not `StringSliceVar` (comma-separated), since paths could contain commas
- Shared worker pool across all dirs, not per-dir pools
- Routing: `len == 0` -> RunLoop, `len == 1` plain dir -> existing single-dir path, `len == 1` glob -> existing runGlobWatch, `len > 1` -> new runMultiWatch that merges all dirs and feeds claimFromDirs

## Completion Summary

- `Config.Watch` changed from `string` to `[]string` across all production code
- `flags.watch` changed to `[]string` with `StringArrayVar` in cobra init
- `Run()` validation updated: `len(cfg.Watch) == 0` for workers check; relative path resolution loops over slice
- `RunWatch()` routing updated: `len > 1` routes to new `runMultiWatch`, single-entry uses `[0]` index
- `runGlobWatchSerial` and `runGlobWatchWorkers` updated to use `cfg.Watch[0]` for the glob pattern
- `runWatchWorkers` and `runWorkerLoop` updated to use `cfg.Watch[0]` for the single-dir path
- `runMultiWatch` added: auto-enables dashboard, routes to `runMultiWatchSerial` or `runMultiWatchWorkers`
- `runMultiWatchSerial` added: serial worker using `claimFromDirs` across merged dir list each cycle
- `runMultiWatchWorkers` added: parallel workers reusing `runGlobWorkerLoop` with a `getDirs` func
- `getDirsForWatch` helper added: expands globs and collects literal dirs for each watch entry
- `tomlStringOrList` custom TOML type added: decodes both `watch = "dir"` and `watch = ["a", "b"]`
- `FileConfig.Watch *tomlStringOrList` added; `ApplyFileConfig` updated to apply it
- `dryrun.go` updated: `cfg.Watch != ""` → `len(cfg.Watch) > 0`
- All existing tests updated to use `Watch: []string{...}` syntax

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/watch_glob.go` (modified)
- `internal/cli/config.go` (modified)
- `internal/cli/dryrun.go` (modified)
- `internal/cli/watch_test.go` (modified)
- `internal/cli/watch_glob_test.go` (modified)
- `internal/cli/dryrun_test.go` (modified)
- `internal/cli/config_test.go` (modified)
- `internal/integration_test/juggle_test.go` (modified)
