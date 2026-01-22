# Task Hierarchy Design

Date: 2026-01-22

## Overview

This document describes a major refactoring of the juggler data model to:

1. **Rename terminology** - Ball → Task, Session → removed (unified model)
2. **Add hierarchy** - Tasks can contain sub-tasks via embedded Steps
3. **Introduce Run templates** - Configurable orchestration patterns with sub-agent support

## Core Data Model

### Task (replaces Ball)

Everything is a Task. A Task with children is implicitly an "Epic."

```go
type Task struct {
    ID                 string
    Title              string
    Context            string           // Background/description
    Steps              []Step           // Mixed: instructions + sub-tasks
    AcceptanceCriteria []string
    Tags               []string         // Cross-cutting groupings
    DependsOn          []string         // Hard blocks on other task IDs
    Priority           Priority

    // Execution state (stays on Task for simplicity)
    State              TaskState        // pending/active/blocked/completed
    BlockedReason      string
    Output             string
    CompletionNote     string

    // Timestamps & VCS
    StartedAt          time.Time
    LastActivity       time.Time
    CompletedAt        *time.Time
    StartingRevision   string
    RevisionID         string

    // Execution hints
    ModelSize          ModelSize
    AgentProvider      string
    ModelOverride      string
}
```

### Step (mixed type)

Steps can be instructions (strings) or embedded sub-tasks.

```go
type StepType string

const (
    StepTypeInstruction StepType = "instruction"
    StepTypeTask        StepType = "task"
)

type ExecMode string

const (
    ExecSequential ExecMode = "sequential"  // Wait for completion before next
    ExecParallel   ExecMode = "parallel"    // Spawn and continue
)

type Step struct {
    Type        StepType  // "instruction" | "task"
    Instruction string    // If Type=instruction
    Task        *Task     // If Type=task (embedded)
    Execution   ExecMode  // "sequential" | "parallel" (default: sequential)
}
```

**Implicit barriers:** When a sequential step follows parallel steps, execution automatically waits for all parallel work to complete. No explicit barrier syntax needed.

### Relationships

| Field | Purpose |
|-------|---------|
| Embedded in `Steps` | Hierarchy - parent owns children |
| `Order` | Implicit in Steps array position |
| `DependsOn` | Cross-cutting blocks across any tasks/epics |
| `Tags` | Cross-cutting groupings for filtering |

## Run & Templates

### Run (execution of a task tree)

```go
type Run struct {
    ID            string
    TargetTaskID  string    // Root task, or empty for tag-based
    TargetTag     string    // If tag-based run
    Template      string    // Template name to use

    State         RunState  // pending/running/paused/completed/failed
    CurrentPhase  string    // Which template phase we're in
    CurrentTaskID string    // Which task is being worked on
    Iteration     int       // Current iteration number

    StartedAt     time.Time
    LastActivity  time.Time
    CompletedAt   *time.Time
}
```

### Progress Tracking

```go
type ProgressType string

const (
    ProgressPhaseStart      ProgressType = "phase_start"
    ProgressPhaseEnd        ProgressType = "phase_end"
    ProgressIterationStart  ProgressType = "iteration_start"
    ProgressIterationEnd    ProgressType = "iteration_end"
    ProgressAgentOutput     ProgressType = "agent_output"
    ProgressTaskComplete    ProgressType = "task_complete"
    ProgressSubagentSpawn   ProgressType = "subagent_spawn"
    ProgressSubagentComplete ProgressType = "subagent_complete"
    ProgressError           ProgressType = "error"
    ProgressUserAction      ProgressType = "user_action"
)

type ProgressEntry struct {
    Timestamp   time.Time
    Iteration   int
    Phase       string           // Which template phase
    TaskID      string           // Which task (if applicable)
    Type        ProgressType
    Message     string
    Metadata    map[string]any   // Extra context (revision, duration, etc.)
}
```

### Template Definition

Templates are YAML files defining orchestration patterns.

