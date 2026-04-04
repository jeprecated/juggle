---
title: On-failure behavior (--on-failure)
priority: medium
---

## Goal

Add `--on-failure {stop|continue|retry}` to control what happens when an agent iteration exits non-zero. Default: stop. Most Ralph Loop tasks are idempotent, so continue/retry is often safe and prevents a transient API 500 from killing an overnight run.

## Acceptance Criteria

- `--on-failure stop` (default) — halt the loop on first non-zero exit
- `--on-failure continue` — log the failure, skip to next iteration
- `--on-failure retry` — retry the same iteration up to `--retries N` (default 2) with short backoff
- Rate-limited results still use existing rate limit backoff (not --on-failure)
- Failure count still tracked for --max-failures consecutive stop
- Works in both RunLoop and RunWatch
- Tests cover all three modes

## Implementation Notes

- Interacts with --max-failures: consecutive failure counter increments on continue/retry exhaustion
- Retry backoff: 10s, 30s (simple doubling, much shorter than rate limit backoff)
