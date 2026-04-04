---
title: Aggregate run stats
priority: medium
---

## Goal

Print a summary at the end of a full run showing total tokens consumed, estimated cost, wall time, and iteration count. Helps users understand the cost of overnight runs and compare efficiency across prompts.

## Acceptance Criteria

- Summary printed to stderr at end of RunLoop and RunWatch
- Shows: total iterations completed, total input/output/cache tokens, estimated cost, total wall time
- Cost estimate uses configurable per-token pricing (sensible defaults for Claude models)
- Summary also written to log file if `--log` is set
- Not printed on early error exit (only on clean completion or stop-when trigger)
- Tests verify accumulation across multiple iterations

## Implementation Notes

- Accumulate RunResult token counts across iterations
- Cost calculation: simple multiplication, no need for live API pricing
- Default pricing can be a const map keyed by model name
