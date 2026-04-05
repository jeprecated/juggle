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

## Completion Summary

- `StartKeypressListener` reads from any `io.Reader` one byte at a time; 'q'/'Q' triggers shutdown and prints stop message (red if color enabled)
- `openTTYKeypress` opens `/dev/tty` in raw mode (Linux) so keystrokes are intercepted independently of the agent's stdin pipe
- Listener is wired in `runRootCmd` alongside signal handling; same `shutdownOnce`/`shutdown` channel used by Ctrl+C
- Silently skipped when stdin is not a TTY (piped/non-interactive runs)
- Terminal state fully restored on cleanup

### Files Changed

- internal/cli/keypress.go (new) — core listener logic
- internal/cli/keypress_linux.go (new) — TTY open + raw mode via syscall
- internal/cli/keypress_other.go (new) — non-Linux stub
- internal/cli/keypress_test.go (new) — 7 unit tests
- internal/cli/color.go (modified) — added ansiRed constant
- internal/cli/juggle.go (modified) — wire up keypress listener in runRootCmd
