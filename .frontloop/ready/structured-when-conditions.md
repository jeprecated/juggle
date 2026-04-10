---
title: Implement structured when-condition evaluator
priority: medium
---

## Goal

Implement the V1 condition grammar from the design doc as an alternative to raw shell `when` commands. The design doc specifies:

- `iteration==N`, `iteration!=N`, `iteration>N`, `iteration>=N`, `iteration<N`, `iteration<=N`
- `success==true`, `success==false`
- `exit_code==N`

Currently `evalWhen` in `executor.go` shells out via `sh -c`. The structured grammar should be evaluated in-process for safety and speed.

## Acceptance Criteria

- When conditions matching the grammar are evaluated in-process (no shell)
- Shell-based conditions still work when the expression doesn't match the grammar (fallback)
- `iteration` comparisons use the current loop iteration number
- `success` and `exit_code` reference the result of the previous node (requires passing state)
- Parsing errors in structured conditions produce clear error messages
- Tests cover: all six iteration operators, success/exit_code checks, fallback to shell, parse errors

## Implementation Notes

- Add `internal/pipeline/when.go` with a small evaluator
- `evalWhen` should try the structured parser first; if it doesn't match, fall back to shell
- Consider a `WhenContext` struct to carry iteration, previous success, previous exit code
- Keep it simple: no boolean composition in V1 per the design doc
