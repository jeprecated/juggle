# Juggle

Unopinionated Ralph Loop runner. Runs an AI coding agent in a loop with fresh context per iteration.

## What is a Ralph Loop?

A Ralph Loop runs an AI agent repeatedly, each iteration starting a new session with fresh context. The agent reads its own previous output from files and git history rather than relying on conversation memory. The pattern is named after Ralph Wiggum.

## Why juggle?

- **Unopinionated.** No conventions about what files your agent reads or writes, what task system you use, or how you track progress. Juggle runs the loop. You bring the workflow.
- **Fresh context per iteration.** Each loop starts a new agent session. State lives in git, not LLM memory.
- **Cost-aware.** Built for overnight runs with `--max-cost`, `--max-failures`, and automatic quota backoff.
- **Composable.** Shell hooks (`--cmd-after`, `--stop-when`), phase agents (`--agent-before`, `--agent-after`), and environment variables let you wire in verification gates without juggle imposing a workflow.

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

## Quick start

```bash
# Run 3 iterations on a task
juggle "fix the failing tests" -n 3

# With a prompt file, test hook, and stop condition
juggle @task.md --cmd-after "make test" --stop-when "make test" -n 10

# Watch mode: pick up task files from a directory
juggle --watch tasks/ready/ @worker-rules.md
```

All positional args are prompt content. Strings and `@file` references are joined into the final prompt.

## Features

- **Loop control** -- iterations, delay with jitter, per-iteration timeout, resume after crash
- **Watch mode** -- process task files from a directory or glob pattern, with parallel workers
- **Lifecycle hooks** -- run shell commands before/after each iteration, stop on a condition
- **Phase agents** -- run separate agent sessions before/after the main loop or each iteration
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
