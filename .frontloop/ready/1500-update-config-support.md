---
title: Update config file support for loop/queue model
priority: 1500
---

## Goal

Update the `juggle.toml` config file support to match the new loop/queue command structure. Each config key should only apply to the command that accepts it.

## Context

See ADR at `docs/adr-001-loop-queue.md`, "Config file" section. The current `FileConfig` struct has keys that only apply to watch/serve but are loaded for all commands.

Depends on tasks 1000 and 1100 (new subcommands must exist).

## Acceptance Criteria

- Config key renamed: `every_immediate` → `now` (TOML key: `now`, type: `*bool`)
- Old `every_immediate` key is silently ignored — no warning, no alias, no backwards compat
- New config keys added:
  - `serve` (`*string`) — address like `:8080`
  - `on_touch` (`*bool`)
  - `dashboard` (`*bool`)
- Config keys `watch`, `on_touch`, `every`, `now`, `serve`, `workers`, `dashboard` only apply when the active subcommand is `queue`. If set in config but running `loop`, silently ignored.
- Config keys `delay` only applies when the active subcommand is `loop`. If set in config but running `queue`, silently ignored.
- Shared keys (model, provider, trust, id, etc.) apply to both commands as before
- `ApplyFileConfig` accepts a parameter indicating which command is active, so it can skip irrelevant keys
- Config tests updated for new structure

## Implementation Notes

- `ApplyFileConfig` should accept a `mode` string ("loop" or "queue") and conditionally apply keys
- The `now` field replaces `every_immediate`: update `FileConfig` struct, `ApplyFileConfig`, and all references
- Watch-related config (`watch`, `workers`) was previously applied to `watchFlags` — now it should apply to `queueFlags`
- Remove `every_immediate` from `FileConfig` struct entirely (no backwards compat)
- `iterations` is loop-only now, not shared

## Files to Change

- `internal/cli/config.go` — update `FileConfig`, `ApplyFileConfig`
- `internal/cli/config_test.go` — update tests
- `internal/cli/juggle.go` — update config loading in `runLoopCmd`
- Queue handler (`runQueueCmd`) — update config loading
