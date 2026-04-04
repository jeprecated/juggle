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

## Completion Summary

- Added `model string` field to `runStats` struct, initialized from `cfg.Model`
- Added `modelPricing` struct, `defaultPricing` map (opus/sonnet/haiku with USD/MTok rates), and `estimateCost()` function
- Updated `printRunSummary` to compute and display estimated cost (`~$X.XXXX`)
- Added `writeSummary(cfg, stats)` helper that prints to stderr and optionally appends to log file
- Added `Log string` field to `Config`, `--log` CLI flag, wired through `run()`
- `RunLoop`: replaced all `printRunSummary` calls with `writeSummary`; added `writeSummary` on clean loop completion and on stop-when trigger
- `RunWatch`: replaced `printRunSummary` calls with `writeSummary`; set `stats.model` on init
- Added 6 new tests: cost in summary, clean completion, stop-when, no summary on error, token accumulation, log file

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/juggle_test.go` (modified)
