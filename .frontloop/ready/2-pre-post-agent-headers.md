---
title: Show headers for all lifecycle steps (phase agents and hooks)
priority: high
---

## Goal

All lifecycle steps run silently — there's no visual indicator when --agent-pre, --agent-before, --agent-after, --agent-post, --cmd-before, or --cmd-after execute. The user only sees output on failure. Add clear header/marker lines before each lifecycle step so users can tell what's happening.

## Affected Code Paths

In juggle.go (RunLoop):
- agent-pre: line 614 (no header)
- agent-before: line 633 (no header)
- agent-after: line 808 (no header)
- agent-post: line 871 (no header)
- cmd-before: line 642 (no header)
- cmd-after: line 816 (no header)

In watch.go (runWatchTask):
- agent-pre: line 215 (no header)
- agent-before: line 233 (no header)
- agent-after: line 398 (no header)
- agent-post: line 458 (no header)
- cmd-before: line 241 (no header)
- cmd-after: line 405 (no header)

## Acceptance Criteria

- A header is printed to stderr before each phase agent runs, e.g. "── Agent Pre ──", "── Agent Before ──"
- A smaller/dimmer marker is printed before cmd-before/cmd-after, e.g. "  cmd-before: <cmd>"
- Same styling approach as iteration headers (dim gray via lipgloss when TTY, plain text otherwise)
- In verbose mode, show the prompt content or filename being passed to phase agents
- Headers appear in both RunLoop and runWatchTask code paths
- Tests verify headers are printed for each lifecycle step
