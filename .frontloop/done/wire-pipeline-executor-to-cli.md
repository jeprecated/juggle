---
title: Wire pipeline executor into the CLI subcommand
priority: high
---

## Goal

Connect `runPipelineCmd` in `internal/cli/pipeline.go` to actually execute the pipeline using the existing `Executor`, replacing the placeholder message at line 79 ("execution not yet implemented").

## Acceptance Criteria

- `juggle pipeline --file pipeline.toml` loads, normalizes, and executes the pipeline
- `juggle pipeline agent "main" "do work" --event loop-body` parses and executes inline pipelines
- The executor receives a properly configured `ExecutorConfig` with:
  - Runner from provider resolution (use the same provider selection as `RunLoop`)
  - Stdout/Stderr from os.Stdout/os.Stderr
  - WorkDir from `--workdir` flag or cwd
  - Shutdown channel wired to signal handling
  - RunID generated
- Exit code reflects executor success/failure
- Iteration count from the pipeline is respected
- Tests cover: successful execution, executor failure propagation, signal handling

## Implementation Notes

- Reuse existing provider setup from `internal/cli/juggle.go` (the `buildRunner` or equivalent path)
- The `pipeline` subcommand needs flags for `--workdir`, `--provider`, `--model` at the top level (these feed into ExecutorConfig and Defaults)
- Keep it simple: single-threaded execution only for now (parallel execution is a separate task)

## Completion Summary

Replaced the placeholder in `runPipelineCmd` with full pipeline execution:

- Added `resolvePipelineRunner()` — mirrors the provider/agentCmd resolution in `Run()`, with a `pipelineTestRunner` hook for test injection
- Added `executePipeline(p, cfg)` — thin wrapper around `pipeline.NewExecutor(p, cfg).Run()`
- `runPipelineCmd` now builds `ExecutorConfig` with Runner, RunnerFactory (via `makeRunnerFactory`), Stdout/Stderr, ForceCtx, Shutdown channel, WorkDir (flag or cwd), and a generated RunID
- Signal handling wired: first SIGINT/SIGTERM closes shutdown channel, second force-cancels context
- `--workdir`, `--provider`, `--model`, `--agent-cmd` are all available as persistent flags (already declared on rootCmd)

Files changed:
- `internal/cli/pipeline.go` — full implementation replacing placeholder
- `internal/cli/pipeline_test.go` — 3 new tests: Success, ExecutorFailure, ShutdownInterrupts
