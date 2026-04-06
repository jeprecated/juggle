---
title: Update shell completions and help for subcommands
priority: medium
---

## Goal

Update shell completion generators and help output to reflect the new `watch` and `serve` subcommands.

## Acceptance Criteria

- `juggle --help` lists `watch` and `serve` as subcommands
- `juggle watch --help` and `juggle serve --help` show correct flags
- Bash, zsh, fish completions register both subcommands
- Nushell and PowerShell completions register both subcommands
- Help grouping in `help.go` updated for subcommand context
- Completions include subcommand-specific flag completion

## Completion Summary

- Added "Serve" to `groupOrder` in `help.go` so `--port` and `--bind` appear in the "Serve" group in `juggle serve --help`
- Added `TestRootHelpListsWatchAndServeSubcommands` to verify root help lists both subcommands
- Added `TestServeHelpShowsServeGroup` to verify serve help shows "Serve:" group with --port and --bind
- Added `TestServeHelpShowsWatchModeGroup` to verify serve help shows "Watch Mode:" group with --workers and --dashboard
- Added nushell test `includes_watch_and_serve_for_juggle_root_command` confirming watch/serve externs are generated
- Bash, zsh, fish, and PowerShell completions already worked via cobra's built-in generators (no changes needed)

### Files Changed

- `internal/cli/help.go` (modified — added "Serve" to groupOrder)
- `internal/cli/help_test.go` (modified — added 3 new tests)
- `internal/cli/nushell_test.go` (modified — added 1 new test)
