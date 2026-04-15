---
title: Add --serve flag to queue as trigger source
priority: 1200
---

## Goal

Add a `--serve <addr>` flag to the `juggle queue` command. Serve is a pure trigger source: HTTP POSTs call `WriteTrigger`, the same function `juggle trigger` uses. No file writing, no temp directories.

## Context

See ADR at `docs/adr-001-loop-queue.md`, "Serve design" section. The queue subcommand (from task 1100) already exists with watch/every/now triggers. This task adds serve as another trigger source.

Depends on task 1100 (queue subcommand) being completed first.

## Acceptance Criteria

- `juggle queue @rules.md --serve :8080 --id myapp` starts an HTTP server that accepts POSTs and triggers runs via WriteTrigger
- `--serve` requires `--id`. Error if `--serve` is used without `--id`: "serve requires --id"
- POST body becomes the trigger message content (like `juggle trigger myapp "message"`)
- Returns 202 Accepted on success
- Returns 400 for non-POST methods
- Returns 400 for empty POST body
- `--serve` works with or without `--watch`. They are independent triggers.
- Parse logic for `--serve` value:
  - `"8080"` → `127.0.0.1:8080`
  - `":8080"` → `127.0.0.1:8080`
  - `"0.0.0.0:8080"` → `0.0.0.0:8080`
  - `"127.0.0.1:8080"` → as-is
- The old `newServeHandler(dir string)` that writes files to disk is removed
- `RunServe()` in `serve.go` is updated to use the trigger-based handler
- The old `serveCmd` cobra command still works during transition (it's removed in task 1400)
- All existing tests pass

## Implementation Notes

- The trigger mechanism uses `WriteTrigger(effectiveID, message)` which writes to a file in the session's `.d/` directory
- The queue loop already checks for triggers via `ReadTrigger(cfg.EffectiveID)` on each iteration — no new polling needed
- The HTTP handler needs access to `cfg.EffectiveID` to call `WriteTrigger`
- `newServeHandler` changes signature from `(dir string)` to `(effectiveID string)` and calls `WriteTrigger` instead of `os.WriteFile`
- The `serveSpecificFlags` struct can stay for now (removed in task 1400)
- For the HTTP handler, keep it simple: single endpoint, POST only, body = trigger message. URL path is ignored or accepted for backwards compat but doesn't affect behavior.

## Files to Change

- `internal/cli/serve.go` — rewrite `newServeHandler`, update `RunServe`
- `internal/cli/juggle.go` — add `--serve` flag to `queueFlags`, add serve wiring in `runQueueCmd`
- `internal/cli/trigger.go` — may need to export or adjust `WriteTrigger`/`ReadTrigger` if not already accessible
