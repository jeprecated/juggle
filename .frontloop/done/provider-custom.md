---
title: Custom/generic agent provider (--agent-cmd)
priority: medium
---

## Goal

Add a generic provider that lets users define any agent CLI via a command template. This is the escape hatch for agents juggle doesn't have a built-in provider for.

## Acceptance Criteria

- `--provider custom --agent-cmd "my-agent --prompt {prompt} --model {model}"` defines the command
- Template variables substituted at runtime: `{prompt}`, `{model}`, `{timeout}`, `{workdir}`
- Stdout/stderr captured in headless mode, passed through in interactive
- No stream-json parsing — token counts are 0 (unavailable)
- `--agent-cmd` without `--provider custom` is an error (or auto-sets provider to custom)
- Exit code captured for rate limit / failure detection
- Tests verify template substitution and execution

## Implementation Notes

- Simple implementation: split template, substitute variables, exec
- No MapModel or MapPermission — user handles that in their template
- Prompt passed via temp file or stdin depending on `{prompt}` vs `{prompt_file}` variable

## Completion Summary

- Added `TypeCustom` provider type constant and updated `IsValid()`
- Implemented `CustomProvider` in `custom.go` with `buildCustomCmd` for template substitution (`{prompt}`, `{prompt_file}`, `{model}`, `{timeout}`, `{workdir}`)
- Headless mode captures stdout/stderr; interactive mode inherits terminal; no stream-JSON parsing (token counts = 0)
- Added `GetCustom(agentCmd)` to `detect.go`; `BinaryName` returns "" for custom; `ValidProviders` includes "custom"
- Added `--agent-cmd` flag to CLI; auto-sets `--provider custom` when `--agent-cmd` is specified; errors if `--provider custom` without `--agent-cmd`
- Added `AgentCmd` field to `Config`
- Updated `TestValidProviders` to expect 4 providers
- 21 new tests covering template substitution, execution, exit codes, timeout, token counts

### Files Changed

- `internal/agent/provider/custom.go` (new)
- `internal/agent/provider/custom_test.go` (new)
- `internal/agent/provider/provider.go` (modified — TypeCustom, IsValid)
- `internal/agent/provider/detect.go` (modified — Get, GetCustom, BinaryName, ValidProviders)
- `internal/agent/provider/provider_test.go` (modified — TestValidProviders count)
- `internal/cli/juggle.go` (modified — AgentCmd field, --agent-cmd flag, runner construction)
