package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
)

func TestScanWatchDir(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("picks first file alphabetically", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "b-task.md"), []byte("b"), 0644)
		os.WriteFile(filepath.Join(dir, "a-task.md"), []byte("a"), 0644)
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "a-task.md" {
			t.Errorf("expected a-task.md, got %q", filepath.Base(got))
		}
	})

	t.Run("skips hidden files", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "visible.md"), []byte("y"), 0644)
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "visible.md" {
			t.Errorf("expected visible.md, got %q", filepath.Base(got))
		}
	})

	t.Run("skips directories", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, "subdir"), 0755)
		os.WriteFile(filepath.Join(dir, "task.md"), []byte("t"), 0644)
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "task.md" {
			t.Errorf("expected task.md, got %q", filepath.Base(got))
		}
	})

	t.Run("numeric prefixes control order", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "02-low.md"), []byte("l"), 0644)
		os.WriteFile(filepath.Join(dir, "01-high.md"), []byte("h"), 0644)
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "01-high.md" {
			t.Errorf("expected 01-high.md, got %q", filepath.Base(got))
		}
	})
}

func TestRunWatchTask_Iterations(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("implement feature"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "iter1"},
		&agent.RunResult{Output: "iter2"},
	)

	cfg := Config{
		Content:    "context",
		Iterations: 2,
		Model:      "sonnet",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}

	err := runWatchTask(cfg, taskPath, "task.md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(mock.Calls))
	}

	// Both prompts should contain task contents and filename
	for i, call := range mock.Calls {
		if !strings.Contains(call.Prompt, "implement feature") {
			t.Errorf("call %d missing task contents", i)
		}
		if !strings.Contains(call.Prompt, "task.md") {
			t.Errorf("call %d missing filename", i)
		}
	}
}

func TestRunWatchTask_FileDeletedByAgent(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	// Don't create the file — simulate agent already deleted it

	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:    "context",
		Iterations: 5,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}

	err := runWatchTask(cfg, taskPath, "task.md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return immediately since file doesn't exist
	if len(mock.Calls) != 0 {
		t.Errorf("expected 0 calls when file missing, got %d", len(mock.Calls))
	}
}

func TestRunWatch_InvalidDirectory(t *testing.T) {
	cfg := Config{
		Watch:  "/nonexistent/directory",
		Stderr: &bytes.Buffer{},
	}
	err := RunWatch(cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestRunWatch_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notadir")
	os.WriteFile(filePath, []byte("x"), 0644)

	cfg := Config{
		Watch:  filePath,
		Stderr: &bytes.Buffer{},
	}
	err := RunWatch(cfg)
	if err == nil {
		t.Fatal("expected error for non-directory path")
	}
}

func TestRunWatchTask_ShutdownPreventNextIteration(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	shutdown := make(chan struct{})
	runner := &closeOnFirstCallRunner{shutdown: shutdown}
	cfg := Config{
		Content:    "context",
		Iterations: 5,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}
	err := runWatchTask(cfg, taskPath, "task.md", nil)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if runner.calls != 1 {
		t.Errorf("expected 1 call, got %d", runner.calls)
	}
}

func TestRunWatch_PreClosedShutdown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "task.md"), []byte("task"), 0644)

	shutdown := make(chan struct{})
	close(shutdown)
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Watch:    dir,
		Runner:   mock,
		Shutdown: shutdown,
		Stderr:   &bytes.Buffer{},
	}
	err := RunWatch(cfg)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected 0 calls with pre-closed shutdown, got %d", len(mock.Calls))
	}
}

func TestRunWatchTask_ConsecutiveFailuresStop(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0}, // should not be reached
	)
	cfg := Config{
		Content:     "context",
		Iterations:  10,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	err := runWatchTask(cfg, taskPath, "task.md", nil)
	if err == nil {
		t.Fatal("expected error on consecutive failures")
	}
	if !strings.Contains(err.Error(), "3 consecutive failures") {
		t.Errorf("error should mention '3 consecutive failures', got: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls before stop, got %d", len(mock.Calls))
	}
}

