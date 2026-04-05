---
title: Keypress to stop after current iteration
priority: medium
---

## Goal

Allow the user to press a key (e.g. `q`) while the loop is running to gracefully stop after the current iteration completes, printing "Stopping after this iteration completes" in red.

## Acceptance Criteria

- Pressing `q` during a running loop sets the shutdown flag (same as first Ctrl+C)
- "Stopping after this iteration completes" printed in red immediately on keypress
- Current iteration runs to completion, then loop exits cleanly
- Only active when stdin is a TTY (non-interactive/piped runs skip this)
- Works in both RunLoop and RunWatch modes
- Does not interfere with agent's own stdin usage
