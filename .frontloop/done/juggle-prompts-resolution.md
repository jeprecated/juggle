---
title: JUGGLE_PROMPTS env var resolution for bare @names
priority: high
---

## Goal

Allow `@TDD` to resolve to `$JUGGLE_PROMPTS/TDD.md` so users don't need full paths to their prompt parts directory.

## Acceptance Criteria

- Bare `@name` (no `/`) checks `$JUGGLE_PROMPTS/<name>` when literal path fails
- Falls back to `$JUGGLE_PROMPTS/<name>.md` if no extension given
- Full paths like `@./local/file.md` are unaffected
- When `$JUGGLE_PROMPTS` is unset, behavior is unchanged (original error)
- All cases covered by tests in `resolve_test.go`

## Design Decisions

- Env var (`JUGGLE_PROMPTS`), not a config file — juggle has no config file and shouldn't get one for this
- Literal path always wins (backwards compat)
- Only bare names trigger lookup — presence of `/` means it's a path
- `.md` auto-suffix is convenience, not magic — explicit `.md` also works

## Completion Summary

- Added `resolveFile()` helper in `internal/cli/resolve.go` with fallback chain: literal → `$JUGGLE_PROMPTS/name` → `$JUGGLE_PROMPTS/name.md`
- 7 new tests covering: bare name, explicit .md, auto-suffix, literal wins, path with /, unset env, exact vs .md precedence
- All 208 tests pass
