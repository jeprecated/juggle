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

## Completion Summary

- Added `OnFailure` string type with constants `OnFailureStop`, `OnFailureContinue`, `OnFailureRetry`
- Added `retryBackoffFor()` helper and `defaultRetryBackoffs` var (10s, 30s)
- Added `OnFailure`, `Retries`, `RetryBackoffs` fields to `Config`
- Added `--on-failure` (default "stop") and `--retries` (default 2) cobra flags
- Added `--on-failure` validation in `Run()`
- Implemented on-failure logic in `RunLoop`: stop halts immediately, continue logs and falls through to MaxFailures check, retry uses `i--; continue` loop with per-attempt backoff and exhaustion fallthrough
- Same logic applied to `runWatchTask` in `watch.go`
- Updated 6 existing tests in `juggle_test.go` and 2 in `watch_test.go` to add `OnFailure: OnFailureContinue` (old MaxFailures-based behavior)
- Added 6 new tests in `juggle_test.go` and 3 in `watch_test.go` covering all three modes

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/juggle_test.go` (modified)
- `internal/cli/watch_test.go` (modified)
