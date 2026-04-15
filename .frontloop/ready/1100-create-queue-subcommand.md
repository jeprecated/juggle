---
title: Create juggle queue subcommand
priority: 1100
---

## Goal

Create a new `juggle queue [prompts] [flags]` cobra subcommand that replaces `juggle watch`. This is the "wait for work, run on trigger" mode. Does NOT include `--serve` yet (that's task 1200).

## Context

See ADR at `docs/adr-001-loop-queue.md`. Depends on task 1000 (loop subcommand) being completed first, because queue uses the same `registerSharedFlags` helper.

## Acceptance Criteria

- `juggle queue @rules.md --watch ./tasks/` works identically to current `juggle watch ./tasks/ @rules.md`
- `juggle queue` has its own cobra command with `Use: "queue [prompt-content...]"`
- Queue flags registered on `queueCmd`:
  - `--watch <path>` — repeatable, replaces the old positional watch dir argument
  - `--on-touch` — trigger on mtime change
  - `--every <dur>` — run on fixed interval
  - `--now` — run immediately, then wait for triggers
  - `--workers N` — parallel workers (default 1)
  - `--dashboard` — TUI dashboard
- Queue does NOT accept: `--delay`, `-n`/`--iterations`, `--resume`, `--continue`
- Shared flags registered via `registerSharedFlags(cmd)` helper (from task 1000) — includes `--id`
- Help groups: "Agent Configuration", "Lifecycle Hooks", "Output", "Queue Mode" (watch, on-touch, every, now, workers, dashboard)
- Validation: `queue` without any trigger flag (`--watch`, `--every`, or `--id`) returns an error: "queue requires at least one trigger: --watch, --every, or --id"
- The handler `runQueueCmd()` calls the same logic as current `runWatchSubcmd()` — stdin reading, config file loading, prompt resolution, glob expansion detection, phase content building, runner construction, signal handling, session setup, keypress listener
- Queue stops on: SIGINT, `--stop-when`, `--max-cost`, `--max-failures`
- `queueCmd` is added to `rootCmd`
- Old `watchCmd` and `serveCmd` still work during transition
- All existing tests pass

## --now Details

Rename `--every-immediate` / `EveryImmediate` to `--now` / `Now` throughout:
- `watchFlags.everyImmediate` → `queueFlags.now` (or similar)
- `Config.EveryImmediate` → `Config.Now`
- `FileConfig.EveryImmediate` → `FileConfig.Now` (TOML key: `now`)
- All watch loop logic that checks `cfg.EveryImmediate` → check `cfg.Now`

## Trigger Interaction

When multiple triggers fire concurrently, first-come-first-served: whatever trigger is detected first runs. Others are picked up on subsequent iterations. This is the same as current behavior — no coalescing.

## Implementation Notes

- New `queueFlags` struct replaces `watchFlags` — holds: `watch []string`, `onTouch bool`, `every time.Duration`, `now bool`, `workers int`, `dashboard bool`
- The handler builds `Config` with `Watch` set from `--watch` flags (not from a positional arg)
- Prompt content comes only from positional args (no positional watch dir anymore)
- The `detectShellGlobExpansion` function needs updating: it currently checks `watch[0]` against positional args. Now `watch` comes from `--watch` flags and there are no positional args to compare against. Either adapt it or remove it.
- `-n`/`--iterations` is NOT registered on queue. Queue has no iteration cap.

## Files to Change

- `internal/cli/juggle.go` — add `queueCmd`, `queueFlags`, `runQueueCmd`
- `internal/cli/watch.go` — rename `EveryImmediate` → `Now` in all loop variants (serial, worker, touch)
- `internal/cli/config.go` — rename `EveryImmediate` → `Now` in `FileConfig` and `ApplyFileConfig`
