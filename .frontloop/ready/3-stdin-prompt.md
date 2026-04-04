---
title: Stdin prompt piping
priority: medium
---

## Goal

When stdin is not a TTY, read it as prompt content (appended after positional args). Enables `echo "fix the test" | juggle` and `gh issue view 42 --json body -q .body | juggle` for composing with other CLI tools.

## Acceptance Criteria

- When stdin is not a TTY, read all of stdin and append to prompt content
- Stdin content comes after positional args (joined with \n\n)
- If both stdin and positional args are empty (and no --watch), error as today
- Stdin is read once before the loop starts (not re-read each iteration)
- `--interactive` mode ignores stdin piping (stdin is for the agent TUI)
- Tests verify stdin reading and combination with positional args

## Implementation Notes

- Check with os.Stdin.Stat() for ModeCharDevice (same pattern as TTY detection in format.go)
- Read with io.ReadAll(os.Stdin) before entering RunLoop
