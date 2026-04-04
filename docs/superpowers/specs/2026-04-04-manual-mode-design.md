# Manual Mode & Watch Mode Design

**Date:** 2026-04-04
**Status:** Approved
**Replaces:** Original manual mode spec (same date)

## Summary

Add three new flags to `juggle agent run` that bypass the ball/session system:

- `--manual` (bool) — disables ball-based prompt generation
- `--context` (repeatable string) — injects content into the prompt, accepts raw strings or `@file` references
- `--watch <dir>` — sources task files from a directory, running a sub-loop per file

Together these enable composable autonomous workflows: a triage agent populates a queue, a worker agent processes it, and a human-in-the-loop agent handles clarifications — all coordinated through the filesystem.

Inspired by [gnhf](https://github.com/kunchenguid/gnhf) and real-world experience running autonomous loops against structured task lists.

## CLI Interface

```bash
# Manual mode — bypass balls, provide everything via context
juggle agent run --manual --context "reduce complexity" --context @instructions.md
juggle agent run --manual --context @objective.md --context @queue-conventions.md

# Watch mode — process task files from a directory
juggle agent run --watch queue/ready/ --context @.agents/worker-instructions.md

# With existing flags
juggle agent run --manual --context @task.md --model opus -n 10 --daemon
juggle agent run --watch queue/ready/ --context @.agents/worker.md --daemon --delay 30
```

### Flag rules

- `--manual`, `--watch`, session positional arg, `--ball`, `--pick` are all mutually exclusive
- `--context` requires `--manual` or `--watch` (not valid in ball mode)
- `--context` can be passed multiple times (zero or more)
- `--manual` with zero `--context` values is valid (agent gets just the instructions template)
- `--context` values are either raw strings or `@filepath` references (reads the file)
- `--watch` implies `--manual` behavior (no balls) but is a distinct mode
- All other `agent run` flags work unchanged with both modes

## Manual Mode

When `--manual` is set:

- Ball-based prompt generation is skipped entirely
- The prompt is built from `--context` values + instructions template
- A lightweight session is auto-created for daemon state and progress tracking
- **Session name**: `manual-<6-char-sha256>` derived from the sorted, concatenated context values (deterministic for resume)
- **Resuming**: same context values = same session hash = resume

### Prompt structure (manual mode)

```xml
{{range .Contexts}}
<context>
{{.}}
</context>
{{end}}

<instructions>
You are an autonomous agent.
This is iteration {{.Iteration}}.

## How to work

1. Read the context above to understand what needs to be done and how to operate.
2. Focus on the next smallest logical unit of work that makes incremental progress.
3. If you made code changes, run build/tests/linters if available to validate your work.

## Signaling

When done with this iteration:
- If more work remains: output <promise>CONTINUE</promise>
- If the objective is fully complete: output <promise>COMPLETE</promise>
- If you're stuck and cannot proceed: output <promise>BLOCKED: reason</promise>

You may include a commit message: <promise>CONTINUE: your commit message</promise>
</instructions>
```

## Watch Mode

When `--watch <dir>` is set, juggler runs an outer loop over files in the directory:

1. **Scan** the watched directory for files (alphabetical order — numeric prefixes like `01-`, `02-` control priority)
2. **Pick** the first file
3. **Read** its contents as the task for a sub-loop
4. **Run iterations** until the agent signals COMPLETE or BLOCKED
5. **Rescan** the directory for the next file (files may have appeared or disappeared)
6. **Idle** when the directory is empty — poll at `--delay` interval until a file appears

### Progress in the task file

The agent appends progress directly to the task file during work. Each iteration, juggler re-reads the file to get updated contents (task description + accumulated progress). No separate progress.txt for watch mode.

### File movement is the agent's job

Juggler reads from the watched directory but never moves, deletes, or modifies task files. The `--context` instructions tell the agent conventions like "move the file to `queue/in_progress/` when you start, move to `queue/done/` when complete."

When the agent moves the file out of the watched directory mid-task, juggler doesn't care — it already has the file path from when it picked the file. It keeps iterating on that path until COMPLETE/BLOCKED, then rescans the watched directory.

### Session management

Watch mode creates a single session `watch-<hash-of-dir-path>` for daemon state and output capture. Task files carry their own progress.

### Prompt structure (watch mode)

```xml
<task>
{{.TaskFileContents}}
</task>

{{range .Contexts}}
<context>
{{.}}
</context>
{{end}}

<instructions>
You are an autonomous agent working on the task above.
This is iteration {{.Iteration}}.

## How to work

1. Read the task above to understand what needs to be done.
2. If there is a progress section in the task, read it to understand what was done in previous iterations.
3. Focus on the next smallest logical unit of work that makes incremental progress.
4. If you made code changes, run build/tests/linters if available to validate your work.

## Signaling

When done with this iteration:
- If more work remains: output <promise>CONTINUE</promise>
- If the objective is fully complete: output <promise>COMPLETE</promise>
- If you're stuck and cannot proceed: output <promise>BLOCKED: reason</promise>

You may include a commit message: <promise>CONTINUE: your commit message</promise>
</instructions>
```

## Signal Behavior

Reuses the existing `<promise>` signal system unchanged.

### Manual mode signals

- **CONTINUE** — more work to do, iterate again
- **COMPLETE** — done, stop the loop
- **BLOCKED** — cannot proceed, stop the loop

### Watch mode signals

- **CONTINUE** — more work on this task file, iterate again
- **COMPLETE** — done with this file, pick next from directory
- **BLOCKED** — can't proceed on this file, pick next from directory

Agent decides when to commit by including a message in the signal (existing behavior). Juggler handles VCS commits.

## Multi-Agent Workflow Example

Three long-running juggler processes coordinated through the filesystem:

```
queue/
├── ready/          # Triage outputs, worker inputs
├── clarify/        # Triage outputs, HITL inputs
├── in_progress/    # Worker moves task here while working
└── done/           # Worker outputs completed tasks with progress
```

```bash
# Triage — analyzes task list, creates ready task files, flags unclear ones
juggle agent run --manual \
  --context @.agents/triage-instructions.md \
  --daemon

# Worker — processes task files from ready/
juggle agent run --watch queue/ready/ \
  --context @.agents/worker-instructions.md \
  --daemon

# HITL — human reviews and resolves clarifications
juggle agent run --watch queue/clarify/ \
  --context @.agents/hitl-instructions.md \
  --interactive
```

Juggler knows nothing about the queue structure. The `*-instructions.md` files teach each agent its role, the directory conventions, and how to move files. The agents communicate solely through file creation and movement.

### Task file format (convention, not enforced)

```markdown
# Quote Age and Stale Data Guards

Enforce max-age checks at the trade execution boundary.
Strategies stay pure — callers reject stale inputs.

## Acceptance Criteria
- RunWeatherTrade rejects execution when any quote is older than configurable TTL
- TTL is per-profile configuration (default 5 minutes for quotes, 1 hour for forecasts)
- Rejection produces structured error with which input was stale and by how much

## Design Decisions
- Enforce at execution boundary, not inside strategies
- Per-profile TTL, not global

## Progress
<!-- Agent appends here during work -->
```

## Implementation Changes

### New files

1. **`internal/agent/manual_prompt.md`** — embedded prompt template for manual mode
2. **`internal/agent/watch_prompt.md`** — embedded prompt template for watch mode
3. **`internal/cli/manual.go`** — manual/watch mode logic:
   - `resolveContexts(values []string) ([]string, error)` — handles `@file` prefix or raw string for each value
   - `manualSessionID(contexts []string) string` — deterministic `manual-<6char>` from sha256 of sorted contexts
   - `watchSessionID(dir string) string` — deterministic `watch-<6char>` from sha256 of dir path
   - `setupLightSession(projectDir, sessionID string) error` — creates session dir if needed
   - `generateManualPrompt(contexts []string, iteration int) (string, error)` — renders manual template
   - `generateWatchPrompt(taskContents string, contexts []string, iteration int) (string, error)` — renders watch template
   - `runWatchLoop(config AgentLoopConfig) error` — outer loop: scan dir, pick file, read, run sub-loop, rescan, idle

### Modified files

4. **`internal/agent/prompt.go`** — add embeds for `manual_prompt.md` and `watch_prompt.md`
5. **`internal/cli/agent.go`**:
   - Add `agentManual bool`, `agentWatch string`, `agentContexts []string` flag variables
   - Register flags in `init()`
   - Add mutual exclusivity validation
   - `AgentLoopConfig` gets `Manual bool`, `WatchDir string`, `Contexts []string`
   - In `runAgentRun()`: branch on manual/watch mode before ball-based path
   - In `RunAgentLoop()`: when `Manual` is set, use `generateManualPrompt()` instead of `generateAgentPrompt()`, skip ball pre-loop checks and ball-based model selection

### Unchanged

Provider code, runner, signal parsing, daemon, VCS integration (commits still handled by juggler), TUI, export.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| `--manual` type | Boolean flag | Just disables balls; all content via `--context` |
| `--context` handling | Repeatable, strings or `@file` | Composable — different files for different concerns |
| `--watch` relationship | Peer to `--manual`, not modifier | Distinct mode with its own loop mechanics |
| Queue awareness | None — juggler doesn't know about queues | Queue conventions live in context instructions; keeps juggler simple |
| File movement | Agent's job via instructions | Juggler reads files, doesn't manage them |
| Watch progress | In the task file itself | Self-contained unit of work; no separate progress.txt |
| Watch idle | Poll at `--delay` interval | Reuses existing flag; simple and predictable |
| VCS commits | Juggler handles (existing behavior) | Agent signals commit message, juggler executes |
| Multi-agent coordination | Filesystem as message bus | No inter-process communication needed; agents are independent |
