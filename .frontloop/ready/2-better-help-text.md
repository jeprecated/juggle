---
title: Better help text and examples
priority: high
---

## Goal

Make juggle self-documenting for first-time users. Show the most important flags upfront, give concrete usage examples, and display help when `juggle` is run with no arguments.

## Acceptance Criteria

- `juggle` with no arguments shows help text (not just an error)
- Help text leads with 3-5 concrete examples showing real workflows:
  - Basic: `juggle "fix the failing tests" -n 3`
  - With prompt file: `juggle @task.md --trust -n 10`
  - Watch mode: `juggle --watch ./tasks/ @rules.md`
  - With hooks: `juggle --cmd-after "npm test" --stop-when "npm test" @task.md`
  - Multi-phase: `juggle --agent-after @tidy @task.md -n 5`
- Flags grouped by category (core, safety, hooks, output) not alphabetical
- Most important flags highlighted at top: -n, --trust, --watch, --model
- Less common flags collapsed or in a "more options" section
- Version shown in help output
- Shell completion instructions shown in help
- Tests verify help output contains examples

## Design Decisions

- Use cobra's built-in grouping and example features
- Show help on no-args instead of error (change Args validator behavior)

## Implementation Notes

- Cobra supports `cmd.Example` field for example blocks
- Cobra supports `cmd.GroupID` for flag grouping
- Current behavior: no-args returns error "requires at least 1 arg" — change to show help when no args AND no --watch
- Consider a short vs long help: `juggle --help` shows full, `juggle` with no args shows short with examples
