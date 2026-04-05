---
title: Verbose dry-run output with all agents and hooks
priority: medium
---

## Goal

Make `--dry-run` show what each agent/hook would run, with clear separators and colour, so users can verify a complex config before running it.

## Acceptance Criteria

- `--dry-run` shows the main prompt (as today) plus all configured phase agents (agent-pre, agent-before, agent-after, agent-post)
- Each section has a coloured header/separator (using existing ANSI colour helpers, respecting NO_COLOR)
- Watch mode prompts are shown when `--watch` is set (using a sample task or placeholder)
- Shell hooks (cmd-before, cmd-after, stop-when) are displayed with their commands
- Session hooks (--hook) are listed
- Output order matches execution order: agent-pre -> cmd-before -> agent-before -> main prompt -> agent-after -> cmd-after -> stop-when -> agent-post
- If a section is not configured, it is omitted (no empty placeholders)