```yaml
name: iterative
description: Classic Ralph loop - assess, execute children sequentially, validate

phases:
  - name: plan
    type: prompt
    prompt: |
      You are working on: {{.Task.Title}}

      Context: {{.Task.Context}}

      Steps to complete:
      {{range .Task.Steps}}
      - {{.}}
      {{end}}

      Assess the order and reorder if needed.

  - name: execute
    type: children
    mode: sequential-loops

  - name: validate
    type: prompt
    prompt: |
      Work completed. Progress so far:
      {{.Progress}}

      Child task results:
      {{range .CompletedChildren}}
      - {{.Title}}: {{.Output}}
        Revision: {{.RevisionID}}
      {{end}}

      Acceptance criteria to verify:
      {{range .Task.AcceptanceCriteria}}
      - {{.}}
      {{end}}

      Review the work. Add tasks if incomplete, or mark done.
```

### Template Phase Types

| Type | Behavior |
|------|----------|
| `prompt` | Agent executes the prompt, logs progress |
| `children` | Execute child tasks per `mode` |

### Children Execution Modes

| Mode | Behavior |
|------|----------|
| `sequential-loops` | Each child gets its own full agent loop (classic Ralph) |
| `parallel-agents` | Spawn sub-agents for each child simultaneously |
| `inline` | Agent handles children within current context (no sub-agents) |

### Template Variables

| Variable | Contents |
|----------|----------|
| `{{.Task}}` | Current task (Title, Context, Steps, ACs, etc.) |
| `{{.Task.Steps}}` | Steps array |
| `{{.Task.AcceptanceCriteria}}` | ACs array |
| `{{.Progress}}` | Progress log so far |
| `{{.Run}}` | Run metadata (iteration, phase, etc.) |
| `{{.CompletedChildren}}` | Finished child tasks with Output, RevisionID |
| `{{.PendingChildren}}` | Remaining child tasks |
| `{{.ParentTask}}` | Parent task if this is a sub-task |

Future extensibility (not built initially):
- `{{.Children.RevisionIDs}}` - All child revision IDs for merge commits
- `{{.Worktrees}}` - Active worktree paths
- `{{.PreviousPhase.Output}}` - Output from prior phase

### Built-in Templates

| Template | Behavior |
|----------|----------|
| `single` | Execute one task directly, no orchestration |
| `iterative` | Classic Ralph loop - assess, execute children one-by-one, validate |
| `parallel` | Sub-agent orchestration - spawn parallel agents |
| `plan-only` | Assess and restructure, don't execute |

### Default Template Inference

| Task shape | Default template |
|------------|------------------|
| Leaf task (no children) | `single` |
| Has children | `iterative` |
| `--parallel` flag | `parallel` |

## Storage Structure

```
.juggle/
  tasks.jsonl              # All tasks (replaces balls.jsonl)
  templates/               # User-defined templates
    iterative.yaml
    parallel.yaml
    custom-review.yaml
  runs/
    <task-id>/             # Tree-based run
      run.json
      progress.jsonl
      history.jsonl
      lock
    tag:<tagname>/         # Tag-based run
      run.json
      progress.jsonl
      history.jsonl
      lock
  config.yaml              # Project config (unchanged)
```

### Template Resolution Order

1. `.juggle/templates/<name>.yaml` (project-specific)
2. `~/.config/juggle/templates/<name>.yaml` (user global)
3. Built-in defaults

## CLI Commands

### Renamed Commands

| Old | New | Notes |
|-----|-----|-------|
| `juggle add` | `juggle add` | Creates a Task (unchanged verb) |
| `juggle list` | `juggle list` | Lists Tasks |
| `juggle show <ball-id>` | `juggle show <task-id>` | Shows Task details |
| `juggle session create` | `juggle add --epic` | Creates Task intended as parent |
| `juggle session list` | `juggle list --epics` | Lists Tasks that have children |
| `juggle agent run <session>` | `juggle run <task-id>` | Runs a task tree |
| `juggle agent run --tag <tag>` | `juggle run --tag <tag>` | Runs all tasks with tag |

### New Commands

| Command | Purpose |
|---------|---------|
| `juggle run <task-id> --template <name>` | Run with specific template |
| `juggle run --list` | Show active runs |
| `juggle run --stop <run-id>` | Stop a run |
| `juggle templates list` | List available templates |
| `juggle templates show <name>` | Show template definition |
| `juggle templates create <name>` | Create new template (interactive) |

### Task Creation CLI

