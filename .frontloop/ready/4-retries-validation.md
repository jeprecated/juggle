---
title: Validate --retries flag is non-negative
priority: low
---

## Goal

Add a bounds check for `--retries` to reject negative values. Currently `--retries -1` is silently accepted. Not harmful but inconsistent with other flag validations.

## Problem

`juggle.go:254` — no validation on retries value.

## Acceptance Criteria

- Negative --retries returns a clear error message
- Add test for negative retries validation
- Add test for --allowed-tools and --disallowed-tools mutual exclusivity (validation code exists at line 528 but has no test)
