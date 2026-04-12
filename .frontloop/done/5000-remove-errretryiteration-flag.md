---
title: Remove errRetryIteration flag - it provides no meaningful behavior
priority: medium
---

## Goal

Remove the errRetryIteration flag and retry mechanic. It doesn't provide meaningful behavior since the prompt is the same whether retrying iteration N or continuing to iteration N+1. The iteration counter is just metadata that's passed to agents.

## Acceptance Criteria

- [ ] Remove `errRetryIteration` variable from juggle.go
- [ ] Remove all `retryIteration:` labels from watch.go and watch_glob.go
- [ ] Replace `goto retryIteration` with `continue` or let loop naturally continue
- [ ] Remove checks for `errors.Is(err, errRetryIteration)`
- [ ] Update any related tests (watch_test.go, etc.)
- [ ] Remove any docs/comments about quota/rate limit retrying same iteration
- [ ] Verify watch mode still handles quota/rate limits by waiting and re-running (just increments counter instead)
