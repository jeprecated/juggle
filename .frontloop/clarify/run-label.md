---
title: Run label (--label)
priority: low
---

## Goal

Add `--label "description"` flag that tags the run with a human-readable name. Appears in iteration headers, log output, and any notification hooks. If omitted, auto-generate from first ~50 chars of prompt.

## Acceptance Criteria

- `--label "refactor auth"` sets the label for the run
- Label appears in iteration header output
- Label included in JSONL log entries if `--log` is set
- Label passed as `JUGGLE_LABEL` environment variable to hooks and agents
- Auto-generated from prompt content when not provided
- Tests verify label in headers and env vars