func TestRunWatchTask_ConsecutiveFailureCounterResets(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0}, // resets counter
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
	)
	cfg := Config{
		Content:     "context",
		Iterations:  5,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	err := runWatchTask(cfg, taskPath, "task.md", nil)
	if err != nil {
		t.Fatalf("counter should reset on success, got: %v", err)
	}
	if len(mock.Calls) != 5 {
		t.Errorf("expected 5 calls, got %d", len(mock.Calls))
	}
}

func TestBuildRunOptions(t *testing.T) {
	t.Run("headless mode with accept edits", func(t *testing.T) {
		cfg := Config{
			Model:        "sonnet",
			Timeout:      0,
			ShowThinking: false,
		}
		opts := buildRunOptions(cfg, "test prompt")
		if opts.Mode != agent.ModeHeadless {
			t.Errorf("expected headless, got %s", opts.Mode)
		}
		if opts.Permission != agent.PermissionAcceptEdits {
			t.Errorf("expected acceptEdits, got %s", opts.Permission)
		}
		if opts.Prompt != "test prompt" {
			t.Errorf("expected 'test prompt', got %q", opts.Prompt)
		}
		if opts.Model != "sonnet" {
			t.Errorf("expected sonnet, got %q", opts.Model)
		}
	})

	t.Run("plan mode", func(t *testing.T) {
		cfg := Config{Plan: true}
		opts := buildRunOptions(cfg, "prompt")
		if opts.Permission != agent.PermissionPlan {
			t.Errorf("expected plan, got %s", opts.Permission)
		}
	})

	t.Run("interactive mode with bypass", func(t *testing.T) {
		cfg := Config{
			Interactive:  true,
			Trust:        true,
			Model:        "opus",
			ShowThinking: true,
		}
		opts := buildRunOptions(cfg, "prompt")
		if opts.Mode != agent.ModeInteractive {
			t.Errorf("expected interactive, got %s", opts.Mode)
		}
		if opts.Permission != agent.PermissionBypass {
			t.Errorf("expected bypass, got %s", opts.Permission)
		}
		if !opts.ShowThinking {
			t.Error("expected ShowThinking=true")
		}
	})
}

func TestRunWatchTask_StopWhenExitsZeroStops(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("do work"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "iter1"},
		&agent.RunResult{Output: "iter2"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "context",
		Iterations: 10,
		Runner:     mock,
		Stderr:     &stderr,
		StopWhen:   "true", // always exits 0
	}

	err := runWatchTask(cfg, taskPath, "task.md", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 call before stop-when triggered, got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "stop-when") {
		t.Errorf("expected stop-when message in stderr, got: %s", stderr.String())
	}
}

func TestRunWatchTask_MaxCostGuardTriggersAtThreshold(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("do work"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens}, // should not be reached
	)
	var stderr bytes.Buffer
	stats := &runStats{model: "sonnet"}
	cfg := Config{
		Content:    "context",
		Iterations: 5,
		Model:      "sonnet",
		MaxCost:    costGuardMaxCost,
		Runner:     mock,
		Stderr:     &stderr,
	}

	err := runWatchTask(cfg, taskPath, "task.md", stats)
	if !errors.Is(err, errCostGuard) {
		t.Fatalf("expected errCostGuard, got: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 call before cost guard triggered, got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "cost guard triggered") {
		t.Errorf("expected 'cost guard triggered' in stderr, got: %s", stderr.String())
	}
}

func TestRunWatch_MaxCostGuardPrintsSummaryAndExitsClean(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "task.md"), []byte("do work"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Watch:      dir,
		Content:    "context",
		Iterations: 5,
		Model:      "sonnet",
		MaxCost:    costGuardMaxCost,
		Runner:     mock,
		Stderr:     &stderr,
	}

	err := RunWatch(cfg)
	if err != nil {
		t.Fatalf("cost guard in watch mode should exit cleanly (nil), got: %v", err)
	}
	if !strings.Contains(stderr.String(), "Run summary") {
		t.Errorf("expected 'Run summary' in stderr on watch cost guard exit, got: %s", stderr.String())
	}
}
