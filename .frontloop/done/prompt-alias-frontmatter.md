---
title: Prompt alias support via frontmatter
priority: medium
---

## Goal

Allow prompt files in `$JUGGLE_PROMPTS` to declare aliases in YAML frontmatter. When a user types `@subagents`, it should resolve to `subagent.md` if that file declares `aliases: [subagents]`. This makes the prompt library more discoverable and forgiving — users don't need to remember exact filenames.

## Acceptance Criteria

- Prompt files can declare `aliases` in YAML frontmatter: `aliases: [subagents, sub]`
- `@subagents` resolves to `subagent.md` if it declares that alias
- Shell autocomplete suggests aliases alongside base names (e.g. typing `@sub` shows both `@subagent` and `@subagents`)
- Aliases are case-insensitive for matching
- If an alias collides with another file's base name, the base name wins (explicit > alias)
- If two files declare the same alias, error with a clear message naming both files
- Frontmatter is stripped from the resolved content (user gets the prompt body, not the YAML block)
- Files without frontmatter continue to work unchanged (no regression)
- Tests cover: alias resolution, alias autocomplete, collision with base name, duplicate alias error, no-frontmatter passthrough, frontmatter stripping

## Design Decisions

- Use `gopkg.in/yaml.v3` for frontmatter parsing (full YAML, supports future frontmatter fields)
- Autocomplete shows source hint for aliases: `@subagents (→ subagent)` so user knows it's an alias
- Frontmatter stripping happens inside `resolveFile` — callers always get clean prompt body
- Only files resolved from `$JUGGLE_PROMPTS` get frontmatter stripped (literal file paths are untouched)

## Completion Summary

- Added `gopkg.in/yaml.v3` dependency (vendored)
- New `internal/cli/frontmatter.go`: `parseFrontmatter()` splits YAML frontmatter from body
- Modified `internal/cli/resolve.go`: frontmatter stripped from `$JUGGLE_PROMPTS` files; `resolveByAlias()` scans for alias matches (case-insensitive, base name wins, duplicate alias = error)
- Modified `internal/cli/complete.go`: `addAliasCompletions()` appends `@alias\t(→ basename)` hints; alias skipped if base name already present
- 20 new tests; all pass; full suite green
