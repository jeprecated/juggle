# Juggle

Minimal agent loop runner. Takes prompt content, runs an AI agent in a loop, stops.

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

## Usage

All positional args are prompt content (strings or `@file` references):

```bash
juggle @task.md
juggle "fix the tests"
juggle @task.md @instructions.md "use jj not git"
```

### Loop configuration

```bash
juggle @task.md -n 5 --model opus --delay 2
juggle @task.md -n 0                          # Run forever
juggle @task.md --dry-run                     # Show prompt, don't run
```

### Watch mode

Process task files from a directory:

```bash
juggle --watch queue/ready/ @worker-instructions.md
```

### Flags

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--watch` | string | | Watch directory for task files |
| `-n` / `--iterations` | int | 10 | Max iterations (0 = unlimited) |
| `--model` | string | sonnet | Model name |
| `--provider` | string | claude | Provider: claude, opencode |
| `--delay` | int | 0 | Minutes between iterations |
| `--fuzz` | int | 0 | +/- random variance on delay |
| `--trust` | bool | false | Skip permission checks |
| `--interactive` | bool | false | Full agent TUI instead of headless |
| `--timeout` | duration | 0 | Per-iteration timeout |
| `--max-wait` | duration | 0 | Max rate limit wait |
| `--dry-run` | bool | false | Show composed prompt, don't run |
| `--show-thinking` | bool | false | Show model thinking blocks |
| `--label` | string | | Optional label for the run (exposed as `JUGGLE_LABEL`) |

### Environment variables

Juggle sets the following environment variables on every spawned agent process. Prompts, skills, and MCP servers can read these to inspect loop state.

| Variable | Description |
|----------|-------------|
| `JUGGLE_ITERATION` | Current iteration number (1-indexed) |
| `JUGGLE_MAX_ITERATIONS` | Total planned iterations (`0` = unlimited) |
| `JUGGLE_RUN_ID` | Stable UUID for the entire run invocation |
| `JUGGLE_MODEL` | Model name passed to the agent |
| `JUGGLE_PROVIDER` | Provider name (`claude`, `opencode`, …) |
| `JUGGLE_LABEL` | Run label if `--label` was set (omitted otherwise) |
| `JUGGLE_TASK_FILE` | *(watch mode only)* Absolute path to the current task file |
| `JUGGLE_WORKER_ID` | *(worker pool only)* 0-indexed worker number |

## Prerequisites

[Claude Code](https://claude.ai/code) or [OpenCode](https://opencode.ai/) installed and authenticated.

## License

MIT
