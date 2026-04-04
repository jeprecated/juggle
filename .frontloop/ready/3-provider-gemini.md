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
