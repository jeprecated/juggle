---
title: Agent-internal session hooks (--hook, --hooks-file)
priority: medium
---

## Goal

Allow juggle to configure hooks on the spawned agent's own hook system. For example, Claude Code supports hooks like `PostToolUse` and `Stop` — a bash script that `echo`s a message effectively adds a user prompt into the session that the agent sees and acts on.

This lets users inject behavior INSIDE the agent session (e.g., "Remember to commit your changes") without juggle needing to know about it — juggle just passes the hook config to the agent CLI.

## CLI Syntax

Two flags, hybrid approach:

```bash
# Simple: --hook EVENT:CMD (repeatable)
juggle --hook "Stop:echo Remember to commit" @task.md

# @file resolution works in hook commands (JUGGLE_PROMPTS → cwd)
juggle --hook "Stop:@commit-reminder" @task.md

# Complex: full JSON file for matchers, timeouts, async
juggle --hooks-file ./hooks.json @task.md

# Both together
juggle --hook "Stop:@commit-reminder" --hooks-file ./extras.json @task.md
```

The `@` prefix in hook commands reuses existing ResolveArgs resolution: check `$JUGGLE_PROMPTS` dir first, then cwd. Consistent with how prompt `@file` references work.

## Acceptance Criteria

- `--hook "EVENT:CMD"` specifies a simple hook (repeatable flag)
- `--hooks-file FILE` specifies a full Claude Code hooks JSON file
- `@` references in hook commands resolve via JUGGLE_PROMPTS → cwd (same as prompt @files)
- Multiple --hook flags can target different events or same event
- Both flags can be used together (merged into single settings overlay)
- Works in both RunLoop and RunWatch
- Tests verify hook config is passed to agent process
- Tests verify @file resolution in hook commands

## Design Decisions

- Claude Code supports `--settings <json-or-file>` to merge temporary settings per invocation
- This is provider-specific — only Claude Code has this hook system. OpenCode may have its own mechanism.
- The Provider interface needs a way to accept hook config (e.g., WithHooks or extra RunOptions field)

## Implementation Notes

- Parse `--hook` flags: split on first `:` → event name + command
- If command starts with `@`, resolve via ResolveArgs (read file contents as the command)
- Generate temp JSON settings file combining all --hook flags and --hooks-file content
- Pass to Claude Code via `--settings /tmp/juggle-hooks-<run-id>.json`
- `--settings` merges with existing user/project settings (doesn't replace)
- 23 hook events available: SessionStart, SessionEnd, PreToolUse, PostToolUse, Stop, etc.
- Clean up temp settings file after iteration completes
