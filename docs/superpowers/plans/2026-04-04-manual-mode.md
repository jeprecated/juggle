# Manual Mode & Watch Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--manual`, `--context`, and `--watch` flags to `juggle agent run` that bypass the ball system and enable composable autonomous workflows.

**Architecture:** `--manual` (bool) disables ball-based prompting. `--context` (repeatable string/`@file`) injects content into the prompt. `--watch <dir>` processes task files from a directory in a sub-loop. All three compose with existing flags (daemon, model, iterations, etc.). New code lives in `internal/cli/manual.go` and `internal/agent/` prompt templates.

**Tech Stack:** Go, cobra flags, Go `text/template`, `embed`, `crypto/sha256`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/agent/manual_prompt.md` | Embedded prompt template for manual mode (contexts + instructions) |
| `internal/agent/watch_prompt.md` | Embedded prompt template for watch mode (task + contexts + instructions) |
| `internal/agent/prompt.go` | Add embeds for the two new templates |
| `internal/cli/manual.go` | All manual/watch mode logic: context resolution, session IDs, prompt generation, watch loop |
| `internal/cli/manual_test.go` | Unit tests for manual.go functions |
| `internal/cli/agent.go` | Add flags, validation, config fields, branching in `runAgentRun()` and `RunAgentLoop()` |
| `internal/integration_test/manual_mode_test.go` | Integration tests for manual mode agent loop |
| `internal/integration_test/watch_mode_test.go` | Integration tests for watch mode agent loop |

---

### Task 1: Prompt Templates and Embeds

**Files:**
- Create: `internal/agent/manual_prompt.md`
- Create: `internal/agent/watch_prompt.md`
- Modify: `internal/agent/prompt.go`

- [ ] **Step 1: Create the manual mode prompt template**

Create `internal/agent/manual_prompt.md`:

```markdown
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

- [ ] **Step 2: Create the watch mode prompt template**

Create `internal/agent/watch_prompt.md`:

```markdown
<task>
{{.TaskContents}}
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

- [ ] **Step 3: Add embeds to prompt.go**

In `internal/agent/prompt.go`, add the new embeds after the existing `PromptTemplate`:

```go
//go:embed prompt.md
var PromptTemplate string

//go:embed manual_prompt.md
var ManualPromptTemplate string

//go:embed watch_prompt.md
var WatchPromptTemplate string

// GetPromptTemplate returns the embedded agent prompt template.
func GetPromptTemplate() string {
	return PromptTemplate
}

// GetManualPromptTemplate returns the embedded manual mode prompt template.
func GetManualPromptTemplate() string {
	return ManualPromptTemplate
}

