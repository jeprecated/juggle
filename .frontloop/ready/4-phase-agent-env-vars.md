---
title: Add run-level env vars to phase agents
priority: low
---

## Goal

Phase agents (--agent-pre/before/after/post) currently only receive JUGGLE_PHASE, JUGGLE_ITERATION, and JUGGLE_MAX_ITERATIONS. They should also receive JUGGLE_RUN_ID, JUGGLE_MODEL, JUGGLE_PROVIDER, and JUGGLE_LABEL so phase agent prompts can correlate with the main run.

## Problem

`phase_agent.go:44` does `opts.Env = env.envSlice()` which overwrites the env with only phase-specific vars.

## Acceptance Criteria

- Phase agents receive JUGGLE_RUN_ID, JUGGLE_MODEL, JUGGLE_PROVIDER
- Phase agents receive JUGGLE_LABEL when label is non-empty
- Existing phase-specific vars (JUGGLE_PHASE, JUGGLE_ITERATION, JUGGLE_MAX_ITERATIONS) still set
- Tests verify run-level vars are present in phase agent env
