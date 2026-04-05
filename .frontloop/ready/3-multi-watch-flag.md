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
