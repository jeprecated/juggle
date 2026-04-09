# Pipeline

Juggle's pipeline model replaces fixed lifecycle hook buckets with a general scheduler over a dependency graph. Users define ordered `agent` and `cmd` nodes, each attached to a lifecycle event, with optional dependencies, conditions, and failure policies.

The pipeline file is the canonical persisted format. The inline CLI syntax and any future GUI editor all map onto the same underlying model — same execution semantics across all representations.

For the internal architecture and design rationale, see [docs/pipeline-design.md](pipeline-design.md).

---

## Lifecycle Events

Every node is attached to exactly one lifecycle event:

| Event | When it runs |
|-------|-------------|
| `run-start` | Once, before any iterations begin |
| `loop-start` | At the start of each iteration, before the body |
| `loop-body` | The main iterative work — exactly one `agent` node required |
| `loop-end` | After the body of each iteration |
| `run-end` | Once, after the loop finishes |
| `failure` | When a node failure propagates after policy handling |

---

## Pipeline File

A pipeline file is a TOML document. It is the canonical format — the same file a CLI run produces, a future GUI editor reads, and a user checks into source control.

```toml
iterations = 5
max_parallel_steps = 4

[defaults]
provider = "claude"
model = "sonnet"

[[agent]]
name = "Setup"
prompt = "@setup.md"
event = "run-start"
model = "haiku"

[[agent]]
name = "Implement"
prompt = "@task.md"
event = "loop-body"

[[cmd]]
name = "Commit"
command = "git add -A && git commit -m 'iteration done'"
event = "loop-end"

[[cmd]]
name = "Notify"
command = "notify-send 'run complete'"
event = "run-end"
```

Run it with:

```bash
juggle pipeline --file pipeline.toml
```

---

## Inline CLI Syntax

Pipelines can also be defined inline. Node kinds (`agent` or `cmd`) start a new node; all tokens following belong to it until the next node keyword.

```bash
juggle pipeline \
  agent "Setup" @setup.md \
    --event run-start \
    --model haiku \
  agent "Implement" @task.md \
    --event loop-body \
  cmd "Commit" "git add -A && git commit -m done" \
    --event loop-end \
  cmd "Notify" "notify-send 'run complete'" \
    --event run-end
```

The inline syntax and file format are equivalent representations of the same pipeline model.

---

## Node Kinds

### `agent`

Runs an AI agent session. The prompt is positional (string or `@file` reference).

Agent-specific flags:

| Flag | Description |
|------|-------------|
| `--provider <name>` | AI provider (`claude`, `opencode`, `codex`, `gemini`) |
| `--model <name>` | Model name (e.g. `haiku`, `sonnet`, `opus`) |
| `--plan` | Enable plan mode |
| `--trust` | Enable trust mode |
| `--system-prompt <text\|@file>` | System prompt |
| `--allowed-tools <csv>` | Restrict allowed tools |
| `--disallowed-tools <csv>` | Block specific tools |
| `--max-turns <n>` | Maximum agent turns |
| `--mcp-config <path>` | MCP configuration file |
| `--passthrough <arg>` | Pass extra args to the agent CLI (repeatable) |

### `cmd`

Runs a shell command. The command is positional.

Cmd-specific flags:

| Flag | Description |
|------|-------------|
| `--shell <sh\|bash\|zsh\|fish\|powershell>` | Shell to use |
| `--env KEY=VALUE` | Set environment variable (repeatable) |

---

## Shared Node Flags

All nodes — both `agent` and `cmd` — support these flags:

| Flag | Description |
|------|-------------|
| `--event <event>` | Lifecycle event (see table above) |
| `--after <name>` | Explicit dependency on a named node (repeatable) |
| `--parallel` | Suppress implicit previous-node dependency |
| `--when <expr>` | Condition expression that gates execution |
| `--on-failure <policy>` | `stop` (default), `continue`, or `retry` |
| `--retries <n>` | Retry count when `on-failure=retry` |
| `--timeout <duration>` | Per-node timeout (e.g. `30s`, `5m`) |
| `--workdir <dir>` | Working directory for this node |

---

## Dependencies

By default, each node implicitly depends on the node declared before it in pipeline order. This makes pipelines read top-to-bottom naturally.

**Explicit dependency** — override the implicit previous-node dependency:

```toml
[[agent]]
name = "Review"
prompt = "review the changes"
event = "loop-end"
after = ["Implement"]   # depends on Implement, not whatever came before
```

**Parallel** — remove the implicit dependency entirely so the node can run concurrently with peers:

```toml
[[cmd]]
name = "Lint"
command = "go vet ./..."
event = "loop-end"
parallel = true   # no implicit dependency
```

