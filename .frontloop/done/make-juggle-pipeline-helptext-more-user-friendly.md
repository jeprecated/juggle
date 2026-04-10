---
title: Make juggle pipeline helptext more user friendly. The normal helptext is coloured, why not here too
priority: medium
---

## Goal

Make juggle pipeline helptext more user friendly. The normal helptext is coloured, why not here too

## Acceptance Criteria

- [x] Move code examples from `Long` to `Example` field so they get colored (comments in yellow, commands in green)
- [x] Verify helptext shows colored examples when running `juggle pipeline --help`
- [x] Verify colors still respect NO_COLOR environment variable

## Completion Summary

Moved the two inline command examples from `pipelineCmd.Long` to `pipelineCmd.Example` in `internal/cli/pipeline.go`. The `groupedHelpWithColor` function already calls `colorizeExamples` on the `Example` field, which colors `#` comment lines yellow and command lines green, and already respects `NO_COLOR` via `isColorEnabled`. No new code needed.
