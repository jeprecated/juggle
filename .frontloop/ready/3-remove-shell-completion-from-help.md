---
title: Remove shell completion from help
priority: medium
---

## Goal

Remove the shell completion instructions block from the top of `juggle --help` output. It's noise for most users and the `juggle completion --help` subcommand already covers it.

## Acceptance Criteria

- Shell completion block no longer appears in `juggle --help`
- `juggle completion --help` still shows completion instructions
