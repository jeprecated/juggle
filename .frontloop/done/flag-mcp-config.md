---
title: Expose --mcp-config flag
priority: medium
---

## Goal

Add `--mcp-config FILE` flag to specify an MCP server configuration file for the agent. Lets users provide custom tool servers to the agent without modifying project config.

## Acceptance Criteria

- `--mcp-config ./mcp.json` passes the MCP config to the agent CLI
- File path validated (must exist)
- Passed to Claude Code as `--mcp-config FILE`
- Mapped for other providers or ignored with verbose warning
- Tests verify flag passed to agent command

## Implementation Notes

- Add MCPConfig string to RunOptions
- Straightforward flag → arg mapping in claude.go

## Completion Summary

- Added `MCPConfig string` to `RunOptions` in `internal/agent/provider/provider.go`
- Added `MCPConfig string` to `Config` in `internal/cli/juggle.go`
- Added `mcpConfig string` to flags struct and registered `--mcp-config` flag in `init()`
- Added file-existence validation in `run()` before config is built
- Wired `MCPConfig` through `buildRunOptions()` in `internal/cli/watch.go`
- Passed `--mcp-config FILE` to Claude CLI in `claudeHeadlessArgs()` in `internal/agent/provider/claude.go`
- Added verbose warning for OpenCode provider in `internal/agent/provider/opencode.go`
- Added tests `TestClaudeHeadlessArgs_MCPConfig` and `TestClaudeHeadlessArgs_MCPConfig_Empty`

### Files Changed

- `internal/agent/provider/provider.go` (modified)
- `internal/agent/provider/claude.go` (modified)
- `internal/agent/provider/opencode.go` (modified)
- `internal/agent/provider/provider_test.go` (modified)
- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
