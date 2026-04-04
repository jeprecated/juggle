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
