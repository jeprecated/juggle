---
title: Cost guard (--max-cost)
priority: medium
---

## Goal

Add `--max-cost DOLLARS` flag that stops the loop when estimated spend exceeds the threshold. Essential safety net for overnight/unattended runs to prevent runaway costs.

## Acceptance Criteria

- `--max-cost 5.00` stops the loop when cumulative cost estimate exceeds $5.00
- Uses same cost estimation as aggregate stats
- Logs a clear message: "cost guard triggered: estimated $X.XX exceeds --max-cost $Y.YY"
- Exits cleanly (not an error), prints aggregate stats before exiting
- Works in both normal loop and watch mode
- Tests verify guard triggers at correct threshold

## Implementation Notes

- Depends on aggregate stats (same token accumulation and pricing logic)
- Check after each iteration, before delay/fuzz sleep

## Completion Summary

- Added `MaxCost float64` field to `Config` struct
- Added `--max-cost` flag (float64, default 0 = disabled) to cobra flags
- Added `errCostGuard` sentinel error for watch mode signaling
- In `RunLoop`: check cumulative cost after each iteration; if exceeded, log message, print summary, return nil
- In `runWatchTask`: same check using shared `*runStats`; return `errCostGuard` when triggered
- In `RunWatch`: handle `errCostGuard` by printing summary and returning nil (clean exit)
- 7 new tests covering: trigger at threshold, no trigger below threshold, log message format, summary printed, watch mode trigger, watch mode clean exit, MaxCost=0 disabled

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/juggle_test.go` (modified)
- `internal/cli/watch_test.go` (modified)
