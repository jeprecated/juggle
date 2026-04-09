# Pipeline Design

## Problem

Juggle's current lifecycle model is fixed around `pre`, `before`, `after`, and `post` hooks. That works for simple helper-agent runs, but it does not scale to user-defined workflows with:

- multiple setup or review steps
- mixed agent and shell command execution
- custom ordering
- conditional runs
- parallel branches
- future visual editing

The next model should replace fixed lifecycle hook buckets with a general pipeline scheduler.

## Goal

Introduce a first-class pipeline system for Juggle where users define ordered `agent` and `cmd` nodes with:

- lifecycle event attachment
- dependencies
- conditions
- failure policies

This should become the canonical execution model, with current hook flags treated as compatibility sugar.

## V1 Scope

Ship a constrained but clean first version:

- `juggle pipeline` as a separate subcommand
- inline pipeline syntax using `agent` and `cmd`
- pipeline file support as the canonical persisted format
- fixed event set
- implicit sequential dependency
- explicit dependency overrides
- explicit opt-in parallelism
- one `agent` node at `loop-body`
- basic conditions
- node-level failure policy

Non-goals for v1:

- arbitrary custom events
- multiple `loop-body` nodes
- rich expression language
- cross-node variable passing
- nested workflows
- dynamic node generation
- GUI implementation

## Core Model

A pipeline is a sequence of executable nodes. Each node has:

- execution kind: `agent` or `cmd`
- lifecycle event
- dependency metadata
- optional condition
- failure policy
- execution settings specific to its kind

Internally, execution is driven by a scheduler over a dependency graph, not by hard-coded pre/post branches.

## Events

Initial supported events:

- `run-start`
- `loop-start`
- `loop-body`
- `loop-end`
- `run-end`
- `failure`

Semantics:

- `run-start`: once before iterations begin
- `loop-start`: once at the beginning of each iteration
- `loop-body`: the main iterative work
- `loop-end`: once after the body of each iteration
- `run-end`: once after the loop finishes
- `failure`: when a node failure produces a failure event

V1 invariant:

- exactly one `agent` node must use `event=loop-body`

## Dependencies

Default behavior:

- if `--after` is omitted, a node implicitly depends on the immediately previous node in pipeline order

This default is pipeline-global, not scoped only to the same event.

Explicit behavior:

- `--after <name>` declares one or more dependencies and replaces the implicit previous-node dependency

Parallel behavior:

- `--parallel` disables the implicit previous-node dependency

This makes execution intuitive by default:

- pipelines read top to bottom
- parallelism is explicit

## Conditions

Each node may optionally include a small `when` condition.

V1 condition grammar:

- `iteration==N`
- `iteration!=N`
- `iteration>N`
- `iteration>=N`
- `iteration<N`
- `iteration<=N`
- `success==true`
- `success==false`
- `exit_code==N`

No boolean composition or arbitrary scripting in v1.

## Failure Policy

Each node supports:

- `stop`
- `continue`
- `retry`

Optional:

- `retries=<n>`

Default:

- `on-failure=stop`

`failure` event nodes become eligible when a node failure is surfaced after policy handling.

## CLI Syntax

Pipeline mode uses a custom inline grammar:

```bash
juggle pipeline <node> <node> <node> ...
```

Node forms:

```bash
agent <name> <prompt> [flags...]
cmd <name> <command> [flags...]
```

Parsing rule:

- `agent` or `cmd` starts a new node
- all following tokens belong to that node until the next `agent` or `cmd`

Example:

```bash
juggle pipeline \
  agent "Setup" @setup.md \
    --event run-start \
    --provider claude \
    --model haiku \
  agent "Gather" "inspect the codebase and summarize context" \
    --event run-start \
    --provider claude \
    --model opus \
  agent "Implement" @task.md \
    --event loop-body \
    --provider codex \
    --model gpt-5.4 \
  cmd "Commit" "git add -A && git commit -m 'work done'" \
    --event loop-end \
  cmd "Notify" "notify-send 'run complete'" \
    --event run-end
```

## Shared Node Flags

Supported by both `agent` and `cmd`:

- `--event <event>`
- `--after <name>` repeatable
- `--parallel`
- `--when <expr>`
- `--on-failure <stop|continue|retry>`
- `--retries <n>`
- `--timeout <duration>`
- `--workdir <dir>`

## Agent Flags

Supported only by `agent`:

