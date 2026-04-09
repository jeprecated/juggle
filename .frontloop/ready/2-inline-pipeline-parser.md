---
title: Parse inline pipeline syntax with agent and cmd nodes
priority: high
---

## Goal

Implement the inline pipeline parser for `juggle pipeline` so users can define workflows directly on the command line using repeated `agent` and `cmd` node blocks.

This task should convert CLI input into the canonical pipeline representation, but does not need full execution yet.

## Acceptance Criteria

- `juggle pipeline` accepts repeated node blocks using:
  - `agent <name> <prompt> [flags...]`
  - `cmd <name> <command> [flags...]`
- `agent` or `cmd` starts a new node block
- Tokens belong to the current node until the next `agent` or `cmd`
- Shared orchestration flags parse correctly:
  - `--event`
  - `--after`
  - `--parallel`
  - `--when`
  - `--on-failure`
  - `--retries`
  - `--timeout`
  - `--workdir`
- Agent-only flags parse correctly
- Cmd-only flags parse correctly
- Unknown or misplaced flags fail with useful errors
- Parsing results in the canonical pipeline model rather than ad hoc CLI-only structs
- Tests cover:
  - multiple adjacent node blocks
  - mixed `agent` and `cmd`
  - shared flags
  - kind-specific flags
  - malformed syntax

## Implementation Notes

- Standard Cobra subcommand parsing likely will not be enough by itself; use a custom parser after entering `pipeline` mode
- Keep parsing separate from validation so later tasks can reuse it for file-based specs

