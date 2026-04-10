---
title: Integrate pipeline adapter into main RunLoop
priority: low
---

## Goal

Wire `AdaptConfigToPipeline` into the main `RunLoop` so that existing lifecycle hook flags (`--agent-pre`, `--cmd-before`, etc.) execute through the pipeline scheduler instead of bespoke code paths. This unifies execution under one model per the design doc's goal.

## Acceptance Criteria

- When lifecycle hook flags are used, `RunLoop` converts them via `AdaptConfigToPipeline` and runs through the executor
- Behavior is identical to the current hook-based execution for all existing tests
- No user-visible change in output or semantics
- An opt-in flag or environment variable gates the new path during rollout (e.g., `JUGGLE_USE_PIPELINE=1`)
- Old code path remains as fallback when the gate is off
- Tests cover: adapter round-trip matches current behavior, gate on/off, all hook combinations

## Implementation Notes

- This is the unification step that makes pipelines the single execution model
- Depends on: wire-pipeline-executor-to-cli, per-node-provider-dispatch
- Keep the gate mechanism simple: env var check at the top of RunLoop
- Once stable, a follow-up task can remove the old code path and the gate

## Completion Summary

Added `runViaPipeline(cfg Config) error` in `internal/cli/juggle.go` that calls `AdaptConfigToPipeline`, normalizes the pipeline, builds an `ExecutorConfig` from `cfg`, and runs via `pipeline.NewExecutor`. Added env var gate at the top of `RunLoop`: `JUGGLE_USE_PIPELINE=1` routes to the new path, any other value falls through to the old path. Tests in `internal/cli/runloop_pipeline_test.go` cover: gate-off uses old path (verified by error format), gate-on uses pipeline path, N iterations, non-"1" values do not activate gate, and lifecycle hooks run via executor.
