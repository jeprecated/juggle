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
