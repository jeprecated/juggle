# ADR-001: Consolidate juggle/serve into loop + queue

**Status:** Accepted
**Date:** 2026-04-15

## Context

Juggle has three invocation modes: `juggle [prompts]`, `juggle watch <dir> [prompts]`, and `juggle serve <dir> [prompts]`. These share ~90% of their code and flags but live as separate cobra commands with duplicated wiring. The recent addition of `--every` exposed the problem: "run every 5m AND watch a folder" needed awkward cross-command flag borrowing.

All three modes are really just combinations of two behaviors:
1. **Keep running** — the agent runs immediately, repeats, with optional delay between iterations.
2. **Wait for work** — the agent sits idle until something happens (file appears, interval elapses, HTTP POST arrives, trigger fires).

## Decision

Replace `juggle`, `juggle watch`, and `juggle serve` with two commands:

### `juggle loop [prompts] [flags]`

Runs immediately, keeps running. Pure iteration loop.

- `-n`/`--iterations` — max number of runs
- `--delay` — gap between iterations
- `--id <name>` — allows `juggle trigger <name>` to wake between delays

That's it. No watch, no serve, no every. Loop just runs.

### `juggle queue [prompts] [flags]`

Waits for work. Runs when a trigger fires. With `--now`, also runs immediately before waiting for triggers.

Triggers are composable — any combination works:

- `--watch <path>` — file(s) in dir/glob (repeatable)
- `--on-touch` — trigger on mtime change
- `--every <dur>` — run on fixed interval
- `--now` — also run immediately, then wait for triggers
- `--serve <addr>` — HTTP endpoint that calls WriteTrigger (requires `--id`)
- `--id <name>` — `juggle trigger` as trigger source
- `--workers N`, `--dashboard` — parallel processing and TUI

Queue requires at least one trigger flag (`--watch`, `--every`, `--serve`, or `--id`).

Queue does NOT accept: `--delay`, `-n`/`--iterations`, `--resume`, `--continue`.

Queue stops on: SIGINT, `--stop-when`, `--max-cost`, or `--max-failures`.

### Trigger interaction

When multiple triggers fire concurrently (e.g. a watch file appears while `--every` interval hits), the first trigger detected runs. Others are picked up on subsequent iterations. This is first-come-first-served — same as current behavior.

### Flag ownership

| Flag | `loop` | `queue` |
|------|--------|---------|
| `-n`/`--iterations` | yes | no |
| `--delay` | yes | no |
| `--resume` | yes | no |
| `--continue` | yes | no |
| `--watch` | no | yes |
| `--on-touch` | no | yes |
| `--every` | no | yes |
| `--now` | no | yes |
| `--serve` | no | yes |
| `--workers` | no | yes |
| `--dashboard` | no | yes |
| `--id` | yes | yes |
| All agent/hook/output flags | yes | yes |

`--id` is shared: registered via `registerSharedFlags`. Same meaning on both (a named session targetable by `juggle trigger`).

### Serve design

`--serve` is a pure trigger source. It does NOT write files to disk. Instead:
- HTTP POST body is passed to `WriteTrigger(effectiveID, body)` — the same function `juggle trigger` uses
- Requires `--id` (the trigger mechanism needs a session to target)
- The session's `.d/` directory is where triggers are stored (existing mechanism)
- Returns 202 Accepted
- No temp directories, no file writing, no coupling to `--watch`

### Bare `juggle` behavior

- `juggle` with no args → help text
- `juggle "prompt"` with no subcommand → error with help
- `juggle loop "prompt"` → runs
- `juggle queue "prompt" --watch ./tasks` → queues

### Shared flags

A helper function `registerSharedFlags(cmd *cobra.Command)` registers `--id` plus all agent config, lifecycle hook, and output flags on both commands. No cobra inheritance tricks.

### Config file (juggle.toml)

| Key | Scope | Notes |
|---------|---------|-------|
| `watch` | Queue only | Repeatable |
| `every` | Queue only | Duration string |
| `now` | Queue only | Replaces old `every_immediate` |
| `serve` | Queue only | Address string |
| `on_touch` | Queue only | |
| `workers` | Queue only | |
| `dashboard` | Queue only | |
| `delay` | Loop only | |
| `iterations` | Shared | |
| `model`, `provider`, etc. | Shared | All agent/hook/output flags |

Old keys (`every_immediate`, etc.) are not aliased or warned. No backwards compatibility.

## Migration

No backwards compatibility. All old invocations break:
- `juggle "prompt" -n 5` → `juggle loop "prompt" -n 5`
- `juggle watch ./tasks/ @rules.md` → `juggle queue @rules.md --watch ./tasks/`
- `juggle serve ./tasks/ @rules.md --port 8080` → `juggle queue @rules.md --serve :8080 --id myapp`

## Consequences

- Two modes with clear boundaries: loop runs, queue waits
- No more cross-command flag borrowing
- `--every` and `--watch` combine naturally in queue
- Serve is just another trigger, not a separate command
- `--now` is cleaner than `--every-immediate`
- `-n` removed from queue — stops on signals/guards only
