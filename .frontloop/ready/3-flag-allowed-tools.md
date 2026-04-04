---
title: Expose --allowed-tools / --disallowed-tools flags
priority: medium
---

## Goal

Add flags to restrict which tools the agent can use during iterations. Important safety measure for overnight runs — e.g., allow only Bash and Read, block Write.

## Acceptance Criteria

- `--allowed-tools Bash,Read,Grep` restricts agent to these tools only
- `--disallowed-tools Write,Edit` blocks specific tools
- Using both simultaneously is an error
- Passed to Claude Code as `--allowedTools` / `--disallowedTools`
- Mapped for other providers or ignored with verbose warning
- Tests verify flags passed to agent command

## Implementation Notes

- Add AllowedTools and DisallowedTools string fields to RunOptions
- Validate mutual exclusivity in Run() before entering loop
