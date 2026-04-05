---
title: Glob watch paths with git root workdir
priority: high
---

## Goal

Allow `--watch` to accept glob patterns (e.g. `**/.frontloop/ready`) so a single juggle instance can watch for tasks across multiple repositories. When a task file is picked up, detect the git/jj root of that file and use it as the agent's working directory. `--workers` controls how many repos may have an active agent concurrently.

## Acceptance Criteria

- `--watch '**/.frontloop/ready'` expands the glob and watches all matching directories
- New directories matching the glob are discovered as they appear (not just at startup)
- Agent workdir is set to the VCS root (git or jj) of the matched task file
- `--workers N` caps how many repos can run an agent at the same time
- Existing single-directory `--watch ./tasks/` behavior is unchanged
