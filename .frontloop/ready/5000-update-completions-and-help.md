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
