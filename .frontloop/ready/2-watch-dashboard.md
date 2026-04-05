---
title: Watch mode dashboard overview
priority: high
---

## Goal

Add a TUI dashboard that shows an overview of running watch workers, their current task, and iteration stats. Available in any watch mode (single dir or glob), activated by default when glob watch would make raw output unreadable. Replaces interleaved agent output with a scannable status screen.

## Acceptance Criteria

- Dashboard shows: repo/watch dir, worker status (active/idle), current task name, iteration progress
- Available in single-directory watch mode (opt-in via flag)
- Default behavior for glob watch patterns
- Updates in real time as iterations complete and tasks change
- Provides a way to drill into or tail a specific worker's output if needed
