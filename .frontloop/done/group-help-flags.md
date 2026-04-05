---
title: Group help flags into categories
priority: high
---

## Goal

Group the 40+ flags in `juggle --help` into logical categories so users can scan for what they need instead of reading a flat wall of flags.

## Acceptance Criteria

- Flags are grouped under clear headings (e.g., Lifecycle Hooks, Limits & Stopping, Agent Configuration, Output, Watch Mode)
- Grouping is implemented using cobra's flag group annotations or custom help template
- `juggle --help` output reads cleanly with visual separation between groups
- No flags are lost or duplicated

## Completion Summary

Implemented via pflag flag annotations and a custom cobra help function.

**Files changed:**
- `internal/cli/help.go` — new file; `flagGroupKey` constant, `groupOrder` slice, `setFlagGroup` helper (panics on unknown flag name), `groupedHelp` function that builds per-group `pflag.FlagSet` instances and calls `FlagUsages()` for aligned rendering.
- `internal/cli/juggle.go` — added group assignment calls after all flag definitions and wired `rootCmd.SetHelpFunc(groupedHelp)`.
- `internal/cli/help_test.go` — new file; TDD tests verifying group headings appear and all key flags are present.

**Groups produced:**
- Loop Control (10 flags)
- Agent Configuration (10 flags)
- Lifecycle Hooks (8 flags)
- Watch Mode (2 flags)
- Output (4 flags)
- Flags (ungrouped: --help, --no-config, --version)
