---
title: Shell completion for @args from JUGGLE_PROMPTS
priority: medium
---

## Goal

When typing `juggle @TD<tab>`, zsh (and bash/fish) suggest matching files from `$JUGGLE_PROMPTS` directory.

## Acceptance Criteria

- Cobra `ValidArgsFunction` on rootCmd handles `@`-prefixed completions
- Lists files from `$JUGGLE_PROMPTS` when env var is set
- Filters by case-insensitive prefix match on the partial after `@`
- Returns matches prefixed with `@`
- Empty or non-`@` args get no file completions (prompt text isn't completable)
- `juggle completion zsh` outputs a working completion script
- Completion function has tests

## Design Decisions

- New file `internal/cli/complete.go` — keeps completion logic separate from core CLI
- Uses `ShellCompDirectiveNoSpace | ShellCompDirectiveNoFileComp` — we handle file listing ourselves
- No completion when `$JUGGLE_PROMPTS` is unset — falls back to nothing rather than guessing

## Completion Summary

- Created `internal/cli/complete.go` with `completeArgs()` — case-insensitive prefix matching from `$JUGGLE_PROMPTS`
- Added `ValidArgsFunction: completeArgs` to rootCmd in `internal/cli/juggle.go`
- Created `internal/cli/complete_test.go` with 6 tests covering: non-@ args, empty env, prefix matching, case-insensitive, hidden/dir filtering, base name output
- Verified `juggle completion zsh` outputs valid completion script
- All 215 tests pass
