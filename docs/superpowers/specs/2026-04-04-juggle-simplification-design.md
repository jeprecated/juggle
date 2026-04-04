# Juggle Simplification Design

**Date:** 2026-04-04
**Status:** Approved

## Summary

Rewrite juggle from scratch as a minimal agent loop runner. Delete the ball/session/TUI system (~35k lines). The new juggle takes prompt content as positional arguments, runs an agent in a loop, and stops. No task storage, no project footprint, no opinions about what's in the prompt.

## CLI Interface

```bash
# All positional args are prompt content (strings or @file references)
juggle @task.md
juggle "fix the tests"
juggle @task.md @instructions.md "use jj not git"

# With loop configuration
juggle @task.md -n 5 --model opus --delay 2

# Watch mode — process task files from a directory, non-stop
juggle --watch queue/ready/ @worker-instructions.md

# Dry run — show the composed prompt, don't run
juggle @task.md --dry-run

# Run forever
juggle @task.md -n 0
```

### Positional arguments

All positional args are prompt content. Each is either:
- A raw string (`"fix the tests"`)
- An `@file` reference (`@task.md`) — file contents are read

All values are joined with `\n\n` into the final prompt. At least one is required.

### Flags

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--watch` | string | | Watch directory for task files |
| `-n` / `--iterations` | int | 10 | Max iterations per run (0 = unlimited) |
| `--model` | string | sonnet | Model: opus, sonnet, haiku |
| `--provider` | string | claude | Provider: claude, opencode |
| `--delay` | int | 0 | Minutes between iterations |
| `--fuzz` | int | 0 | +/- random variance on delay (minutes) |
| `--trust` | bool | false | Skip agent permission checks |
| `--interactive` | bool | false | Full agent TUI instead of headless |
| `--timeout` | duration | 0 | Per-iteration timeout (0 = none) |
| `--max-wait` | duration | 0 | Max rate limit wait (0 = unlimited) |
| `--dry-run` | bool | false | Show composed prompt, don't run |
| `--show-thinking` | bool | false | Show model thinking blocks |

## The Loop

Each iteration:

1. **Build prompt** — join all positional args (resolved `@file` contents or raw strings) with `\n\n`. Append a minimal instructions footer with the iteration number.
2. **Run agent** — send prompt to provider, headless or interactive.
3. **Wait** — delay + fuzz if configured.
4. **Repeat** until max iterations reached.

No signal parsing. No VCS commits. No early exit detection. Just a loop.

### Rate limiting

On 429 (rate limit) or 529 (overload), retry with exponential backoff up to `--max-wait`. If exceeded, exit with error.

### Prompt template

The prompt wraps user content with minimal iteration metadata:

```
{joined positional arg content}

---
This is iteration {N} of {max}.
```

No signal instructions, no conventions. The user's prompt content controls what the agent does.

## Watch Mode

When `--watch <dir>` is set, juggle runs an outer loop over files in the directory:

1. **Scan** the watched directory for files (alphabetical order, numeric prefixes control priority).
2. **Pick** the first regular file (skip hidden files starting with `.`).
3. **Read** its contents, prepend as a `<task>` section to the prompt.
4. **Run** N iterations on that file.
5. **Rescan** the directory for the next file. If the current file was moved/deleted by the agent, it's already gone.
6. **Idle** when empty — poll at `--delay` interval (minimum 30 seconds if delay is 0).

### Watch mode prompt

```
<task>
{task file contents}
</task>

{joined positional arg content}

---
This is iteration {N} of {max}, processing {filename}.
```

The task file is re-read each iteration to pick up any progress the agent appended.

## What Gets Deleted

**Entire directories:**
- `internal/tui/` (13,107 lines) — all TUI code
- `internal/session/` (4,724 lines) — balls, sessions, store, config, discovery, archive, locks, metrics
- `internal/agent/daemon/` (~400 lines) — daemon system

**Files from `internal/agent/`:**
- `prompt.md` — ball-based agent prompt template
- `refine.go` + `refine_prompt.md` — refine command

**Files from `internal/cli/`:**
- All existing files (~16,900 lines). The new CLI is written from scratch.

**Integration tests:**
- All existing tests in `internal/integration_test/` — rewritten for the new interface.

**Documentation:**
- All existing arch docs in `docs/` that reference balls, sessions, TUI.

## What Survives

| Component | Lines | Action |
|-----------|-------|--------|
| `cmd/juggle/main.go` | ~20 | Rewrite — minimal entry point |
| `internal/agent/provider/` | ~2,200 | Keep as-is — claude/opencode abstraction |
| `internal/agent/runner.go` | ~150 | Keep — runner interface, MockRunner for tests |
| `internal/agent/prompt.go` | ~30 | Simplify — single template embed |
| `internal/vcs/` | ~590 | Keep — available for future hooks |
| New `internal/cli/juggle.go` | ~300 est. | New — single file, all flags, loop logic |
| New `internal/cli/watch.go` | ~100 est. | New — watch directory scanning |
| New `internal/cli/resolve.go` | ~30 est. | New — @file resolution |
| New prompt template | ~10 | New — minimal iteration footer |
| New watch prompt template | ~15 | New — task section + footer |

**Estimated total:** ~3,500 lines, down from ~40,000 (91% reduction).

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Positional args as prompt | All args are content | No flags needed for the primary input — simplest UX |
| No signal parsing | Removed | Juggle doesn't parse agent output. Future hook system can add this. |
| No VCS commits | Removed | Agent or future hooks handle commits. Juggle is just a loop. |
| No daemon mode | Removed | Use `&` or tmux. Can add back later. |
| No project footprint | No `.juggle/` directory | Juggle is stateless. No init, no config files. |
| No progress validation | Removed | Trust the agent. Simpler. |
| No global config | Removed | All configuration via flags per invocation. |
| Watch idle polling | 30s minimum, respects --delay | Prevents busy-wait on empty directories. |
| Rate limit retry | Exponential backoff | Reuse existing provider retry logic. |
| Default model | sonnet | Good balance of speed/capability for autonomous loops. |

## Implementation Approach

This is a **rewrite, not a refactor**. The new codebase is small enough to write from scratch while cherry-picking the provider system and VCS integration from the existing code.

The approach:
1. Create the new CLI structure (single command, flags, positional args)
2. Wire in the existing provider system
3. Implement the loop
4. Implement watch mode
5. Write new integration tests
6. Delete everything else
