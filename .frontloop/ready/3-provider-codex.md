---
title: Codex CLI provider
priority: medium
---

## Goal

Add a provider implementation for the OpenAI Codex CLI. Research its flag interface, implement the Provider trait (Type, Run, MapModel, MapPermission), and add detection to detect.go.

## Acceptance Criteria

- `--provider codex` selects the Codex CLI provider
- MapModel maps canonical names (small/medium/large or haiku/sonnet/opus) to Codex model names
- MapPermission maps to Codex's equivalent permission/safety flags
- Headless mode captures output, interactive mode passes through terminal
- Binary detection via `exec.LookPath("codex")`
- Tests using mock pattern (same as existing provider tests)

## Implementation Notes

- Research Codex CLI flags and invocation pattern first
- Follow claude.go as the template for implementation
- Add TypeCodex to provider.go Type enum
