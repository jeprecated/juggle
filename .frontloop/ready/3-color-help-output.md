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
