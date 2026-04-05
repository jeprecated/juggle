---
title: Include task file path in watch prompt
priority: medium
---

## Goal

Give the agent the local file path of the task file so it can read and update the file directly. Currently only the basename appears in the footer.

## Acceptance Criteria

- Task file's relative path (from working dir) appears as first line inside `<task>` block: `file: tasks/comments.json`
- Footer also uses the relative path instead of basename
- Falls back to absolute path if relative path can't be computed
- `BuildWatchPrompt` parameter renamed from `filename` to `taskRelPath`
- `runWatchTask` computes relative path instead of passing `filepath.Base(taskPath)`
- `JUGGLE_TASK_FILE` env var unchanged (already absolute)

## Design Decisions

- Path goes inside `<task>` block as first line (`file: {path}`), not as an attribute or separate section
- Relative path preferred over absolute for readability; absolute as fallback
