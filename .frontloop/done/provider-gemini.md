---
title: Gemini CLI provider
priority: medium
---

## Goal

Add a provider implementation for the Google Gemini CLI. Research its flag interface, implement the Provider trait, and add detection.

## Acceptance Criteria

- `--provider gemini` selects the Gemini CLI provider
- MapModel maps canonical names to Gemini model names
- MapPermission maps to Gemini's equivalent flags
- Headless and interactive modes supported
- Binary detection via `exec.LookPath("gemini")`
- Tests using mock pattern

## Implementation Notes

- Research Gemini CLI flags and invocation pattern first
- Follow claude.go as template
- Add TypeGemini to provider.go Type enum

## Completion Summary

- Added `TypeGemini = "gemini"` to provider.go Type enum; updated `IsValid()` to include it
- Created `internal/agent/provider/gemini.go`: `GeminiProvider` implementing `Provider` interface with `MapModel`, `MapPermission`, `Run` (headless + interactive), `geminiHeadlessArgs`, and `parseRateLimit`
  - `MapModel`: small/haiku → gemini-2.5-flash; medium/sonnet/large/opus → gemini-2.5-pro; pass-through otherwise
  - `MapPermission`: bypass → `--yolo`; plan → `--sandbox`; acceptEdits/default → no flag
  - Headless: uses `-p <prompt>` flag; interactive: passes prompt as positional arg
- Updated `detect.go`: added `TypeGemini` to `BinaryName` ("gemini"), `Get`, and `ValidProviders`
- Created `internal/agent/provider/gemini_test.go` with full TDD test coverage
- Updated `TestValidProviders` expected count from 4 → 5

### Files Changed

- `internal/agent/provider/provider.go` (modified)
- `internal/agent/provider/gemini.go` (new)
- `internal/agent/provider/gemini_test.go` (new)
- `internal/agent/provider/detect.go` (modified)
- `internal/agent/provider/provider_test.go` (modified)
