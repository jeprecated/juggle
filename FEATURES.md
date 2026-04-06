# Juggle Hooks and Lifecycle

Juggle provides three independent hook systems that let you control what happens around each agent iteration.

---

## Hook Categories

### 1. Shell Hooks

Shell hooks run ordinary shell commands at fixed points in the loop. They have access to JUGGLE_* environment variables and their output is forwarded to stderr.

| Flag | When it runs | Failure behavior |
|------|--------------|-----------------|
| `--cmd-before <cmd>` | Before each iteration (after `agent-before`) | Non-zero exit skips the iteration; loop continues |
| `--cmd-after <cmd>` | After each iteration (after the agent, before `agent-after`) | Non-zero exit logs a warning; loop continues |
| `--stop-when <cmd>` | After each iteration (after `cmd-after`) | Exit 0 stops the loop gracefully; non-zero continues |

**Examples:**

```bash
# Skip iteration if tests are already passing
juggle --cmd-before "go test ./..." "fix failing tests"

# Notify on completion of each iteration
juggle --cmd-after "curl -s -d 'Iteration done' ntfy.sh/my-topic" "refactor auth"

# Stop once a sentinel file appears
juggle --stop-when "test -f .done" "keep iterating until done"
```

`@file` syntax works: `--cmd-before @check.sh` resolves `check.sh` via `$JUGGLE_PROMPTS` or the current directory.

---

### 2. Agent Phase Hooks

Phase hooks run a full agent session (same provider, same model) at fixed points relative to the loop. They accept prompt text or `@file` references.

| Flag | When it runs | Failure behavior |
|------|--------------|-----------------|
| `--agent-pre <prompt>` | Once before the loop starts | Non-zero exit aborts the run |
| `--agent-before <prompt>` | Before each iteration (before `cmd-before`) | Non-zero exit skips the iteration; loop continues |
| `--agent-after <prompt>` | After each iteration (after `cmd-after`) | Non-zero exit logs a warning; loop continues |
| `--agent-post <prompt>` | Once after the loop ends | Non-zero exit returns an error |

All four flags accept multiple values (comma-separated or repeated `--agent-before`). Multiple values are joined and passed as a single session.

**Examples:**

```bash
# Bootstrap the environment once before the loop
juggle --agent-pre "read SETUP.md and run the setup script" "implement feature X"

# Validate after each iteration
juggle --agent-after "run the test suite and report results" "fix all lint errors"

# Summarise results at the end
juggle --agent-post "write a CHANGELOG entry for today's changes" "refactor payment module"
```

---

### 3. Session Hooks

Session hooks fire inside the agent session itself, before or after specific events. They are passed directly to Claude Code's hook system and have no effect with other providers.

#### Flags

| Flag | Purpose |
|------|---------|
| `--hook EVENT:CMD` | Register a command for a specific Claude Code event (repeatable) |
| `--hooks-file <path>` | Load a full Claude Code hooks settings JSON file as the base |

`--hook` entries are merged on top of `--hooks-file` contents; existing entries in the file are preserved.

#### Supported Events

| Event | When it fires |
|-------|--------------|
| `PreToolUse` | Before the agent executes any tool call |
| `PostToolUse` | After the agent executes any tool call |
| `SessionStart` | At the very start of the agent session, before any turns |
| `Stop` | When the agent session is about to end naturally |
| `SubagentStop` | When a sub-agent spawned by the main agent is about to stop |
| `PreCompact` | Before the context window is compacted |

A `PreToolUse` hook that exits non-zero blocks the tool call. All other hook failures are logged as warnings.

#### Examples

```bash
# Log every tool use
juggle --hook "PreToolUse:echo tool fired >> /tmp/tools.log" "refactor CLI"

# Block writes to production config
juggle --hook "PreToolUse:if echo \$TOOL_INPUT | grep -q prod-config; then exit 1; fi" "update configs"

# Run a notification when the session ends
juggle --hook "Stop:notify-send 'Juggle iteration done'" "write tests"

# Use a full hooks settings file as the base, plus an inline hook
juggle --hooks-file hooks.json --hook "PreToolUse:./validate.sh" "fix bugs"
```

`CMD` also accepts `@file` references: `--hook "PreToolUse:@pre-tool.sh"` loads the script from `$JUGGLE_PROMPTS` or cwd.

---

## Lifecycle Diagram

```
juggle run
│
├── [agent-pre]               # once; failure aborts
│
└── for each iteration:
    │
    ├── [agent-before]        # failure skips iteration
    ├── [cmd-before]          # failure skips iteration
    │
    ├── agent session         ← session hooks active here
    │   ├── SessionStart
    │   ├── PreToolUse / PostToolUse  (per tool call)
    │   ├── SubagentStop      (if subagents are spawned)
    │   ├── PreCompact        (if context compaction occurs)
    │   └── Stop
    │
    ├── [cmd-after]           # failure logs warning, continues
    ├── [agent-after]         # failure logs warning, continues
    └── [stop-when]           # exit 0 stops loop gracefully
│
└── [agent-post]              # once; failure returns error
```

---

## Environment Variables

All shell hooks and agent phase hooks receive these variables. Variables marked "when set" are omitted when the condition does not apply.

| Variable | Set by | Value |
|----------|--------|-------|
| `JUGGLE_RUN_ID` | All hooks | Stable UUID for the entire run |
| `JUGGLE_ITERATION` | All hooks | Current iteration number (1-based; 0 for `pre`/`post` phases) |
| `JUGGLE_MAX_ITERATIONS` | All hooks | Max iterations (`--iterations`; 0 means unlimited) |
| `JUGGLE_MODEL` | All hooks | Model name (e.g. `sonnet`) |
| `JUGGLE_PROVIDER` | All hooks | Provider name (e.g. `claude`) |
| `JUGGLE_LABEL` | All hooks | Value of `--label` (omitted when not set) |
| `JUGGLE_EXIT_CODE` | `cmd-after`, `stop-when` | Agent exit code from this iteration |
| `JUGGLE_INPUT_TOKENS` | `cmd-after`, `stop-when` | Input tokens used this iteration |
| `JUGGLE_OUTPUT_TOKENS` | `cmd-after`, `stop-when` | Output tokens used this iteration |
| `JUGGLE_PHASE` | Agent phase hooks only | `pre`, `before`, `after`, or `post` |
| `JUGGLE_TASK_FILE` | Watch mode only | Path to the task file being processed |
| `JUGGLE_WORKER_ID` | Parallel watch mode only | Worker index (0-based) |

---

## Failure Behavior Summary

| Hook | On failure |
|------|-----------|
| `--agent-pre` | Aborts the entire run |
| `--agent-before` | Skips the current iteration; loop continues |
| `--cmd-before` | Skips the current iteration; loop continues |
| Agent session | Controlled by `--on-failure` (stop / continue / retry) |
| `--cmd-after` | Logs a warning; loop continues |
| `--agent-after` | Logs a warning; loop continues |
| `--stop-when` | Exit 0 stops gracefully; non-zero continues |
| `--agent-post` | Returns an error after the loop |
| Session hook (`PreToolUse`) | Non-zero exit blocks the tool call |
| Session hook (all others) | Failure is logged as a warning |
