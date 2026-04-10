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
