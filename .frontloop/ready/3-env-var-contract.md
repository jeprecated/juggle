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
