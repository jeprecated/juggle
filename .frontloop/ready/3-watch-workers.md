---
title: Watch mode workers (--workers N)
priority: medium
---

## Goal

Add `--workers N` flag for watch mode that processes multiple task files concurrently instead of serially. Each worker picks a different file from the watch directory.

## Acceptance Criteria

- `--workers N` (default 1) controls concurrent watch task processing
- Workers coordinate to never pick the same task file
- File claiming is atomic (rename to in-progress or use lock file)
- Each worker runs its own iteration loop for its claimed task
- Worker output is prefixed or separated to avoid interleaving
- When a worker finishes its task, it picks the next available file
- `--workers` without `--watch` is an error
- Tests verify no duplicate task selection and correct concurrency

## Design Decisions

- Juggle does NOT manage task lifecycle (no moving files to processing/done) — that's the user's task system (e.g., frontloop)
- Each worker gets its own JUGGLE_WORKER_ID env var (0-indexed)
- Workers just need to coordinate to not pick the same file; the task system handles state transitions

## Implementation Notes

- Semaphore pattern: buffered channel of size N
- ScanWatchDir returns files; workers claim via mutex to avoid double-pick
- The user's task system (frontloop, custom scripts) handles moving files between states (ready → in_progress → done)
- Juggle only reads from the watch dir; it never moves or renames files
