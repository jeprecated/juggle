# Juggle

Unopinionated Ralph Loop runner. Runs an AI coding agent in a loop with fresh context per iteration.

## What is a Ralph Loop?

A Ralph Loop runs an AI agent repeatedly, each iteration starting a new session with fresh context. The agent reads its own previous output from files and git history rather than relying on conversation memory. The pattern is named after Ralph Wiggum.

## Why juggle?

- **Unopinionated.** No conventions about what files your agent reads or writes, what task system you use, or how you track progress. Juggle runs the loop. You bring the workflow.
- **Fresh context per iteration.** Each loop starts a new agent session. State lives in git, not LLM memory.
- **Cost-aware.** Built for overnight runs with `--max-cost`, `--max-failures`, and automatic quota backoff.
- **Composable.** Shell hooks (`--cmd-after`, `--stop-when`), phase agents (`--agent-before`, `--agent-after`), and environment variables let you wire in verification gates without juggle imposing a workflow.

<details>
<summary><strong>Why I made this</strong></summary>

I kept running into the same problem: I wanted to run different kinds of agent loops -- overnight "find a problem and fix it" runs, trigger-based loops that watch a file for updates, and Ralph Loops that work through a list of tasks. None of the existing tools did all of these. My own scripts were too rigid. The Claude Code `/loop` command builds up context instead of starting fresh. And none of them let me reuse prompt templates.

Juggle exists to solve all of this in one tool -- strong enough to trust with your workflow, flexible enough to fit any workflow.

</details>

## Quick start

```bash
# Run 3 iterations on a task
juggle "fix the failing tests" -n 3
```

```bash
# Leave it running overnight -- find and fix issues autonomously
juggle "Find one issue to fix or improvement to make. Check git log to avoid repeating past work." -n 20 --max-cost 10
```

```bash
# Work through a todo list while you sleep
juggle "Read TODO.md and implement the next unchecked item. Mark it done when complete." -n 0 --max-cost 20
```

```bash
# @file reads a file's contents into the prompt
juggle @task.md -n 5
```

```bash
# Mix and match -- all positional args join into one prompt
juggle @tdd @fix "the login form rejects valid emails" -n 5
```

```bash
# Point JUGGLE_PROMPTS at a library of reusable prompts
export JUGGLE_PROMPTS=~/prompts
juggle @tdd @fix "broken email validation" -n 5
```

