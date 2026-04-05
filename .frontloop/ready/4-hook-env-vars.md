---
title: Add run-level env vars to hook commands
priority: low
---

## Goal

Hook commands (--cmd-before, --cmd-after, --stop-when) currently only receive JUGGLE_ITERATION, JUGGLE_MAX_ITERATIONS, JUGGLE_EXIT_CODE, JUGGLE_INPUT_TOKENS, JUGGLE_OUTPUT_TOKENS. They should also receive JUGGLE_RUN_ID, JUGGLE_LABEL, JUGGLE_MODEL, JUGGLE_PROVIDER so hook scripts can correlate with the log file or tag notifications.

## Problem

`hooks.go:23-30` — `hookEnv` struct and `envSlice()` are missing run-level fields.

## Acceptance Criteria

- Add runID, label, model, provider fields to hookEnv struct
- Update envSlice() to include JUGGLE_RUN_ID, JUGGLE_MODEL, JUGGLE_PROVIDER
- Include JUGGLE_LABEL only when non-empty
- Update all calling sites in juggle.go and watch.go to pass run-level values
- Tests verify run-level vars are present in hook env
