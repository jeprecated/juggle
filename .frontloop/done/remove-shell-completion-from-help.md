---
title: Remove shell completion from help
priority: medium
---

## Goal

Remove the shell completion instructions block from the top of `juggle --help` output. It's noise for most users and the `juggle completion --help` subcommand already covers it.

## Acceptance Criteria

- Shell completion block no longer appears in `juggle --help`
- `juggle completion --help` still shows completion instructions

## Completion Summary

Removed the shell completion block from `rootCmd.Long` in `internal/cli/juggle.go`. Also deleted the now-obsolete `TestHelpLongContainsCompletion` test in `juggle_test.go`. The `juggle completion` subcommand and its help remain intact.
