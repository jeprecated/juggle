---
title: Add tab completion for local prompt files
priority: medium
---

## Goal

Make local prompt files discoverable via shell tab completion when using `@` prefix args.

## Acceptance Criteria

- [ ] Tab completion for `@` args suggests `.md` files from JUGGLE_PROMPTS dir (existing env var)
- [ ] Tab completion also discovers `.md` files in any subdirectory whose name contains "prompts" (e.g., `./prompts/`, `./docs/prompts/`, `.juggle/prompts/`) — but not dirs like `./docs/plans/`
- [ ] Works in bash, zsh, and fish (existing completion infrastructure)
- [ ] No new CLI flags or subcommands needed — `@file` resolution itself already works

## Design Decisions

- Discovery uses existing `@file` resolution — no new invocation mechanism needed
- Prompt file content is passed as-is (no frontmatter/metadata parsing)
- Discovery is tab-completion only — no explicit `juggle list` command
- Search paths: JUGGLE_PROMPTS env var dir + any subdirectory matching `*prompts*` in the repo

## Implementation Notes

- Extend the existing shell completion logic in `internal/cli/complete.go`
- Match directories by checking if the directory name contains the substring "prompts" (case-insensitive)
- Walk from the repo/cwd root, pruning hidden dirs and non-matching subtrees
