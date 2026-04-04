---
title: Multi-phase agent sessions (--agent-pre, --agent-before, --agent-after, --agent-post)
priority: high
---

## Goal

Run separate agent sessions at lifecycle points around the main loop. Four slots: agent-pre (once), agent-before (each iteration), agent-after (each iteration), agent-post (once). Each is a full agent invocation with its own prompt, fresh context, and configurable model/trust.

Distinct from shell command hooks (`--cmd-before`/`--cmd-after`) and agent-internal hooks (`--hook`).

## CLI Syntax

```bash
# Single prompt per phase
juggle --agent-after @tidy @task.md

# Multiple prompts for ONE session — comma-separated, joined with \n\n
juggle --agent-after @tidy,@document,@etc @task.md

# All four phases
juggle \
  --agent-pre @analyze,@setup \
  --agent-before @check-clean \
  --agent-after @tidy,@commit-if-clean \
  --agent-post @summarize \
  @task.md -n 10
```

Each flag is a StringSlice (comma-separated). All values are resolved via @file and joined with `\n\n` into a single agent session prompt.

## Acceptance Criteria

- `--agent-pre` — runs a full agent session once before the loop starts
- `--agent-before` — runs a full agent session before each main iteration
- `--agent-after` — runs a full agent session after each main iteration
- `--agent-post` — runs a full agent session once after the loop ends
- Each flag takes comma-separated values; all values merge into ONE session prompt (joined \n\n)
- Each value supports @file resolution (JUGGLE_PROMPTS → cwd)
- Phase agents receive JUGGLE_PHASE env var (pre/before/after/post)
- Phase agents receive JUGGLE_ITERATION, JUGGLE_RUN_ID etc.
- If --agent-after exits non-zero, log warning but continue loop
- If --agent-before exits non-zero, skip the main iteration for that round
- --agent-pre and --agent-post failures are errors that stop the run
- Works in both RunLoop and RunWatch
- Tests cover each phase slot, multi-value merging, and failure handling

## Design Decisions

- Comma-separated values within one flag, merged into one session — no repeated flags needed
- Phase agents use same provider and model as main unless overridden (future: per-phase config via juggle.toml)
- Phase agents run in same working directory as main agent
- Phase output goes to stderr (same as main agent in headless mode)

## Implementation Notes

- Execution order: agent-pre(1x) → [agent-before → main → agent-after] × N → agent-post(1x)
- Cobra StringSliceVar for each flag (native comma-splitting)
- Reuse ResolveArgs() for @file resolution on flag values
- Reuse Runner.Run() for phase agents — just different prompt
- Between-iteration agents should be fast; consider a shorter default timeout

## Completion Summary

- Added `Env []string` field to `RunOptions` in `provider.go`, threaded through claude and opencode providers (`cmd.Env`)
- Added `AgentPre`, `AgentBefore`, `AgentAfter`, `AgentPost string` fields to `Config`
- Created `internal/cli/phase_agent.go` with `BuildPhaseContent()`, `phaseEnv`, and `runPhaseAgent()`
- Wired phase agents into `RunLoop`: pre before loop, before/after each iteration, post after loop
- Wired phase agents into `runWatchTask` with same semantics (pre/post per task file)
- Added `--agent-pre/before/after/post` CLI flags as `StringSliceVar` in `init()`
- Wired flag resolution in `run()` via `BuildPhaseContent()`
- Created `internal/cli/phase_agent_test.go` with 14 tests covering all phases, failure behaviors, env vars, and multi-value merging

### Files Changed

- `internal/agent/provider/provider.go` (modified — Env field in RunOptions)
- `internal/agent/provider/claude.go` (modified — pass Env to cmd)
- `internal/agent/provider/opencode.go` (modified — pass Env to cmd, both headless and interactive)
- `internal/cli/juggle.go` (modified — Config fields, flags, run() wiring, RunLoop phase hooks)
- `internal/cli/watch.go` (modified — runWatchTask phase hooks)
- `internal/cli/phase_agent.go` (new)
- `internal/cli/phase_agent_test.go` (new)
