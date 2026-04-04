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

## Completion Summary

- Added `TypeCodex Type = "codex"` to provider.go; updated `IsValid()` to include it
- Updated `detect.go`: `BinaryName()` returns `"codex"`, `Get()` returns `NewCodexProvider()`, `ValidProviders()` includes `"codex"`
- Created `internal/agent/provider/codex.go` with `CodexProvider` implementing the `Provider` interface:
  - `MapModel`: small/haiku/medium/sonnet → `o4-mini`; large/opus → `o3`; pass-through otherwise
  - `MapPermission`: plan → `--approval-mode suggest`; acceptEdits → `auto-edit`; bypass → `full-auto`
  - `runHeadless`: uses `--quiet` flag, captures stdout/stderr; warns on unsupported flags (hooks, tools, max-turns, mcp-config)
  - `runInteractive`: inherits terminal stdin/stdout/stderr
  - `parseRateLimit`: OpenAI-specific quota/rate-limit patterns (shared with opencode)
- Added tests in `provider_test.go`: Type, MapModel, MapPermission, codexHeadlessArgs (basic/model/permission/passthrough), IsValid, BinaryName, Get, ValidProviders, Detect
- Updated existing `TestValidProviders` (2→3) and `TestType_IsValid` to include TypeCodex

### Files Changed

- `internal/agent/provider/provider.go` (modified)
- `internal/agent/provider/detect.go` (modified)
- `internal/agent/provider/codex.go` (new)
- `internal/agent/provider/provider_test.go` (modified)
- `.frontloop/in_progress/3-provider-codex.md` (modified)
