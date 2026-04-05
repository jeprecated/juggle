---
title: Recursive prompt folder discovery
priority: medium
---

## Goal

Support nested subdirectories inside `$JUGGLE_PROMPTS` so users can organize prompts into folders (e.g. `workflows/`, `reviews/`, `templates/`). Resolution and autocomplete should walk the tree recursively.

## Acceptance Criteria

- `@fix` resolves by searching `$JUGGLE_PROMPTS` recursively, not just the top level
- Subdirectory prompts can also be referenced explicitly: `@workflows/fix` resolves to `$JUGGLE_PROMPTS/workflows/fix.md`
- Shell autocomplete lists prompts from all subdirectories, prefixed with their relative path (e.g. `@workflows/fix`, `@reviews/code-review`)
- Bare name `@fix` does a recursive search; if multiple files match the same bare name, error with a clear message listing all candidates
- Explicit path `@workflows/fix` is unambiguous and skips bare-name search
- Aliases (from the frontmatter task) work across subdirectories too
- Hidden directories (starting with `.`) are skipped
- Tests cover: nested resolution, explicit path resolution, root-wins precedence, hidden directory skipping, interaction with aliases, fuzzy autocomplete matching

## Design Decisions

- Root-level files win silently over nested files with the same bare name (no ambiguity error)
- No depth limit — walk the full tree
- Autocomplete uses fuzzy matching: typing `@fix` then tab shows `@workflows/fix` if that's where it lives (not just prefix matching)
- Explicit paths (`@workflows/fix`) skip bare-name search entirely