[prompt-lego](https://github.com/ohare93/prompt-lego) is a starter kit of reusable prompt files that work out of the box.

```bash
# Add a test gate -- stop when tests pass
juggle @tdd @fix "broken email validation" --cmd-after "make test" --stop-when "make test" -n 10
```

```bash
# Run a code review agent after each iteration
juggle @fix "broken email validation" --agent-after @code-review -n 5
```

```bash
# Watch a directory -- agents pick up tasks as they appear
juggle --watch tasks/ready/ @tdd -n 0
```

`--watch` runs a fresh agent session for each file in a directory, and waits for new files to appear. That pairs naturally with a task queue like [frontloop](https://github.com/ohare93/frontloop), which stores tasks as markdown files in `.frontloop/ready/` -- you plan tasks while a background juggle loop works through them. But juggle doesn't care where tasks come from -- a directory of markdown files, a script that dumps issues, or anything else that puts files in a folder.

All positional args are prompt content -- strings and `@file` references join into the final prompt. Point `JUGGLE_PROMPTS` at a directory of reusable prompts and compose them like lego pieces.

## Prompt library

Set `JUGGLE_PROMPTS` to a directory of reusable prompt files. Reference any file with `@name` — no path needed:

```bash
export JUGGLE_PROMPTS=~/prompts
juggle @tdd @fix "broken email validation" -n 5
```

**Resolution order** for a bare `@name` (no `/`):

1. Literal path — `name` read as-is from disk
2. `$JUGGLE_PROMPTS/name`
3. `$JUGGLE_PROMPTS/name.md` (only when `name` has no extension)
4. Alias scan — all files in `$JUGGLE_PROMPTS` checked for a matching `aliases` frontmatter entry
5. Error

Names containing `/` (like `@./local/task.md`) skip steps 2–4 and are treated as literal paths only. Frontmatter is stripped from every file resolved via `$JUGGLE_PROMPTS`.

**Aliases** let a single file answer to multiple names. Add YAML frontmatter:

```markdown
---
aliases: [fix, fixer, bugfix]
---

You are a careful bug-fixing agent...
```

`@fix`, `@fixer`, and `@bugfix` all resolve to that file. Base name (filename without extension) always wins over an alias — `fix.md` is found at step 2 before aliases are checked. Two files declaring the same alias is an error.

**Organizing prompts** — bare-name resolution only searches the top level of `$JUGGLE_PROMPTS`. Subdirectories are for grouping; files in them must be referenced by full path.

```
~/prompts/
├── tdd.md           # @tdd
├── fix.md           # @fix
├── review.md        # @review
└── roles/
    └── senior.md    # @/home/you/prompts/roles/senior.md
```

**Autocomplete** — `juggle completion bash|zsh|fish|nushell|powershell` generates a shell completion script. When enabled, pressing tab after `@` suggests prompt names and aliases from `$JUGGLE_PROMPTS`. Run `juggle completion --help` for setup instructions.

## Install

**macOS (Homebrew):**

```bash
brew tap ohare93/tap && brew install juggle
```

**Windows (Scoop):**

```bash
scoop bucket add ohare93 https://github.com/ohare93/scoop && scoop install juggle
```

**Linux:**

```bash
curl -sSL https://raw.githubusercontent.com/ohare93/juggle/main/install.sh | bash
```

**Go:**

```bash
go install github.com/ohare93/juggle/cmd/juggle@latest
```

## Features

- **Loop control** -- iterations, delay with jitter, per-iteration timeout, resume after crash
- **Watch mode** -- process task files from a directory or glob pattern, with parallel workers (`juggle --watch tasks/ready/ @worker-rules.md`)
- **Lifecycle hooks** -- run shell commands before/after each iteration, stop on a condition
- **Phase agents** -- run separate agent sessions before/after the main loop or each iteration
- **Pipeline** -- define ordered `agent` and `cmd` nodes with events, dependencies, conditions, and failure policies (`juggle pipeline --file pipeline.toml`)
- **Config file** -- `juggle.toml` for project-level defaults
- **JSONL logging** -- per-iteration token counts, cost estimates, and run summary
- **Failure handling** -- stop, continue, or retry with backoff on agent failures

Run `juggle --help` for the full flag reference.

## Providers

Juggle wraps AI coding CLIs. Supported providers:

| Provider | Binary | Flag |
|----------|--------|------|
| Claude Code (default) | `claude` | `--provider claude` |
| OpenCode | `opencode` | `--provider opencode` |
| OpenAI Codex | `codex` | `--provider codex` |
| Gemini CLI | `gemini` | `--provider gemini` |
| Custom | any | `--agent-cmd "your-cli"` |

## Environment variables

Juggle exposes loop state to every spawned agent process. Prompts, skills, and MCP servers can read these.

| Variable | Description |
|----------|-------------|
| `JUGGLE_ITERATION` | Current iteration number (1-indexed) |
| `JUGGLE_MAX_ITERATIONS` | Total planned iterations (`0` = unlimited) |
| `JUGGLE_RUN_ID` | Stable UUID for the entire run |
| `JUGGLE_MODEL` | Model name passed to the agent |
| `JUGGLE_PROVIDER` | Provider name (`claude`, `opencode`, ...) |
| `JUGGLE_LABEL` | Run label if `--label` was set (omitted otherwise) |
| `JUGGLE_TASK_FILE` | *(watch mode)* Absolute path to the current task file |
| `JUGGLE_WORKER_ID` | *(glob watch)* 0-indexed worker number |

## Prerequisites

One of the supported AI coding CLIs installed and authenticated:
[Claude Code](https://claude.ai/code),
[OpenCode](https://opencode.ai/),
[Codex](https://github.com/openai/codex),
[Gemini CLI](https://github.com/google-gemini/gemini-cli),
or any CLI via `--agent-cmd`.

## License

MIT
