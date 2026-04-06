---
title: Document all hooks and lifecycle in FEATURES.md
priority: medium
---

## Goal

Create a FEATURES.md documenting all of juggle's hook and lifecycle mechanisms so users can understand what's available and when each hook fires.

## Acceptance Criteria

- FEATURES.md at project root
- Documents all three hook categories:
  - Shell hooks: `--cmd-before`, `--cmd-after`, `--stop-when`
  - Agent phase hooks: `--agent-pre`, `--agent-before`, `--agent-after`, `--agent-post`
  - Session hooks: `--hook EVENT:CMD`, `--hooks-file`
- For session hooks, documents all supported Claude Code event types (PreToolUse, PostToolUse, SessionStart, Stop, etc.) with what each event means and when it fires
- Lifecycle diagram showing execution order
- Environment variables table (all JUGGLE_* vars, when set, when omitted)
- Failure behavior for each hook (what happens when it fails)
- Brief examples for each hook type, including session hook EVENT:CMD examples
- No implementation details — user-facing documentation only

## Implementation Notes

- Read Claude Code docs or session_hooks.go to enumerate all supported event types
- The EVENT in `--hook EVENT:CMD` maps to Claude Code's hook event system — document which events are available and what triggers them
