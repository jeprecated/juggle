---
title: Make bare juggle show help instead of running
priority: 1300
---

## Goal

Change the root `juggle` command so that bare invocations without a subcommand show help text. Currently `juggle "prompt"` runs the loop — after this task, it should error and show help.

## Context

See ADR at `docs/adr-001-loop-queue.md`. Once `loop` and `queue` subcommands exist, bare `juggle` should require a subcommand.

Depends on tasks 1000 and 1100 being completed first (both new subcommands must exist).

## Acceptance Criteria

- `juggle` with no args → shows help text (same as `juggle --help`)
- `juggle "some prompt"` with no subcommand → error message suggesting `juggle loop` or `juggle queue`, plus help text
- `juggle loop "prompt"` → works as expected
- `juggle queue "prompt" --watch ./tasks` → works as expected
- `juggle --version` → still works
- `juggle completion bash` → still works
- `juggle trigger myapp "message"` → still works
- Root command no longer accepts prompt args — `Args: cobra.NoArgs` (or `cobra.ArbitraryArgs` with custom validation that errors)
- The old `run()` handler on rootCmd is removed or replaced with help/error logic
- All existing tests for `juggle loop` and `juggle queue` pass
- Tests for old bare `juggle "prompt"` are updated to use `juggle loop "prompt"`

## Implementation Notes

- Change `rootCmd.Args` from `cobra.ArbitraryArgs` to `cobra.NoArgs`
- Change `rootCmd.RunE` to return an error suggesting subcommands, or just let cobra's default behavior show help
- The `rootCmd.Example` should be updated to show `juggle loop` and `juggle queue` examples
- The `rootCmd.Short` and `rootCmd.Long` should be updated to describe the two modes
- Consider: `rootCmd.RunE = func(cmd *cobra.Command, args []string) error { return cmd.Help() }` for bare invocation
- Tests that call `run()` directly (not through cobra) need to be updated to use `runLoopCmd()` instead

## Files to Change

- `internal/cli/juggle.go` — update `rootCmd` definition, remove or replace `run()` handler
- `internal/cli/juggle_test.go` — update tests that use bare invocation
- `internal/cli/help_test.go` — update help output tests
