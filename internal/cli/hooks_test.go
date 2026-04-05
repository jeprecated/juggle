package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
)

// TestRunHook_Success verifies a successful hook command runs and returns nil.
func TestRunHook_Success(t *testing.T) {
	var stderr bytes.Buffer
	err := runHook("echo hello", hookEnv{iteration: 1, maxIterations: 3}, &stderr)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

// TestRunHook_Failure verifies a non-zero exit returns an error.
func TestRunHook_Failure(t *testing.T) {
	var stderr bytes.Buffer
	err := runHook("exit 1", hookEnv{iteration: 1, maxIterations: 3}, &stderr)
	if err == nil {
		t.Fatal("expected error for failing hook")
	}
}

// TestRunHook_Empty verifies empty command is a no-op.
func TestRunHook_Empty(t *testing.T) {
	var stderr bytes.Buffer
	err := runHook("", hookEnv{iteration: 1, maxIterations: 3}, &stderr)
	if err != nil {
		t.Fatalf("empty hook should be no-op, got: %v", err)
	}
}

// TestRunHook_FileRef verifies @file references are resolved and executed.
func TestRunHook_FileRef(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "hook.sh")
	os.WriteFile(script, []byte("#!/bin/sh\necho ran\n"), 0755)

	var stderr bytes.Buffer
	err := runHook("@"+script, hookEnv{iteration: 1, maxIterations: 3}, &stderr)
	if err != nil {
		t.Fatalf("file hook should succeed, got: %v", err)
	}
}

// TestRunHook_FileRefMissing verifies a missing @file reference returns an error.
func TestRunHook_FileRefMissing(t *testing.T) {
	var stderr bytes.Buffer
	err := runHook("@/nonexistent/hook.sh", hookEnv{iteration: 1, maxIterations: 3}, &stderr)
	if err == nil {
		t.Fatal("expected error for missing @file hook")
	}
}

// TestRunHook_SetsEnvVars verifies environment variables are passed to hook.
func TestRunHook_SetsEnvVars(t *testing.T) {
	var stderr bytes.Buffer
	// Command that prints the env var; output goes to stderr
	err := runHook(`sh -c 'echo ITER=$JUGGLE_ITERATION' >&2`, hookEnv{iteration: 7, maxIterations: 10}, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "ITER=7") {
		t.Errorf("expected ITER=7 in stderr output, got: %q", out)
	}
}

// TestRunHook_AfterEnvVars verifies exit code and token env vars are set.
func TestRunHook_AfterEnvVars(t *testing.T) {
	var stderr bytes.Buffer
	env := hookEnv{
		iteration:     2,
		maxIterations: 5,
		exitCode:      42,
		inputTokens:   100,
		outputTokens:  200,
	}
	err := runHook(`sh -c 'echo EXIT=$JUGGLE_EXIT_CODE IN=$JUGGLE_INPUT_TOKENS OUT=$JUGGLE_OUTPUT_TOKENS' >&2`, env, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "EXIT=42") {
		t.Errorf("expected EXIT=42, got: %q", out)
	}
	if !strings.Contains(out, "IN=100") {
		t.Errorf("expected IN=100, got: %q", out)
	}
	if !strings.Contains(out, "OUT=200") {
		t.Errorf("expected OUT=200, got: %q", out)
	}
}

// TestRunLoop_CmdBeforeSkipsOnFailure verifies that cmd-before failure skips the iteration.
func TestRunLoop_CmdBeforeSkipsOnFailure(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "should not be called"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		CmdBefore:  "exit 1",
		Runner:     mock,
		Stderr:     &stderr,
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("cmd-before failure should not return error, got: %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected 0 runner calls when cmd-before fails, got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "cmd-before") {
		t.Errorf("expected warning about cmd-before in stderr, got: %q", stderr.String())
	}
}

// TestRunLoop_CmdAfterFailureLogsWarning verifies cmd-after failure logs warning but doesn't stop loop.
func TestRunLoop_CmdAfterFailureLogsWarning(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		CmdAfter:   "exit 1",
		Runner:     mock,
		Stderr:     &stderr,
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("cmd-after failure should not stop loop, got: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 runner call, got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "cmd-after") {
		t.Errorf("expected warning about cmd-after in stderr, got: %q", stderr.String())
	}
}

// TestRunLoop_CmdBeforeSuccess verifies normal iteration when cmd-before succeeds.
func TestRunLoop_CmdBeforeSuccess(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok"},
	)
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		CmdBefore:  "true",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 runner call, got %d", len(mock.Calls))
	}
}

