---
title: Create juggle loop subcommand
priority: 1000
---

## Goal

Create a new `juggle loop [prompts] [flags]` cobra subcommand that replaces the current bare `juggle [prompts]` invocation. This is the "run immediately, keep running" mode.

## Context

See ADR at `docs/adr-001-loop-queue.md`. This is the first step of the consolidation. We're adding the new command alongside the existing ones — removal comes later.

## Acceptance Criteria

- `juggle loop "do the thing" -n 5` works identically to current `juggle "do the thing" -n 5`
- `juggle loop` has its own cobra command definition with `Use: "loop [prompt-content...]"`
- Loop-specific flags registered directly on `loopCmd`: `--delay`, `-n`/`--iterations`, `--resume`, `--continue`
- Shared flags registered via `registerSharedFlags(cmd)` helper — this registers `--id` plus all agent/hook/output flags
- The `registerSharedFlags` helper registers: `--id`, `--model`, `--provider`, `--trust`, `--plan`, `--timeout`, `--max-wait`, `--dry-run`, `--show-thinking`, `-v`/`--verbose`, `--max-failures`, `--cmd-before`, `--cmd-after`, `--stop-when`, `--agent-pre`, `--agent-before`, `--agent-after`, `--agent-post`, `--hook`, `--hooks-file`, `--log`, `--max-cost`, `--label`, `--allowed-tools`, `--disallowed-tools`, `--max-turns`, `--mcp-config`, `--on-failure`, `--retries`, `--agent-cmd`, `--command`, `--system-prompt`, `--retry-prompt`, `--workdir`, `--channels`, `-X`/`--extra`, `--no-config`, `--no-log`
- Hidden flags: `--fuzz`, `--interactive`, `--show-thinking`, `--provider`
- Help groups: "Loop Control" (delay, iterations, resume, continue, timeout, max-wait, max-failures, stop-when, max-cost, on-failure, retries, retry-prompt), "Agent Configuration", "Lifecycle Hooks", "Output"
- `juggle loop --help` shows all flags correctly grouped
- The handler `runLoopCmd()` calls the same logic as current `run()` — stdin reading, config file loading, prompt resolution, phase content building, runner construction, signal handling, session setup, keypress listener, and finally `Run(cfg)`
- `loopCmd` is added to `rootCmd`
- Old `rootCmd` behavior (bare `juggle "prompt"`) still works during transition — we're NOT removing it yet
- All existing tests pass (no behavior changes to existing commands)

## Implementation Notes

- New command definition goes in `internal/cli/juggle.go` near the existing `watchCmd` definition
- The `registerSharedFlags` function should live in `internal/cli/juggle.go` — it takes a `*cobra.Command` and registers all shared flag groups including `--id`
- The current `flags` struct and `rootCmd.PersistentFlags()` setup in `init()` stays for now — `loopCmd` gets its own flags via `registerSharedFlags` + direct registration
- The handler can initially be a thin wrapper that calls the existing `run()` function with appropriate arg massaging
- Look at how `watchCmd` registers its own flags (line ~327 in juggle.go) for the pattern

## Files to Change

- `internal/cli/juggle.go` — add `loopCmd`, `registerSharedFlags`, `runLoopCmd`

## Completion Summary

- `loopCmd` cobra command defined with `Use: "loop [prompt-content...]"` and examples
- `registerSharedFlags(cmd)` helper registers --id plus all agent/hook/output flags on any command
- Loop-specific flags (--delay, -n/--iterations, --resume, --continue, --fuzz) registered directly on loopCmd
- Hidden flags: --fuzz, --interactive, --show-thinking, --provider
- Help groups: Loop Control, Agent Configuration, Lifecycle Hooks, Output
- `runLoopCmd()` delegates to existing `run()` for identical behavior
- `loopCmd` added to `rootCmd`; bare `juggle "prompt"` still works
- Comprehensive tests in `loop_test.go` covering command existence, flags, help groups, and execution

### Files Changed

- `internal/cli/juggle.go` (modified) — added loopCmd, registerSharedFlags, runLoopCmd
- `internal/cli/loop_test.go` (new) — tests for loop subcommand
