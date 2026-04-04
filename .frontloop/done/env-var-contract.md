---
title: Environment variable contract
priority: medium
---

## Goal

Set well-documented environment variables before spawning each agent process so prompts, skills, and MCP servers can read loop state without juggle being opinionated about how they use it.

## Acceptance Criteria

- `JUGGLE_ITERATION` — current iteration number (1-indexed)
- `JUGGLE_MAX_ITERATIONS` — total planned iterations (0 = unlimited)
- `JUGGLE_RUN_ID` — stable UUID for the entire run invocation
- `JUGGLE_LABEL` — run label if set
- `JUGGLE_MODEL` — model name being used
- `JUGGLE_PROVIDER` — provider name being used
- In watch mode, additionally: `JUGGLE_TASK_FILE` — path to current task file
- With --workers, additionally: `JUGGLE_WORKER_ID` — 0-indexed worker number
- All vars documented in README
- Tests verify env vars are set on spawned processes

## Implementation Notes

- Set via cmd.Env in provider Run() — append to os.Environ()
- JUGGLE_RUN_ID generated once in Run() and passed through RunOptions
- Worker ID assigned per goroutine in watch worker pool

## Completion Summary

- Added `generateRunID()` (UUID v4 via crypto/rand) in `internal/cli/juggle_env.go`
- Added `buildJuggleEnv()` helper constructing all `JUGGLE_*` vars; omits JUGGLE_LABEL/TASK_FILE/WORKER_ID when not applicable
- Added `Label string` and `RunID string` fields to `Config`; RunID auto-generated in `Run()` and `RunLoop()` / `runWatchTask()` if empty
- Added `--label` flag wired to `Config.Label`
- Updated `RunLoop` to append `buildJuggleEnv(...)` to `RunOptions.Env` per iteration
- Updated `runWatchTask` to append `buildJuggleEnv(...)` with `JUGGLE_TASK_FILE` per iteration
- Fixed `claude.go` `runInteractive` to apply `opts.Env` (was previously ignored)
- Documented all `JUGGLE_*` vars in README under new "Environment variables" section
- Tests in `juggle_env_test.go` verify `buildJuggleEnv`, `generateRunID`, RunLoop env propagation, and runWatchTask task file var

### Files Changed

- `internal/cli/juggle_env.go` (new)
- `internal/cli/juggle_env_test.go` (new)
- `internal/cli/juggle.go` (modified — Label/RunID fields, --label flag, RunLoop env injection)
- `internal/cli/watch.go` (modified — RunID generation, runWatchTask env injection)
- `internal/agent/provider/claude.go` (modified — runInteractive now applies opts.Env)
- `README.md` (modified — env vars table and --label flag doc)
