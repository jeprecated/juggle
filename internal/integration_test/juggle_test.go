package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/cli"
)

func TestBasicRun(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "completed"})

	var stdout, stderr bytes.Buffer
	cfg := cli.Config{
		Content:    "fix the tests",
		Iterations: 1,
		Model:      "sonnet",
		Runner:     mock,
		Stdout:     &stdout,
		Stderr:     &stderr,
	}

	err := cli.Run(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.Calls))
	}
	if !strings.Contains(mock.Calls[0].Prompt, "fix the tests") {
		t.Error("prompt missing content")
	}
}

func TestMultipleIterations(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "1"},
		&agent.RunResult{Output: "2"},
		&agent.RunResult{Output: "3"},
	)

	cfg := cli.Config{
		Content:    "work",
		Iterations: 3,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}

	err := cli.Run(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(mock.Calls))
	}
}

func TestDryRun(t *testing.T) {
	var stdout bytes.Buffer
	cfg := cli.Config{
		Content:    "test prompt",
		Iterations: 10,
		DryRun:     true,
		Stdout:     &stdout,
	}

	err := cli.Run(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "test prompt") {
		t.Error("dry-run missing content")
	}
	if !strings.Contains(output, "iteration 1 of 10") {
		t.Error("dry-run missing footer")
	}
}

func TestFileResolution(t *testing.T) {
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "task.md")
	os.WriteFile(taskFile, []byte("implement feature"), 0644)

	resolved, err := cli.ResolveArgs([]string{"@" + taskFile, "extra context"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("expected 2, got %d", len(resolved))
	}
	if resolved[0] != "implement feature" {
		t.Errorf("file content = %q, want 'implement feature'", resolved[0])
	}
	if resolved[1] != "extra context" {
		t.Errorf("raw arg = %q, want 'extra context'", resolved[1])
	}
}

func TestTrustMode(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := cli.Config{
		Content: "work", Iterations: 1, Trust: true,
		Runner: mock, Stderr: &bytes.Buffer{},
	}
	cli.Run(cfg)
	if mock.Calls[0].Permission != agent.PermissionBypass {
		t.Errorf("expected bypass, got %s", mock.Calls[0].Permission)
	}
}

func TestInteractiveMode(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := cli.Config{
		Content: "work", Iterations: 1, Interactive: true,
		Runner: mock, Stderr: &bytes.Buffer{},
	}
	cli.Run(cfg)
	if mock.Calls[0].Mode != agent.ModeInteractive {
		t.Errorf("expected interactive, got %s", mock.Calls[0].Mode)
	}
}

func TestPromptFormat(t *testing.T) {
	prompt := cli.BuildPrompt("content here", 3, 10)
	expected := "content here\n\n---\nThis is iteration 3 of 10.\n"
	if prompt != expected {
		t.Errorf("prompt = %q\nwant  = %q", prompt, expected)
	}
}

func TestWatchPromptFormat(t *testing.T) {
	prompt := cli.BuildWatchPrompt("task data", "context", "task.md", 2, 5)
	if !strings.HasPrefix(prompt, "<task>\ntask data\n</task>") {
		t.Error("should start with task section")
	}
	if !strings.Contains(prompt, "context") {
		t.Error("missing context")
	}
	if !strings.Contains(prompt, "processing task.md") {
		t.Error("missing filename")
	}
}

func TestWatchTaskProcessing(t *testing.T) {
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "001-task.md")
	os.WriteFile(taskFile, []byte("do the thing"), 0644)

	mock := agent.NewMockRunner(&agent.RunResult{Output: "done"})

	var stderr bytes.Buffer
	cfg := cli.Config{
		Content:    "instructions",
		Watch:      []string{dir},
		Iterations: 1,
		Model:      "sonnet",
		Runner:     mock,
		Stderr:     &stderr,
	}
	// Silence the "cfg unused" warning — cfg is here to show the full pipeline shape.
	_ = cfg

	// RunWatch is an infinite loop, so we test the inner function instead.
	// ScanWatchDir should find the task file.
	taskPath, err := cli.ScanWatchDir(dir)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if filepath.Base(taskPath) != "001-task.md" {
		t.Fatalf("expected 001-task.md, got %s", filepath.Base(taskPath))
	}

	// Verify the watch prompt format includes task contents
	prompt := cli.BuildWatchPrompt("do the thing", "instructions", "001-task.md", 1, 1)
	if !strings.Contains(prompt, "do the thing") {
		t.Error("watch prompt missing task contents")
	}
	if !strings.Contains(prompt, "instructions") {
		t.Error("watch prompt missing context")
	}
	if !strings.Contains(prompt, "001-task.md") {
		t.Error("watch prompt missing filename")
	}
}

func TestWatchInvalidDirectory(t *testing.T) {
	cfg := cli.Config{
		Watch:  []string{"/nonexistent/directory"},
		Stderr: &bytes.Buffer{},
	}
	err := cli.RunWatch(cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}
