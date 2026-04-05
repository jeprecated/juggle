---
title: Add color to help text output
priority: medium
---

## Goal

Add color/ANSI styling to `juggle --help` output so headings, flag names, and examples are visually distinct and easier to scan.

## Acceptance Criteria

- Section headings are styled (e.g., bold or colored)
- Flag names are visually distinct from their descriptions
- Examples section has syntax highlighting or at minimum distinct coloring for commands vs comments
- Color is disabled when stdout is not a TTY (respects NO_COLOR convention)
- No external dependency added just for color (use ANSI codes directly or a lightweight lib already in the dep tree)

## Completion Summary

Added `internal/cli/color.go` with `isColorEnabled`, `bold`, `colorizeHeading`, `colorizeFlagUsages`, `colorizeExamples`. Refactored `help.go` to extract `groupedHelpWithColor(cmd, args, color bool)` — color auto-detected via TTY check and NO_COLOR env var. Tests cover all utilities and both colored/plain help variants.
