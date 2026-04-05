---
title: Add tab completion for local prompt files
priority: medium
---

## Goal

Make local prompt files discoverable via shell tab completion when using `@` prefix args.

## Acceptance Criteria

- [x] Tab completion for `@` args suggests `.md` files from JUGGLE_PROMPTS dir (existing env var)
- [x] Tab completion also discovers `.md` files in any subdirectory whose name contains "prompts" (e.g., `./prompts/`, `./docs/prompts/`, `.juggle/prompts/`) — but not dirs like `./docs/plans/`
- [x] Works in bash, zsh, and fish (existing completion infrastructure)
- [x] No new CLI flags or subcommands needed — `@file` resolution itself already works

## Design Decisions

- Discovery uses existing `@file` resolution — no new invocation mechanism needed
- Prompt file content is passed as-is (no frontmatter/metadata parsing)
- Discovery is tab-completion only — no explicit `juggle list` command
- Search paths: JUGGLE_PROMPTS env var dir + any subdirectory matching `*prompts*` in the repo

## Completion Summary

Added `findPromptDirs(root string) []string` to `internal/cli/complete.go` that walks from cwd, finds subdirs whose name contains "prompts" (case-insensitive), skipping hidden dirs and not descending into matched dirs. Extended `completeArgs` to call this walker and include `.md` files from discovered dirs, deduplicating with any results from `$JUGGLE_PROMPTS`. All existing tests pass; 10 new tests cover the new behaviour.