// TestRunWatchTask_CmdBeforeSkipsOnFailure verifies cmd-before skips iteration in watch mode.
func TestRunWatchTask_CmdBeforeSkipsOnFailure(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "should not be called"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "context",
		Iterations: 1,
		CmdBefore:  "exit 1",
		Runner:     mock,
		Stderr:     &stderr,
	}
	err := runWatchTask(cfg, taskPath, "task.md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected 0 runner calls, got %d", len(mock.Calls))
	}
}

// TestRunWatchTask_CmdAfterFailureLogsWarning verifies cmd-after in watch mode.
func TestRunWatchTask_CmdAfterFailureLogsWarning(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "context",
		Iterations: 1,
		CmdAfter:   "exit 1",
		Runner:     mock,
		Stderr:     &stderr,
	}
	err := runWatchTask(cfg, taskPath, "task.md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 runner call, got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "cmd-after") {
		t.Errorf("expected cmd-after warning in stderr, got: %q", stderr.String())
	}
}

// TestRunHook_RunLevelEnvVars verifies run-level env vars are passed to hook.
func TestRunHook_RunLevelEnvVars(t *testing.T) {
	var stderr bytes.Buffer
	env := hookEnv{
		iteration:     1,
		maxIterations: 3,
		runID:         "abc123",
		label:         "my-label",
		model:         "claude-opus-4-5",
		provider:      "claude",
	}
	err := runHook(`sh -c 'echo RID=$JUGGLE_RUN_ID LBL=$JUGGLE_LABEL MDL=$JUGGLE_MODEL PRV=$JUGGLE_PROVIDER' >&2`, env, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "RID=abc123") {
		t.Errorf("expected RID=abc123, got: %q", out)
	}
	if !strings.Contains(out, "LBL=my-label") {
		t.Errorf("expected LBL=my-label, got: %q", out)
	}
	if !strings.Contains(out, "MDL=claude-opus-4-5") {
		t.Errorf("expected MDL=claude-opus-4-5, got: %q", out)
	}
	if !strings.Contains(out, "PRV=claude") {
		t.Errorf("expected PRV=claude, got: %q", out)
	}
}

// TestRunHook_EmptyLabelOmitted verifies JUGGLE_LABEL is not set when label is empty.
func TestRunHook_EmptyLabelOmitted(t *testing.T) {
	var stderr bytes.Buffer
	env := hookEnv{
		iteration:     1,
		maxIterations: 3,
		runID:         "xyz",
		label:         "",
		model:         "gpt-4",
		provider:      "openai",
	}
	// Print whether JUGGLE_LABEL is set via ${JUGGLE_LABEL+SET} trick
	err := runHook(`sh -c 'echo LABEL_SET=${JUGGLE_LABEL+SET}' >&2`, env, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stderr.String()
	if strings.Contains(out, "LABEL_SET=SET") {
		t.Errorf("expected JUGGLE_LABEL to be unset when label is empty, got: %q", out)
	}
}

// --- lifecycle markers ---

func TestRunLoop_CmdBefore_PrintsMarker(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		CmdBefore:  "make lint",
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "cmd-before") {
		t.Errorf("expected 'cmd-before' marker in stderr, got: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "make lint") {
		t.Errorf("expected command in marker, got: %q", stderr.String())
	}
}

func TestRunLoop_CmdAfter_PrintsMarker(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		CmdAfter:   "echo done",
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "cmd-after") {
		t.Errorf("expected 'cmd-after' marker in stderr, got: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "echo done") {
		t.Errorf("expected command in marker, got: %q", stderr.String())
	}
}

func TestRunWatchTask_CmdBefore_PrintsMarker(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "context",
		Iterations: 1,
		CmdBefore:  "make build",
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := runWatchTask(cfg, taskPath, "task.md", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "cmd-before") {
		t.Errorf("expected 'cmd-before' marker in stderr, got: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "make build") {
		t.Errorf("expected command in marker, got: %q", stderr.String())
	}
}

func TestRunWatchTask_CmdAfter_PrintsMarker(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "context",
		Iterations: 1,
		CmdAfter:   "echo done",
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := runWatchTask(cfg, taskPath, "task.md", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "cmd-after") {
		t.Errorf("expected 'cmd-after' marker in stderr, got: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "echo done") {
		t.Errorf("expected command in marker, got: %q", stderr.String())
	}
}
