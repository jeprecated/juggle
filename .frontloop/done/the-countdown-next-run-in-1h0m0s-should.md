---
title: The countdown 'Next run in 1h0m0s' should actually update the time too. Remove the seconds though, until there's only seconds remaining
priority: medium
---

## Goal

The countdown "Next run in Xh Xm Xs" message currently shows a static duration string for the entire wait period. It should tick down in real time.

## Acceptance Criteria

- [ ] The "Next run in ..." message updates every tick (~150ms) as time passes
- [ ] Seconds are hidden when minutes or hours remain (show "1h30m" not "1h30m0s")
- [ ] Seconds appear once the countdown is under 1 minute (show "45s")
- [ ] Works in both TTY and non-TTY modes

## Design Decisions

- Format: hide seconds until under 1 minute, then show only seconds
- Tick rate: reuse existing 150ms ticker

## Implementation Notes

- Modify `pollWaitWithWake` in `internal/cli/format.go` to compute remaining time each tick
- Track start time and total delay, compute remaining = delay - elapsed on each tick
- For non-TTY mode, just print the initial static message (no point updating without \r)

## Completion Summary

- Added `formatCountdown()` helper that formats durations with seconds hidden when >= 1 minute
- Modified `pollWaitWithWake` to accept `countdown bool` param; when true, tracks start time and computes remaining time each tick
- Updated 5 callers in watch.go: 3 countdown ("Next run in"), 2 non-countdown ("Watching", "Waiting for tasks")
- Fixed `IterationStatus` to prepend newline in TTY mode, preventing stats from appearing on same line as agent output

### Files Changed

- internal/cli/format.go (modified)
- internal/cli/watch.go (modified)