- `--provider <name>`
- `--model <name>`
- `--plan`
- `--trust`
- `--system-prompt <text|@file>`
- `--allowed-tools <csv>`
- `--disallowed-tools <csv>`
- `--max-turns <n>`
- `--mcp-config <path>`
- `--passthrough <arg>` repeatable

## Cmd Flags

Supported only by `cmd`:

- `--shell <sh|bash|zsh|fish|powershell>`
- `--env KEY=VALUE` repeatable

## Validation Rules

- node names must be unique
- `agent` requires a prompt
- `cmd` requires a command
- kind-specific flags must match the node kind
- `--after` targets must exist
- nodes may not depend on later-declared nodes
- dependency cycles are invalid
- `--parallel` suppresses implicit previous-node dependency
- exactly one `agent` node must use `event=loop-body` in v1
- event names must be from the supported set in v1

## File Format

The file format is the canonical persisted representation.

Example TOML:

```toml
iterations = 10
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
name = "Gather"
prompt = "inspect the codebase and summarize context"
event = "run-start"
model = "opus"

[[agent]]
name = "Implement"
prompt = "@task.md"
event = "loop-body"
provider = "codex"
model = "gpt-5.4"

[[cmd]]
name = "Commit"
command = "git add -A && git commit -m 'work done'"
event = "loop-end"

[[cmd]]
name = "Notify"
command = "notify-send 'run complete'"
event = "run-end"
```

The file format should follow the same semantics as inline CLI:

- omitted `after` implies previous-node dependency unless `parallel = true`

## Canonical Representation

The long-term system should support three equivalent views of the same workflow:

- inline CLI pipeline syntax
- pipeline file
- GUI graph/editor

The canonical source of truth is the pipeline model, not any one interface.

Implications:

- CLI input should parse into the canonical model
- file load/save should read/write the canonical model
- GUI should edit the canonical model
- execution behavior must round-trip losslessly across representations

GUI-only metadata may exist, but it must not affect execution semantics.

Possible future metadata:

- node positions
- grouping/collapsed state
- tags/colors
- visual lanes

This supports a future GUI where users visually arrange nodes, while the underlying artifact remains the same pipeline file used by the CLI.

## Long-Term Goal: GUI

A future GUI should be able to:

- load a pipeline file
- render nodes, dependencies, and event lanes visually
- edit node settings and graph structure
- save back to the same canonical file format
- export a CLI pipeline command where possible
- import a CLI pipeline command into the canonical model and GUI view

This means the pipeline design is not only a CLI feature. It is the foundation for a possible visual workflow editor.

## Round-Trip Goal

The architecture should explicitly support round-tripping between:

- CLI inline pipeline syntax
- canonical pipeline file
- GUI editor representation

Desired future capabilities:

- GUI exports runnable CLI commands
- GUI saves canonical pipeline files
- CLI inline pipelines can be normalized into files
- pipeline files can be rendered into GUI state
- import/export commands may later be added for conversion

Execution semantics must remain stable across these representations.

## Compatibility Mapping

Existing lifecycle hooks can be modeled as pipeline sugar:

- `--agent-pre` -> `agent ... --event run-start`
- `--agent-before` -> `agent ... --event loop-start`
- main prompt -> `agent ... --event loop-body`
- `--agent-after` -> `agent ... --event loop-end`
- `--agent-post` -> `agent ... --event run-end`
- `--cmd-before` -> `cmd ... --event loop-start`
- `--cmd-after` -> `cmd ... --event loop-end`

This allows backward compatibility while unifying execution under the scheduler model.

## Implementation Direction

Recommended implementation layers:

- canonical pipeline types
- inline pipeline parser
- file loader/saver
- validation layer
- scheduler/executor
- compatibility adapters from old hook flags

Likely package split:

- `internal/pipeline/types.go`
- `internal/pipeline/validate.go`
- `internal/pipeline/scheduler.go`
- `internal/pipeline/when.go`
- `internal/pipeline/exec_agent.go`
- `internal/pipeline/exec_cmd.go`
- `internal/cli/pipeline.go`
- `internal/cli/pipeline_file.go`

## Deferred Goals

Explicitly deferred beyond v1:

- multiple `loop-body` nodes
- richer expression language
- arbitrary user-defined events
- cross-node outputs/variables
- reusable pipeline fragments
- fan-out/fan-in helpers
- event-specific concurrency controls
- resumable graph execution
- graph visualization/debugging tooling
- full GUI editor

## Key Design Principle

The pipeline scheduler is the real abstraction. CLI hooks, file definitions, and any future GUI are all frontends over that one execution model.
