---
title: Document UTC assumption in quota reset time parsing
priority: low
---

## Goal

`parseQuotaResetTime()` hardcodes `time.UTC` for absolute reset times like "resets at 16:00 PST". The timezone token in the string is ignored. In practice APIs report UTC, so this is unlikely to cause issues, but the assumption should be documented.

## Problem

`claude.go:459` — `time.Date(..., time.UTC)` ignores any timezone present in the error message.

## Acceptance Criteria

- Add a comment at line 459 documenting that absolute times are assumed UTC
- Add a comment noting that API providers typically report UTC
- If simple to implement: parse timezone with time.LoadLocation() as a best-effort improvement
- If timezone parsing adds complexity, the comment is sufficient

## Completion Summary

Added a two-line comment before `time.Date(...)` in `parseQuotaResetTime()` documenting the UTC assumption and that API providers consistently report UTC. Timezone parsing was skipped as Go's `time.LoadLocation` doesn't handle abbreviations like "PST" directly, which would add non-trivial complexity for negligible practical benefit.