```bash
# Simple task
juggle add "Fix login bug" \
  --context "Users reporting login failures" \
  --step "Review auth code" \
  --step "Implement fix" \
  --ac "Login works with valid credentials" \
  --ac "Invalid credentials show error"

# With sub-tasks (parallel execution)
juggle add "Build auth system" \
  --context "We need user auth for the API" \
  --step "Review existing code" \
  --step '{"type":"task","title":"Implement login","execution":"parallel"}' \
  --step '{"type":"task","title":"Implement logout","execution":"parallel"}' \
  --step "Write integration tests" \
  --ac "All auth flows work end-to-end"
```

## TUI

### Task Creation Order

1. Context (set the scene)
2. Title (name it)
3. Steps (define work, including sub-tasks)
4. Acceptance Criteria (define done)

### Steps Section Features

- Add instruction (text input)
- Add sub-task (opens nested form)
- Mark step as parallel/sequential
- Reorder steps via drag or keyboard

## Migration

### Automated Migration (`juggle migrate`)

1. **Rename file:** `balls.jsonl` → `tasks.jsonl`

2. **Transform each Ball → Task:**
   - Field renames (mostly 1:1)
   - `Steps` stays as `[]string` (instructions only, no embedded tasks yet)
   - `Tags` preserved
   - `DependsOn` preserved

3. **Convert Sessions → Tasks:**
   - Each Session becomes a Task
   - Session's Context, ACs, DefaultModel transfer to Task
   - All Tasks with that Session's tag become children (embedded in Steps)

4. **Migrate run data:**
   - `sessions/<id>/` → `runs/tag:<id>/`
   - Progress, history files move as-is

5. **Backward compatibility:**
   - Keep `balls.jsonl` as read-only fallback for one version
   - Warn if old format detected

## Package Changes

| Old | New |
|-----|-----|
| `internal/session/` | `internal/core/` |
| `internal/session/ball.go` | `internal/core/task.go` |
| `internal/session/juggle_session.go` | Merged into `internal/core/task.go` |
| (new) | `internal/template/` |
| (new) | `internal/migrate/` |

## TUI Changes

### Main View Layout

**Current:** Sessions list on left, balls on right

**New:** Epics list on left (with tag toggle), tasks on right

| Element | Change |
|---------|--------|
| Left panel header | "Sessions" → "Epics" (or "Tags" when toggled) |
| Left panel content | Sessions → Tasks with children (Epics) |
| Toggle keybind | New: switch between Epic view and Tag view |
| Right panel | Balls → Tasks (children of selected Epic or tagged tasks) |

### Task Creation/Edit Form

**Field order:** Context → Title → Steps → ACs

**Steps section enhancements:**
- Each step shows type indicator (instruction vs sub-task)
- Keybind to convert instruction → sub-task (opens nested form)
- Keybind to expand/edit sub-task inline
- Sub-tasks can be collapsed/expanded
- Visual nesting indication

**Nested form behavior:**
- Creating sub-task opens nested form overlay
- Nested form has same fields: Context → Title → Steps → ACs
- Can nest arbitrarily deep (though UI may limit display)
- Save commits entire tree atomically
- Cancel discards all unsaved changes in tree

### Keybinds (proposed)

| Key | Action |
|-----|--------|
| `Tab` | Toggle left panel: Epics ↔ Tags |
| `t` | Convert step to sub-task (in Steps section) |
| `Enter` | Expand/edit sub-task (in Steps section) |
| `Esc` | Close nested form, return to parent |

### Test Strategy

TUI tests will break due to terminology and structure changes. Approach:

1. Make all code changes first
2. Run tests to identify failures
3. Update test expectation files in bulk (`go test -update` or similar)
4. Review updated expectations for correctness

## Implementation Order

1. **Phase 1: Core model**
   - Create `internal/core/` with Task, Step, Run types
   - Create `internal/template/` with template loading and execution

2. **Phase 2: Storage**
   - Update store to read/write tasks.jsonl
   - Implement run storage in `runs/<target>/`

3. **Phase 3: Migration**
   - Implement `juggle migrate` command
   - Add backward compatibility fallback

4. **Phase 4: CLI**
   - Update `juggle add` with --step flag and JSON sub-task support
   - Rename `juggle agent run` → `juggle run`
   - Add template commands

5. **Phase 5: TUI**
   - Update task form with new field order
   - Add sub-task creation in Steps section

6. **Phase 6: Run engine**
   - Update daemon to use templates
   - Implement parallel sub-agent spawning