Rules:
- `--after` targets must be declared before the node that depends on them
- Dependency cycles are invalid
- `--parallel` and `--after` can be combined: `--parallel --after Foo` depends on Foo but not on the previous node

---

## Conditions

A node can be gated by a `--when` condition. If the condition is not met, the node is skipped without error.

In v1, conditions are evaluated as shell commands. A zero exit code means "run this node"; non-zero means "skip."

```toml
[[cmd]]
name = "Deploy"
command = "./deploy.sh"
event = "run-end"
when = "test -f .deploy-ready"
```

Conditions have access to `JUGGLE_*` environment variables, so iteration-based logic works naturally:

```bash
# Only run on the first iteration
cmd "Bootstrap" "./bootstrap.sh" --event loop-start --when "[ $JUGGLE_ITERATION -eq 1 ]"
```

---

## Failure Policies

Each node declares what happens when it fails:

| Policy | Behavior |
|--------|---------|
| `stop` (default) | Stop the run; trigger `failure` event nodes |
| `continue` | Log the failure and continue |
| `retry` | Retry the node up to `--retries` times, then apply `stop` |

```toml
[[cmd]]
name = "Lint"
command = "go vet ./..."
event = "loop-end"
on_failure = "continue"   # lint failures are warnings, not blockers
```

**Failure nodes** — nodes attached to `event = "failure"` run when a stop-policy failure occurs:

```toml
[[cmd]]
name = "Alert"
command = "notify-send 'Pipeline failed'"
event = "failure"
on_failure = "continue"
```

---

## Migrating from Lifecycle Hooks

Juggle's existing lifecycle hook flags (`--agent-pre`, `--cmd-before`, etc.) are internally modeled as pipeline nodes. You do not need to migrate existing runs — they continue to work exactly as before.

When you want more control — multiple setup steps, mixed agent and shell nodes, custom ordering, conditions, or explicit failure policies — the pipeline model exposes the full scheduler directly.

**Hook-to-pipeline mapping:**

| Existing flag | Pipeline equivalent | Notes |
|---------------|--------------------|----|
| `--agent-pre <prompt>` | `agent "..." --event run-start --on-failure stop` | Failure aborts the run |
| `--agent-before <prompt>` | `agent "..." --event loop-start --on-failure stop` | Failure stops; original behavior skipped iteration |
| `--cmd-before <cmd>` | `cmd "..." --event loop-start --on-failure stop` | Failure stops; original behavior skipped iteration |
| *(main prompt)* | `agent "..." --event loop-body` | Exactly one required |
| `--cmd-after <cmd>` | `cmd "..." --event loop-end --on-failure continue` | Failure is a warning |
| `--agent-after <prompt>` | `agent "..." --event loop-end --on-failure continue` | Failure is a warning |
| `--agent-post <prompt>` | `agent "..." --event run-end --on-failure stop` | Failure returns an error |

### Migration example

**Before — hook flags:**

```bash
juggle \
  --agent-pre "read SETUP.md and configure the environment" \
  @implement \
  --cmd-after "go test ./..." \
  --agent-post "write a CHANGELOG entry for today's changes" \
  -n 5
```

**After — pipeline file:**

```toml
iterations = 5

[[agent]]
name = "Setup"
prompt = "read SETUP.md and configure the environment"
event = "run-start"

[[agent]]
name = "Implement"
prompt = "@implement"
event = "loop-body"

[[cmd]]
name = "Test"
command = "go test ./..."
event = "loop-end"
on_failure = "continue"

[[agent]]
name = "Changelog"
prompt = "write a CHANGELOG entry for today's changes"
event = "run-end"
```

The pipeline file makes the execution order explicit, allows per-node failure policies, and can be extended with additional nodes, conditions, or dependencies without restructuring the command line.

---

## Future: GUI and Round-Tripping

The pipeline file is designed to be the single source of truth across multiple frontends. A future GUI editor will:

- Load a pipeline file and render nodes, dependencies, and event lanes visually
- Allow editing node settings and graph structure
- Save back to the same canonical file format
- Export runnable `juggle pipeline` CLI commands
- Import inline CLI pipelines into the canonical model

The GUI is a different frontend over the same execution model — not a separate execution model. A pipeline authored in the GUI runs identically from the CLI. A pipeline authored on the CLI opens in the GUI without conversion.

GUI-only metadata (node positions, colors, labels) may be added to the file format without affecting execution semantics.

This also enables round-tripping:

- CLI inline syntax → canonical pipeline file
- Pipeline file → GUI representation
- GUI edits → pipeline file → CLI command

Execution semantics remain stable across all representations.