// GetWatchPromptTemplate returns the embedded watch mode prompt template.
func GetWatchPromptTemplate() string {
	return WatchPromptTemplate
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./internal/agent/...`
Expected: success (no errors)

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat: add manual and watch mode prompt templates"
```

---

### Task 2: Manual Mode Logic (`manual.go`) — Core Functions

**Files:**
- Create: `internal/cli/manual.go`
- Create: `internal/cli/manual_test.go`

- [ ] **Step 1: Write tests for `resolveContexts`**

Create `internal/cli/manual_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveContexts_InlineString(t *testing.T) {
	result, err := resolveContexts([]string{"do the thing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "do the thing" {
		t.Errorf("expected [\"do the thing\"], got %v", result)
	}
}

func TestResolveContexts_FileReference(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ctx.md")
	os.WriteFile(f, []byte("file contents here"), 0644)

	result, err := resolveContexts([]string{"@" + f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "file contents here" {
		t.Errorf("expected [\"file contents here\"], got %v", result)
	}
}

func TestResolveContexts_FileNotFound(t *testing.T) {
	_, err := resolveContexts([]string{"@/nonexistent/file.md"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestResolveContexts_Mixed(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ctx.md")
	os.WriteFile(f, []byte("from file"), 0644)

	result, err := resolveContexts([]string{"inline string", "@" + f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0] != "inline string" {
		t.Errorf("expected first = \"inline string\", got %q", result[0])
	}
	if result[1] != "from file" {
		t.Errorf("expected second = \"from file\", got %q", result[1])
	}
}

func TestResolveContexts_Empty(t *testing.T) {
	result, err := resolveContexts([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestResolveContexts -v`
Expected: FAIL — `resolveContexts` not defined

- [ ] **Step 3: Implement `resolveContexts`**

Create `internal/cli/manual.go`:

```go
package cli

import (
	"fmt"
	"os"
	"strings"
)

// resolveContexts processes --context flag values.
// Values starting with @ are treated as file paths (contents are read).
// All other values are used as-is.
func resolveContexts(values []string) ([]string, error) {
	resolved := make([]string, 0, len(values))
	for _, v := range values {
		if strings.HasPrefix(v, "@") {
			path := v[1:]
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read context file %s: %w", path, err)
			}
			resolved = append(resolved, string(data))
		} else {
			resolved = append(resolved, v)
		}
	}
	return resolved, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestResolveContexts -v`
Expected: PASS (all 5 tests)

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat: add resolveContexts for manual mode"
```

---

### Task 3: Session ID Generation and Prompt Generation

**Files:**
- Modify: `internal/cli/manual.go`
- Modify: `internal/cli/manual_test.go`

- [ ] **Step 1: Write tests for `manualSessionID` and `watchSessionID`**

Append to `internal/cli/manual_test.go`:

```go
func TestManualSessionID_Deterministic(t *testing.T) {
	id1 := manualSessionID([]string{"context a", "context b"})
	id2 := manualSessionID([]string{"context a", "context b"})
	if id1 != id2 {
		t.Errorf("expected deterministic IDs, got %q and %q", id1, id2)
	}
}

func TestManualSessionID_DifferentInputs(t *testing.T) {
	id1 := manualSessionID([]string{"context a"})
	id2 := manualSessionID([]string{"context b"})
	if id1 == id2 {
		t.Error("expected different IDs for different inputs")
	}
}

func TestManualSessionID_Prefix(t *testing.T) {
	id := manualSessionID([]string{"anything"})
	if !strings.HasPrefix(id, "manual-") {
		t.Errorf("expected prefix \"manual-\", got %q", id)
	}
	// Should be "manual-" + 6 hex chars = 13 chars total
	if len(id) != 13 {
		t.Errorf("expected length 13, got %d (%q)", len(id), id)
	}
}

func TestManualSessionID_OrderIndependent(t *testing.T) {
	id1 := manualSessionID([]string{"a", "b"})
	id2 := manualSessionID([]string{"b", "a"})
	if id1 != id2 {
		t.Errorf("expected order-independent IDs, got %q and %q", id1, id2)
	}
}

func TestManualSessionID_EmptyContexts(t *testing.T) {
	id := manualSessionID([]string{})
	if !strings.HasPrefix(id, "manual-") {
		t.Errorf("expected prefix \"manual-\", got %q", id)
	}
}

func TestWatchSessionID(t *testing.T) {
	id := watchSessionID("/some/path/queue/ready")
	if !strings.HasPrefix(id, "watch-") {
		t.Errorf("expected prefix \"watch-\", got %q", id)
	}
	if len(id) != 12 {
		t.Errorf("expected length 12, got %d (%q)", len(id), id)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run "TestManualSessionID|TestWatchSessionID" -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement session ID functions**

Add to `internal/cli/manual.go` (add `"crypto/sha256"`, `"sort"` to imports):

```go
import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
)

// manualSessionID generates a deterministic session ID from context values.
// Contexts are sorted before hashing so order doesn't matter.
func manualSessionID(contexts []string) string {
	sorted := make([]string, len(contexts))
	copy(sorted, contexts)
	sort.Strings(sorted)
	h := sha256.Sum256([]byte(strings.Join(sorted, "\x00")))
	return fmt.Sprintf("manual-%x", h[:3])
}

// watchSessionID generates a deterministic session ID from a directory path.
func watchSessionID(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("watch-%x", h[:3])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run "TestManualSessionID|TestWatchSessionID" -v`
Expected: PASS (all 6 tests)

- [ ] **Step 5: Write tests for prompt generation**

Append to `internal/cli/manual_test.go`:

```go
func TestGenerateManualPrompt_Basic(t *testing.T) {
	prompt, err := generateManualPrompt([]string{"do the thing"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "do the thing") {
		t.Error("expected prompt to contain context")
	}
	if !strings.Contains(prompt, "<context>") {
		t.Error("expected prompt to contain <context> tag")
	}
	if !strings.Contains(prompt, "iteration 1") {
		t.Error("expected prompt to contain iteration number")
	}
	if !strings.Contains(prompt, "<promise>") {
		t.Error("expected prompt to contain promise signal instructions")
	}
}

func TestGenerateManualPrompt_MultipleContexts(t *testing.T) {
	prompt, err := generateManualPrompt([]string{"objective", "instructions"}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "objective") {
		t.Error("expected prompt to contain first context")
	}
	if !strings.Contains(prompt, "instructions") {
		t.Error("expected prompt to contain second context")
	}
	if !strings.Contains(prompt, "iteration 3") {
		t.Error("expected prompt to contain iteration 3")
	}
}

func TestGenerateManualPrompt_NoContexts(t *testing.T) {
	prompt, err := generateManualPrompt([]string{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "<instructions>") {
		t.Error("expected prompt to contain instructions even with no contexts")
	}
	if strings.Contains(prompt, "<context>") {
		t.Error("expected no <context> tags with empty contexts")
	}
}

func TestGenerateWatchPrompt_Basic(t *testing.T) {
	prompt, err := generateWatchPrompt("task file contents", []string{"worker instructions"}, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "task file contents") {
		t.Error("expected prompt to contain task contents")
	}
	if !strings.Contains(prompt, "<task>") {
		t.Error("expected prompt to contain <task> tag")
	}
	if !strings.Contains(prompt, "worker instructions") {
		t.Error("expected prompt to contain context")
	}
	if !strings.Contains(prompt, "iteration 2") {
		t.Error("expected prompt to contain iteration 2")
	}
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run "TestGenerateManualPrompt|TestGenerateWatchPrompt" -v`
Expected: FAIL — functions not defined

- [ ] **Step 7: Implement prompt generation functions**

Add to `internal/cli/manual.go` (add `"bytes"`, `"text/template"`, and agent import):

```go
import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/ohare93/juggle/internal/agent"
)

// manualPromptData holds template data for manual mode prompts.
type manualPromptData struct {
	Contexts  []string
	Iteration int
}

// watchPromptData holds template data for watch mode prompts.
type watchPromptData struct {
	TaskContents string
	Contexts     []string
	Iteration    int
}

// generateManualPrompt renders the manual mode prompt template.
func generateManualPrompt(contexts []string, iteration int) (string, error) {
	tmpl, err := template.New("manual").Parse(agent.GetManualPromptTemplate())
	if err != nil {
		return "", fmt.Errorf("failed to parse manual prompt template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, manualPromptData{
		Contexts:  contexts,
		Iteration: iteration,
	})
	if err != nil {
		return "", fmt.Errorf("failed to render manual prompt: %w", err)
	}

	return buf.String(), nil
}

// generateWatchPrompt renders the watch mode prompt template.
func generateWatchPrompt(taskContents string, contexts []string, iteration int) (string, error) {
	tmpl, err := template.New("watch").Parse(agent.GetWatchPromptTemplate())
	if err != nil {
		return "", fmt.Errorf("failed to parse watch prompt template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, watchPromptData{
		TaskContents: taskContents,
		Contexts:     contexts,
		Iteration:    iteration,
	})
	if err != nil {
		return "", fmt.Errorf("failed to render watch prompt: %w", err)
	}

	return buf.String(), nil
}
```

- [ ] **Step 8: Run all manual_test.go tests**

Run: `go test ./internal/cli/ -run "TestResolveContexts|TestManualSessionID|TestWatchSessionID|TestGenerateManualPrompt|TestGenerateWatchPrompt" -v`
Expected: PASS (all tests)

- [ ] **Step 9: Commit**

```bash
jj commit -m "feat: add session ID and prompt generation for manual/watch mode"
```

---

### Task 4: Add Flags to `agent.go`

**Files:**
- Modify: `internal/cli/agent.go:35-62` (flag variables)
- Modify: `internal/cli/agent.go:222-253` (init function)
- Modify: `internal/cli/agent.go:320-338` (AgentLoopConfig)

- [ ] **Step 1: Add flag variables**

In `internal/cli/agent.go`, add three new variables inside the existing `var` block (after `agentShowThinking string` at line ~55):

```go
	agentManual        bool     // Manual mode (bypass balls)
	agentWatch         string   // Watch mode directory
	agentContexts      []string // Context strings/files for manual/watch mode
```

- [ ] **Step 2: Register flags in init()**

In the `init()` function, add after the `agentShowThinking` flag registration (around line ~242):

```go
	agentRunCmd.Flags().BoolVar(&agentManual, "manual", false, "Manual mode: bypass balls, use --context for prompt content")
	agentRunCmd.Flags().StringVar(&agentWatch, "watch", "", "Watch mode: process task files from directory")
	agentRunCmd.Flags().StringArrayVar(&agentContexts, "context", nil, "Context to inject into prompt (string or @file). Repeatable. Requires --manual or --watch")
```

Note: Use `StringArrayVar` (not `StringSliceVar`) so that commas in values are not split.

- [ ] **Step 3: Add fields to AgentLoopConfig**

In the `AgentLoopConfig` struct, add after `ShowThinking bool`:

```go
	Manual        bool     // Manual mode: bypass balls, use contexts for prompt
	WatchDir      string   // Watch mode: process task files from this directory
	WatchTaskFile string   // Current task file path (set by watch loop, re-read each iteration)
	Contexts      []string // Resolved context values for manual/watch mode
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./cmd/juggle`
Expected: success (new fields exist but aren't used yet)

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat: add --manual, --watch, --context flag definitions"
```

---

### Task 5: Wire Manual Mode into `runAgentRun()`

**Files:**
- Modify: `internal/cli/agent.go:1506-1962` (runAgentRun function)

- [ ] **Step 1: Add validation at the top of `runAgentRun()`**

After `projectDir := cwd` (line ~1514), before the `--monitor` handling, add:

```go
	// Validate mutual exclusivity of run modes
	if agentManual && agentWatch != "" {
		return fmt.Errorf("cannot use --manual with --watch (they are mutually exclusive)")
	}
	if agentManual || agentWatch != "" {
		if agentBallID != "" {
			return fmt.Errorf("cannot use --ball with --manual or --watch")
		}
		if agentPickBall {
			return fmt.Errorf("cannot use --pick with --manual or --watch")
		}
		if len(args) > 0 {
			return fmt.Errorf("cannot specify session argument with --manual or --watch")
		}
	}
	if len(agentContexts) > 0 && !agentManual && agentWatch == "" {
		return fmt.Errorf("--context requires --manual or --watch")
	}
```

- [ ] **Step 2: Add manual/watch handling block**

After the validation block and before the `--monitor` handling, add the manual/watch mode branch that resolves contexts, creates a session, builds the config, and either calls `runWatchLoop` or `RunAgentLoop`:

```go
	// Handle manual mode or watch mode
	if agentManual || agentWatch != "" {
		// Resolve context values (@file or raw string)
		resolvedContexts, err := resolveContexts(agentContexts)
		if err != nil {
			return err
		}

		iterations := agentIterations

		// Handle --dry-run: generate and show the prompt, then exit
		if agentDryRun || agentDebug {
			var prompt string
			var err error
			if agentWatch != "" {
				prompt, err = generateWatchPrompt("(task file contents will appear here at runtime)", resolvedContexts, 1)
			} else {
				prompt, err = generateManualPrompt(resolvedContexts, 1)
			}
			if err != nil {
				return fmt.Errorf("failed to generate prompt: %w", err)
			}

			fmt.Println("=== Agent Prompt Info ===")
			fmt.Println()
			if agentManual {
				fmt.Println("Mode: manual")
			} else {
				fmt.Printf("Mode: watch (%s)\n", agentWatch)
			}
			fmt.Printf("Contexts: %d\n", len(resolvedContexts))
			fmt.Printf("Max iterations: %d\n", iterations)
			fmt.Println()
			fmt.Println("=== Generated Prompt ===")
			fmt.Println()
			fmt.Println(prompt)
			fmt.Println()
			fmt.Printf("=== Prompt Length: %d characters ===\n", len(prompt))

			if agentDryRun {
				fmt.Println()
				fmt.Println("(Dry run - agent not started)")
				return nil
			}

			fmt.Println()
			fmt.Println("=== Starting Agent ===")
			fmt.Println()
		}

		// Load iteration delay settings (same logic as ball mode)
		var iterDelay time.Duration
		var delayMinutes, fuzz int
		if cmd.Flags().Changed("delay") {
			delayMinutes = agentDelay
			if cmd.Flags().Changed("fuzz") {
				fuzz = agentFuzz
			}
		} else {
			delayMinutes, fuzz, _ = session.GetGlobalIterationDelayWithOptions(GetConfigOptions())
			if cmd.Flags().Changed("fuzz") {
				fuzz = agentFuzz
			}
		}
		if delayMinutes > 0 {
			iterDelay = calculateFuzzyDelay(delayMinutes, fuzz)
		}

		// Determine session ID
		var sessionID string
		if agentWatch != "" {
			sessionID = watchSessionID(agentWatch)
		} else {
			sessionID = manualSessionID(resolvedContexts)
		}

		// Ensure session exists
		sessionStore, err := session.NewSessionStoreWithConfig(projectDir, GetStoreConfig())
		if err != nil {
			return fmt.Errorf("failed to initialize session store: %w", err)
		}
		if _, err := sessionStore.LoadSession(sessionID); err != nil {
			desc := "manual mode session"
			if agentWatch != "" {
				desc = fmt.Sprintf("watch mode: %s", agentWatch)
			}
			if _, err := sessionStore.CreateSession(sessionID, desc); err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
		}

		showThinking := false
		if agentShowThinking == "true" {
			showThinking = true
		}

		if agentManual {
			fmt.Printf("Starting manual mode agent (session: %s)\n", sessionID)
		} else {
			fmt.Printf("Starting watch mode agent (dir: %s, session: %s)\n", agentWatch, sessionID)
		}
		fmt.Printf("Max iterations: %d\n", iterations)
		if len(resolvedContexts) > 0 {
			fmt.Printf("Contexts: %d\n", len(resolvedContexts))
		}
		fmt.Println()

		// Handle daemon mode (reuse existing daemon fork logic)
		storageID := sessionStorageID(sessionID)
		if os.Getenv("JUGGLE_DAEMON_CHILD") == "1" {
			os.Unsetenv("JUGGLE_DAEMON_CHILD")
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
			go func() {
				<-sigChan
				daemon.SendControlCommand(projectDir, storageID, daemon.CmdCancel, "signal")
			}()
		} else if agentDaemon {
			logPath := filepath.Join(projectDir, ".juggle", "sessions", storageID, "agent.log")
			if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
				return fmt.Errorf("failed to create session directory: %w", err)
			}
			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("failed to create log file: %w", err)
			}
			daemonCmd := exec.Command(os.Args[0], os.Args[1:]...)
			daemonCmd.Env = append(os.Environ(), "JUGGLE_DAEMON_CHILD=1")
			daemonCmd.Stdout = logFile
			daemonCmd.Stderr = logFile
			daemonCmd.Dir = projectDir
			if err := daemonCmd.Start(); err != nil {
				logFile.Close()
				return fmt.Errorf("failed to start daemon: %w", err)
			}
			fmt.Printf("Agent daemon started (PID %d)\n", daemonCmd.Process.Pid)
			fmt.Printf("Log file: %s\n", logPath)
			logFile.Close()
			return nil
		}

		loopConfig := AgentLoopConfig{
			SessionID:            sessionID,
			ProjectDir:           projectDir,
			MaxIterations:        iterations,
			Trust:                agentTrust,
			IterDelay:            iterDelay,
			Timeout:              agentTimeout,
			MaxWait:              agentMaxWait,
			Interactive:          agentInteractive,
			Model:                agentModel,
			OverloadRetryMinutes: -1,
			Provider:             agentProvider,
			IgnoreLock:           agentIgnoreLock,
			DaemonMode:           agentDaemon,
			ShowThinking:         showThinking,
			Manual:               true,
			WatchDir:             agentWatch,
			Contexts:             resolvedContexts,
		}

		if agentWatch != "" {
			return runWatchLoop(loopConfig)
		}

		result, err := RunAgentLoop(loopConfig)
		if err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("=== Summary ===")
		fmt.Printf("Iterations: %d\n", result.Iterations)
		if result.Complete {
			fmt.Println("Status: COMPLETE")
		} else if result.Blocked {
			fmt.Printf("Status: BLOCKED (%s)\n", result.BlockedReason)
		}
		return nil
	}
```

- [ ] **Step 3: Add `runWatchLoop` stub to `manual.go`**

Add to `internal/cli/manual.go` (implemented fully in Task 7):

```go
// runWatchLoop processes task files from a directory. Implemented in Task 7.
func runWatchLoop(config AgentLoopConfig) error {
	return fmt.Errorf("watch mode not yet implemented")
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./cmd/juggle`
Expected: success

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat: wire manual/watch mode into runAgentRun"
```

---

### Task 6: Manual Mode Branch in `RunAgentLoop()`

**Files:**
- Modify: `internal/cli/agent.go:349-975` (RunAgentLoop function)

This is the core change: when `config.Manual` is true, skip ball-related logic and use the manual/watch prompt generation instead.

- [ ] **Step 1: Skip ball pre-loop check**

In `RunAgentLoop()`, wrap the ball pre-loop check (lines ~533-556) with a condition. Find:

```go
	// Pre-loop check: is there any work the agent can do?
```

Wrap the entire block (through the closing `}` of `if workable == 0`) with:

```go
	if !config.Manual {
		// Pre-loop check: ...existing code unchanged...
	}
```

- [ ] **Step 2: Branch prompt generation and model selection inside the iteration loop**

In the iteration loop, before the existing model selection block that starts with `// Load balls for model selection`, declare the variables and add the manual branch:

```go
		var modelSelection modelSelectionResult
		var prompt string

		if config.Manual {
			// Manual mode: skip ball-based model selection, use flag or default to sonnet
			model := config.Model
			if model == "" {
				model = "sonnet"
			}
			modelSelection = modelSelectionResult{Model: model, Reason: "manual mode"}

			var promptErr error
			if config.WatchTaskFile != "" {
				// Watch mode: re-read the task file each iteration to get updated progress
				taskData, readErr := os.ReadFile(config.WatchTaskFile)
				if readErr != nil {
					return nil, fmt.Errorf("failed to read task file %s: %w", config.WatchTaskFile, readErr)
				}
				prompt, promptErr = generateWatchPrompt(string(taskData), config.Contexts, iteration)
			} else {
				prompt, promptErr = generateManualPrompt(config.Contexts, iteration)
			}
			if promptErr != nil {
				return nil, fmt.Errorf("failed to generate prompt: %w", promptErr)
			}
		} else {
```

Then close the `else` block after the existing prompt generation (`prompt, err := generateAgentPrompt(...)`). Change the existing inline declarations to assignments:

- `modelSelection := selectModelForIteration(...)` becomes `modelSelection = selectModelForIteration(...)`
- `prompt, err := generateAgentPrompt(...)` becomes `prompt, err = generateAgentPrompt(...)`

Close with `}` after the prompt generation error check.

- [ ] **Step 3: Add manual mode shortcut in COMPLETE signal handler**

After the progress validation succeeds for COMPLETE (the `} else {` branch), add a manual mode shortcut before the ball terminal check:

```go
			} else {
				if config.Manual {
					// Manual mode: no ball validation needed, accept COMPLETE
					if runResult.CommitMessage != "" {
						commitResult, commitErr := performJJCommit(config.ProjectDir, runResult.CommitMessage)
						if commitErr == nil && commitResult != nil && commitResult.Success && commitResult.CommitHash != "" {
							fmt.Printf("📝 Committed: %s\n", commitResult.CommitHash)
						}
					}
					result.Complete = true
					result.CommitMessage = runResult.CommitMessage
					break
				}
				// VALIDATE: Check if all balls are actually in terminal state...
```

- [ ] **Step 4: Add manual mode shortcut in CONTINUE signal handler**

In the CONTINUE handler, after progress validation passes, add before the ball-specific logic:

```go
				if config.Manual {
					fmt.Println()
					fmt.Printf("✓ Continuing to next iteration...\n")
					if runResult.CommitMessage != "" {
						commitResult, commitErr := performJJCommit(config.ProjectDir, runResult.CommitMessage)
						if commitErr == nil && commitResult != nil && commitResult.Success && commitResult.CommitHash != "" {
							fmt.Printf("📝 Committed: %s\n", commitResult.CommitHash)
						}
					}
					continue
				}
```

- [ ] **Step 5: Skip terminal state check at loop bottom**

Wrap the terminal state check (lines ~958-967) with:

```go
		if !config.Manual {
			// Check if all balls are in terminal state...existing code...
		}
```

- [ ] **Step 6: Verify it compiles**

Run: `go build ./cmd/juggle`
Expected: success

- [ ] **Step 7: Commit**

```bash
jj commit -m "feat: add manual mode branch in RunAgentLoop"
```

---

### Task 7: Watch Loop Implementation

**Files:**
- Modify: `internal/cli/manual.go` (replace stub)

- [ ] **Step 1: Replace `runWatchLoop` stub with full implementation**

Replace the stub in `manual.go`. Add `"path/filepath"`, `"time"` to imports:

```go
// runWatchLoop processes task files from a watched directory.
// For each file (alphabetical order), reads contents as the task,
// runs a sub-loop until COMPLETE or BLOCKED, then picks the next file.
// Idles when empty, polling at configured delay interval.
func runWatchLoop(config AgentLoopConfig) error {
	dir := config.WatchDir

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("watch directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("watch path %s is not a directory", dir)
	}

	pollDelay := config.IterDelay
	if pollDelay == 0 {
		pollDelay = 30 * time.Second
	}

	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("failed to read watch directory: %w", err)
		}

		// Pick first regular file (ReadDir returns sorted)
		var taskFile string
		for _, e := range entries {
			if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				taskFile = filepath.Join(dir, e.Name())
				break
			}
		}

		if taskFile == "" {
			fmt.Printf("⏳ Watch directory empty, polling in %v...\n", pollDelay.Round(time.Second))
			time.Sleep(pollDelay)
			continue
		}

		fmt.Printf("📋 Processing task: %s\n\n", filepath.Base(taskFile))

		subConfig := config
		subConfig.Manual = true
		subConfig.WatchTaskFile = taskFile

		result, err := RunAgentLoop(subConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Error processing %s: %v\n", filepath.Base(taskFile), err)
		}

		if result != nil {
			if result.Complete {
				fmt.Printf("✅ Task complete: %s\n\n", filepath.Base(taskFile))
			} else if result.Blocked {
				fmt.Printf("🚫 Task blocked: %s (%s)\n\n", filepath.Base(taskFile), result.BlockedReason)
			}
		}
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/juggle`
Expected: success

- [ ] **Step 3: Commit**

```bash
jj commit -m "feat: implement watch loop for directory-based task processing"
```

---

### Task 8: Integration Tests — Manual Mode

**Files:**
- Create: `internal/integration_test/manual_mode_test.go`

- [ ] **Step 1: Write manual mode integration tests**

Create `internal/integration_test/manual_mode_test.go`:

```go
package integration_test

import (
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/cli"
)

func TestManualMode_CompleteSignal(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "manual-aabbcc"
	env.CreateSession(t, sessionID, "manual mode test")
	sessionStore := env.GetSessionStore(t)

	mock := agent.NewMockRunner(
		&agent.RunResult{
			Output:   "Done.\n<promise>COMPLETE</promise>",
			Complete: true,
		},
	)
	agent.SetRunner(&progressUpdatingMockRunner{
		mock:         mock,
		sessionStore: sessionStore,
		sessionID:    sessionID,
	})
	defer agent.ResetRunner()

	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 5,
		IterDelay:     0,
		Manual:        true,
		Contexts:      []string{"do the thing"},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected result.Complete=true")
	}
	if mock.NextIndex != 1 {
		t.Errorf("Expected 1 call to runner, got %d", mock.NextIndex)
	}

	// Verify prompt contained context, not ball XML
	prompt := mock.Calls[0].Prompt
	if !strings.Contains(prompt, "do the thing") {
		t.Error("Expected prompt to contain context text")
	}
	if strings.Contains(prompt, "<balls>") {
		t.Error("Expected prompt to NOT contain <balls> tag in manual mode")
	}
}

func TestManualMode_ContinueThenComplete(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "manual-112233"
	env.CreateSession(t, sessionID, "manual mode continue test")
	sessionStore := env.GetSessionStore(t)

	mock := agent.NewMockRunner(
		&agent.RunResult{
			Output:   "Working...\n<promise>CONTINUE</promise>",
			Continue: true,
		},
		&agent.RunResult{
			Output:   "Done.\n<promise>COMPLETE</promise>",
			Complete: true,
		},
	)
	agent.SetRunner(&progressUpdatingMockRunner{
		mock:         mock,
		sessionStore: sessionStore,
		sessionID:    sessionID,
	})
	defer agent.ResetRunner()

	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 5,
		IterDelay:     0,
		Manual:        true,
		Contexts:      []string{"build a feature"},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected result.Complete=true")
	}
	if mock.NextIndex != 2 {
		t.Errorf("Expected 2 calls to runner, got %d", mock.NextIndex)
	}
}

func TestManualMode_NoContexts(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "manual-000000"
	env.CreateSession(t, sessionID, "bare manual mode")
	sessionStore := env.GetSessionStore(t)

	mock := agent.NewMockRunner(
		&agent.RunResult{
			Output:   "Done.\n<promise>COMPLETE</promise>",
			Complete: true,
		},
	)
	agent.SetRunner(&progressUpdatingMockRunner{
		mock:         mock,
		sessionStore: sessionStore,
		sessionID:    sessionID,
	})
	defer agent.ResetRunner()

	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 1,
		IterDelay:     0,
		Manual:        true,
		Contexts:      []string{},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected result.Complete=true for bare manual mode")
	}

	prompt := mock.Calls[0].Prompt
	if !strings.Contains(prompt, "<instructions>") {
		t.Error("Expected prompt to contain <instructions>")
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test -v ./internal/integration_test/ -run "TestManualMode"`
Expected: PASS (all 3 tests)

- [ ] **Step 3: Commit**

```bash
jj commit -m "test: add manual mode integration tests"
```

---

### Task 9: Integration Tests — Watch Mode

**Files:**
- Create: `internal/integration_test/watch_mode_test.go`

- [ ] **Step 1: Write watch mode integration tests**

Create `internal/integration_test/watch_mode_test.go`:

```go
package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/cli"
	"github.com/ohare93/juggle/internal/session"
)

func TestWatchMode_ProcessesTaskFile(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "watch-aabbcc"
	env.CreateSession(t, sessionID, "watch mode test")
	sessionStore := env.GetSessionStore(t)

	// Create a task file
	taskDir := filepath.Join(env.ProjectDir, "queue", "ready")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("Failed to create task dir: %v", err)
	}
	taskFile := filepath.Join(taskDir, "01-test-task.md")
	if err := os.WriteFile(taskFile, []byte("# Test Task\nDo the thing."), 0644); err != nil {
		t.Fatalf("Failed to write task file: %v", err)
	}

	mock := agent.NewMockRunner(
		&agent.RunResult{
			Output:   "Done.\n<promise>COMPLETE</promise>",
			Complete: true,
		},
	)
	agent.SetRunner(&progressUpdatingMockRunner{
		mock:         mock,
		sessionStore: sessionStore,
		sessionID:    sessionID,
	})
	defer agent.ResetRunner()

	// Test the sub-loop directly (not runWatchLoop which loops forever)
	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 5,
		IterDelay:     0,
		Manual:        true,
		WatchTaskFile: taskFile,
		Contexts:      []string{"worker instructions here"},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected result.Complete=true")
	}

	prompt := mock.Calls[0].Prompt
	if !strings.Contains(prompt, "Test Task") {
		t.Error("Expected prompt to contain task file contents")
	}
	if !strings.Contains(prompt, "<task>") {
		t.Error("Expected prompt to contain <task> tag")
	}
	if !strings.Contains(prompt, "worker instructions here") {
		t.Error("Expected prompt to contain context")
	}
}

func TestWatchMode_ReReadsTaskFile(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "watch-112233"
	env.CreateSession(t, sessionID, "watch re-read test")
	sessionStore := env.GetSessionStore(t)

	taskDir := filepath.Join(env.ProjectDir, "queue")
	os.MkdirAll(taskDir, 0755)
	taskFile := filepath.Join(taskDir, "task.md")
	os.WriteFile(taskFile, []byte("# Task\nOriginal content"), 0644)

	callCount := 0
	customRunner := &funcRunner{
		fn: func(opts agent.RunOptions) (*agent.RunResult, error) {
			callCount++
			_ = sessionStore.AppendProgress(sessionID, "[progress]\n")

			if callCount == 1 {
				// Simulate agent appending progress to task file
				f, _ := os.OpenFile(taskFile, os.O_APPEND|os.O_WRONLY, 0644)
				f.WriteString("\n## Progress\n- Did step 1\n")
				f.Close()

				return &agent.RunResult{
					Output:   "<promise>CONTINUE</promise>",
					Continue: true,
				}, nil
			}
			return &agent.RunResult{
				Output:   "<promise>COMPLETE</promise>",
				Complete: true,
			}, nil
		},
	}
	agent.SetRunner(customRunner)
	defer agent.ResetRunner()

	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 5,
		IterDelay:     0,
		Manual:        true,
		WatchTaskFile: taskFile,
		Contexts:      []string{},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected complete")
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

// funcRunner is a simple Runner implementation that calls a function.
type funcRunner struct {
	fn func(opts agent.RunOptions) (*agent.RunResult, error)
}

func (f *funcRunner) Run(opts agent.RunOptions) (*agent.RunResult, error) {
	return f.fn(opts)
}
```

- [ ] **Step 2: Run tests**

Run: `go test -v ./internal/integration_test/ -run "TestWatchMode"`
Expected: PASS (both tests)

- [ ] **Step 3: Commit**

```bash
jj commit -m "test: add watch mode integration tests"
```

---

### Task 10: Full Test Suite and Smoke Test

**Files:** None (verification only)

- [ ] **Step 1: Run all unit tests**

Run: `go test ./internal/cli/ -v`
Expected: PASS (all existing + new tests)

- [ ] **Step 2: Run all integration tests**

Run: `go test -v ./internal/integration_test/...`
Expected: PASS (all existing tests still pass, new manual/watch tests pass)

- [ ] **Step 3: Build the binary**

Run: `go build -o juggle ./cmd/juggle`
Expected: success

- [ ] **Step 4: Smoke test — manual mode**

Run: `./juggle agent run --manual --context "hello world" --dry-run`
Expected: shows the generated prompt with `<context>` containing "hello world" and the instructions template

- [ ] **Step 5: Smoke test — mutual exclusivity**

Run: `./juggle agent run --manual --watch /tmp --context "test"`
Expected: error — cannot use --manual with --watch

Run: `./juggle agent run --context "test"`
Expected: error — --context requires --manual or --watch

Run: `./juggle agent run --manual --ball some-ball`
Expected: error — cannot use --ball with --manual or --watch

- [ ] **Step 6: Commit (if any fixes were needed)**

```bash
jj commit -m "fix: test suite fixes for manual/watch mode"
```
