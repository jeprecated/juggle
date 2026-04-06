---
title: Refactor watch from --watch flag to juggle watch subcommand
priority: high
---

## Goal

Convert `--watch` from a root-level flag into a `juggle watch` cobra subcommand. Move watch-specific flags (`--workers`, `--dashboard`) onto the subcommand. Shared run flags (iterations, provider, model, hooks, etc.) stay on the root and are inherited.

## Acceptance Criteria

- `juggle watch ./tasks/ "base prompt" -n 5` works identically to current `juggle "base prompt" --watch ./tasks/ -n 5`
- `--watch` flag removed from root command
- First positional arg is the watch directory, remaining positional args are prompt content
- `--dir` flag for additional watch directories (repeatable)
- `--workers` and `--dashboard` are flags on `watch`, not root
- Shared flags (`-n`, `--provider`, `--model`, hooks, etc.) inherited from root
- `juggle watch --help` shows watch-specific flags and inherited flags
- `juggle --help` shows `watch` as a subcommand
- All existing watch tests pass with updated invocation
- Do NOT update shell completions — that's handled in a separate task (5000)

## Design Decisions

- Positional args: first arg is watch directory, rest are prompt content (same @file resolution as root command)
- Multiple watch directories: `--dir` flag for extras, repeatable. Glob patterns also supported on the primary positional arg.
- `--` separator is reserved for passthrough to the agent, not for splitting dirs from prompts.

## Implementation Notes

- Cobra subcommand with `Args: cobra.MinimumNArgs(1)` — at least the watch directory
- `args[0]` is the watch dir, `args[1:]` are prompt content
- `--dir` appends to `cfg.Watch` alongside `args[0]`
- Shared flags registered on root `PersistentFlags()`, watch-specific on subcommand `Flags()`

## Completion Summary

- Added `watchCmd` cobra subcommand (`juggle watch <dir> [prompt...]`) with `--dir`, `--workers`, `--dashboard` flags
- Moved all shared flags from `rootCmd.Flags()` to `rootCmd.PersistentFlags()` so subcommands inherit them
- Removed `--watch`, `--workers`, `--dashboard` from root/flags struct; added `watchFlags` struct
- Added `runWatchSubcmd()` handler: `args[0]` is watch dir, `args[1:]` are prompt content, `--dir` adds extra dirs
- Updated `run()` to remove watch-specific logic (no more `--watch` on root)
- Updated `groupedHelpWithColor` to visit `Flags()`, `PersistentFlags()`, and `InheritedFlags()` for correct help display
- Updated root command example to use `juggle watch ./tasks/ @rules.md`
- Implemented `serve.go` helpers (`generateServeFilename`, `newServeHandler`, `serveCmd`) needed to unblock compilation
- Updated help tests to reflect new design (Watch Mode on subcommand, not root)
- Added 6 new cobra structure tests verifying the subcommand design

### Files Changed

- `internal/cli/juggle.go` (modified) — watchCmd, runWatchSubcmd, PersistentFlags refactor, flags struct
- `internal/cli/help.go` (modified) — visit InheritedFlags in groupedHelpWithColor
- `internal/cli/help_test.go` (modified) — updated for new help structure
- `internal/cli/juggle_test.go` (modified) — added cobra structure tests, updated examples check
- `internal/cli/serve.go` (new) — serve subcommand helpers and serveCmd implementation
