---
title: More providers (Codex, Gemini, custom)
priority: medium
---

## Goal

Extend the provider abstraction to support more agent CLIs beyond Claude Code and OpenCode. Key targets: OpenAI Codex CLI, Google Gemini CLI, and a generic "custom" provider where the user defines the command template.

## Acceptance Criteria

- Codex provider: wraps the `codex` CLI with appropriate flag mapping
- Gemini provider: wraps the `gemini` CLI with appropriate flag mapping
- Custom provider: `--provider custom --agent-cmd "my-agent --prompt {prompt}"` lets users define any agent CLI
- Custom provider uses template variables: `{prompt}`, `{model}`, `{timeout}` substituted at runtime
- Provider auto-detection: if `--provider` not set, detect which CLIs are available on PATH
- All providers implement the existing Provider interface (Type, Run, MapModel, MapPermission)
- `juggle doctor` or similar can list available providers
- Tests for each new provider (mock-based, same pattern as existing tests)

## Design Decisions

- Custom provider is the escape hatch — any CLI that takes a prompt and produces output can be wrapped
- Custom provider doesn't need stream-json parsing — just capture stdout/stderr
- Token counting may not be available for all providers — fields are 0 when unavailable

## Implementation Notes

- Provider interface already supports this cleanly — just add new implementations
- detect.go's Get() factory needs extending for new types
- Custom provider: parse `--agent-cmd` template, substitute variables, exec
- Research Codex CLI and Gemini CLI flag interfaces before implementing
