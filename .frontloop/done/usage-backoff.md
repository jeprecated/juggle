---
title: Usage/quota backoff until window resets
priority: high
---

## Goal

When the agent hits API usage limits (daily quota, TPM/RPM caps, etc.), instead of failing or burning retries, detect the reset window and sleep until it reopens. Currently juggle does exponential backoff on rate limits (30s → 10min) but doesn't understand usage windows — it may give up before the window resets.

## Acceptance Criteria

- Detect usage/quota exhaustion distinct from transient rate limits
- Parse reset time from API response if available (e.g., "retry after" headers, error messages with timestamps)
- Sleep until the reset window, then resume the loop from where it left off
- Log clearly: "usage quota hit, waiting until HH:MM:SS (Xm Ys) for window reset"
- If reset time is unknown, fall back to existing exponential backoff
- Respect --max-wait as a cap — if reset window exceeds max-wait, exit cleanly
- Works in both RunLoop and RunWatch
- Tests cover quota detection, window parsing, and max-wait interaction

## Implementation Notes

- Extend rate limit detection in claude.go and opencode.go to distinguish quota vs transient rate limit
- Look for patterns like "daily limit", "quota exceeded", "usage limit", reset timestamps
- RetryAfter field on RunResult already exists — may need a separate QuotaResetsAt field
- Consider reading response headers if available through stream-json output

## Completion Summary

Implemented in commit `xu c0`:

- **provider.go**: Added `QuotaExhausted bool` and `QuotaResetsAt time.Time` to `RunResult`
- **claude.go**: Added shared `quotaPatterns` var; updated `parseRateLimit` to detect quota vs transient; added `parseQuotaResetTime` to extract relative ("resets in 2 hours") and absolute ("resets at 00:00 UTC") reset times
- **opencode.go**: Updated `parseRateLimit` similarly with OpenAI-specific quota patterns ("exceeded your quota", etc.)
- **juggle.go**: Added quota handling block in `RunLoop` before the rate-limit block; logs "usage quota hit, waiting until HH:MM:SS (Xm Ys) for window reset"; respects `--max-wait`; added `formatWaitDuration` helper
- **watch.go**: Same quota handling block in `runWatchTask`
- **quota_test.go** (new): Tests for quota pattern detection, reset time parsing
- **juggle_test.go**: Added 5 new tests for RunLoop quota behavior
