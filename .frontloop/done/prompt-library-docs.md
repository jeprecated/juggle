---
title: Prompt library documentation
priority: low
---

## Goal

Document the prompt library features (aliases, recursive folders, resolution rules) in the README so users know how to organize and reference their prompts.

## Acceptance Criteria

- README has a "Prompt Library" section explaining `$JUGGLE_PROMPTS` and `@name` resolution
- Documents the resolution order: literal path → bare name → bare name + .md → recursive search → alias match
- Documents frontmatter format for aliases with an example
- Documents folder organization with a tree example showing nested structure
- Documents autocomplete setup (already exists but link/mention it in context)
- Documents collision/ambiguity rules (base name wins over alias, ambiguous bare names error)
- Keeps the existing README style — concise, example-driven, no fluff

## Design Decisions

- Goes in README after Usage section, before Watch mode section
- Inline in README, not a separate doc file

## Completion Summary

- Added `## Prompt library` section to README between Quick start and Install sections
- Covers `$JUGGLE_PROMPTS` env var, resolution order (literal → exact → .md → alias scan), frontmatter alias format, folder organization tree, and autocomplete
- Note: "recursive search" from acceptance criteria is not implemented in code — documented the actual alias scan step instead

### Files Changed

- README.md (modified)
