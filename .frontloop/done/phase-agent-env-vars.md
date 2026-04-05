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

## Completion Summary

- Extended `phaseEnv` struct with `runID`, `model`, `provider`, `label` fields
- Updated `envSlice()` to emit `JUGGLE_RUN_ID`, `JUGGLE_MODEL`, `JUGGLE_PROVIDER`, and conditionally `JUGGLE_LABEL`
- Updated all 4 `phaseEnv{}` literals in `juggle.go` to populate run-level fields from `cfg`
- Updated all 4 `phaseEnv{}` literals in `watch.go` to populate run-level fields from `cfg`
- Added `TestRunLoop_PhaseAgent_ReceivesRunLevelEnvVars` verifying all 4 run-level vars in pre/before/after/post phases
- Added `TestPhaseEnv_EnvSlice_NoLabelOmitsLabelVar` and `TestPhaseEnv_EnvSlice_LabelIncludedWhenSet` as unit tests for `envSlice()`

### Files Changed

- `internal/cli/phase_agent.go` (modified)
- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/phase_agent_test.go` (modified)
