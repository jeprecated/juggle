---
title: Working directory flag (--workdir)
priority: medium
---

## Goal

Add `--workdir path/to/repo` so juggle spawns the agent with that directory as its working directory instead of cwd. Useful for CI, cron, and orchestration scripts.

## Acceptance Criteria

- `--workdir DIR` sets the agent's working directory
- Directory must exist, error if not
- Affects agent spawning only — juggle itself stays in its original cwd
- Watch directory paths are relative to workdir if not absolute
- @file resolution still uses juggle's cwd (not workdir) for prompt files
- Passed through as RunOptions.WorkingDir (field already exists)
- Tests verify agent runs in specified directory

## Implementation Notes

- WorkingDir field already exists on RunOptions — just need to wire the flag through
- Set cmd.Dir in provider Run() implementations
