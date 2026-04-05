---
title: Add PowerShell shell completion
priority: low
---

## Goal

Add PowerShell completion support to the `juggle completion` subcommand so Windows users get tab completion for flags and subcommands.

## Acceptance Criteria

- `juggle completion powershell` outputs a valid PowerShell completion script
- Help text for `juggle completion` lists powershell as an option

## Completion Summary

- Added `internal/cli/powershell.go` with `genPowerShellCompletion` wrapping cobra's built-in `GenPowerShellCompletion`
- Added `internal/cli/powershell_test.go` with 3 tests (TDD: RED first)
- Updated `completionCmd` in `juggle.go`: added `powershell` to the `Use` string and switch case, updated error message
