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
