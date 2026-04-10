---
title: Parallel node execution in pipeline scheduler
priority: medium
---

## Goal

Implement concurrent execution for pipeline nodes marked with `--parallel`. Currently the executor runs all nodes sequentially within each event. Nodes with `parallel = true` (which suppresses implicit dependencies) should run concurrently with respect to other parallel siblings.

## Acceptance Criteria

- Nodes with `parallel = true` and no explicit `after` dependencies run concurrently
- Nodes with explicit `after` dependencies wait for their dependencies to complete before starting
- `max_parallel_steps` from the pipeline config is respected as a concurrency limit
- If any parallel node fails with `on_failure = stop`, other running nodes are cancelled
- `on_failure = continue` nodes do not cancel siblings
- Shutdown signal cancels all running nodes
- Tests cover: parallel execution ordering, dependency waiting, concurrency limit, failure propagation, shutdown

## Implementation Notes

- The dependency graph is already built during normalization (After fields)
- Use a semaphore pattern with `max_parallel_steps` (or unlimited if 0)
- Consider a topological walk: nodes whose dependencies are all complete become eligible
- Use `sync.WaitGroup` or `errgroup.Group` for goroutine management
- Keep the sequential fast path: if no nodes are parallel, skip the concurrent machinery

## Completion Summary

- `internal/pipeline/executor.go`: added `runEventConcurrent` with a topological scheduler using a channel-based semaphore for `max_parallel_steps`, per-event cancellable context forwarding the shutdown signal, and a `schedule()` loop that launches goroutines as dependencies complete. Context param threaded through `runNodeWithPolicy`, `runNode`, `runAgentNode`, `runCmdNode`, `evalWhen`. Sequential fast path preserved when no `Parallel=true` nodes present.
- `internal/pipeline/executor_test.go`: added 6 tests — both nodes complete, dependency ordering, stop failure propagates, continue failure allows siblings, max_parallel_steps timing, shutdown cancellation.
