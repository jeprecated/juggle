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
