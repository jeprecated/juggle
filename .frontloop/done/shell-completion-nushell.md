---
title: Add Nushell shell completion
priority: low
---

## Goal

Add Nushell completion support to the `juggle completion` subcommand for tab completion of flags and subcommands in nu.

## Acceptance Criteria

- `juggle completion nushell` outputs a valid Nushell completion script
- Help text for `juggle completion` lists nushell as an option

## Implementation Notes

Cobra doesn't have built-in nushell support. Options: use carapace-bin integration, generate manually from cobra's command tree, or use a community cobra-nushell adapter.

## Completion Summary

- Added `genNushellCompletion` function that walks the cobra command tree and generates extern blocks with typed flag annotations
- Added `writeNushellExtern` helper and `nushellFlagType` to map pflag types to Nushell types
- Updated `completionCmd` Use field and error message to include `nushell`
- Added `nushell` case to `completionCmd` switch calling `genNushellCompletion`

### Files Changed

- `internal/cli/nushell.go` (new)
- `internal/cli/nushell_test.go` (new)
- `internal/cli/juggle.go` (modified)
