---
title: Expose useful Claude Code flags
priority: medium
---

## Goal

Expose select Claude Code CLI flags that are commonly useful for Ralph Loop workflows. Currently juggle only exposes model, trust, thinking, and verbose. Other useful Claude flags should be available without resorting to `--` pass-through.

## Acceptance Criteria

Expose the following Claude Code flags through juggle:

- `--max-turns N` → `--max-turns` — cap the number of tool-use turns per iteration (prevents runaway single iterations)
- `--allowed-tools LIST` → `--allowed-tools` — restrict which tools the agent can use (safety for overnight runs)
- `--disallowed-tools LIST` → `--disallowed-tools` — block specific tools
- `--permission-mode plan` → `--plan` flag (shortcut, in addition to existing --trust for bypass)
- `--mcp-config FILE` → `--mcp-config` — specify MCP server config for the agent
- `--settings FILE` → already planned via --hooks-file, but also useful standalone

Each flag maps to the appropriate provider-specific flag. If a provider doesn't support a flag, it's silently ignored with a verbose-mode warning.

## Design Decisions

- Only expose flags that are genuinely useful across multiple providers or for safety
- Provider-specific flags should use `--` pass-through instead
- Flags that only make sense for one provider get a verbose warning when used with others

## Implementation Notes

- Add fields to RunOptions for each new flag
- Update claude.go and opencode.go to map them
- --max-turns is particularly important for Ralph Loops — prevents a single iteration from consuming the entire context window and budget
