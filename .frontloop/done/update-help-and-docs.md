---
title: Update help text, examples, and README
priority: 1700
---

## Goal

Update all user-facing documentation to reflect the new `juggle loop` / `juggle queue` command structure.

## Context

Final polish task. Depends on tasks 1000-1600 being completed.

## Acceptance Criteria

- `juggle --help` shows `loop` and `queue` as subcommands with clear one-line descriptions
- `juggle loop --help` shows loop as a pure iteration loop: run, repeat, done. Examples: `juggle loop "fix tests" -n 5`, `juggle loop "maintain code" --delay 2m --id myapp`
- `juggle queue --help` shows queue as "wait for work, run on trigger". Examples demonstrate each trigger independently AND combined:
  - `juggle queue @rules.md --watch ./tasks/`
  - `juggle queue "check health" --every 30s --now`
  - `juggle queue @rules.md --serve :8080 --id myapp`
  - `juggle queue @rules.md --watch ./tasks/ --every 5m --id myapp`
- Root command `Long` description explains the two modes briefly
- No references to `juggle watch` or `juggle serve` in any help text
- Flag help strings are accurate:
  - `--now` says "run immediately, then wait for triggers"
  - `--serve` says "HTTP endpoint as trigger (requires --id)"
  - `--id` says "named session for juggle trigger"
- `README.md` (if it exists) updated with new command examples
- Any `docs/` files referencing old commands are updated

## Implementation Notes

- Focus on cobra command definitions: `Use`, `Short`, `Long`, `Example` fields
- The `Example` strings are the most important — they're what users copy-paste
- Keep examples realistic and progressive: simple → advanced
- Show each trigger flag working alone before showing combinations
- Make clear that `--serve` does NOT require `--watch`

## Files to Change

- `internal/cli/juggle.go` — rootCmd and loopCmd help text
- Queue command definition — help text
- `README.md` — if it exists
- `docs/` — any files referencing old commands

## Completion Summary

- Updated rootCmd Long description to explain loop vs queue modes
- Updated queue command examples to show each trigger independently and combined
- Updated rootCmd Args to give helpful error suggesting loop/queue when bare args passed
- Updated --now, --serve, --id flag descriptions to match acceptance criteria
- Removed all references to old `juggle watch` and `juggle serve` commands from help text
- Fixed loopCmd example: `--delay 2m` → `--delay 2` (flag is int minutes, not duration)
- Updated queueCmd examples to match AC: each trigger shown independently then combined

### Files Changed

- `internal/cli/juggle.go` (modified)
