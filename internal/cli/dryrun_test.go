package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintDryRun_OnlyMainPrompt(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Content:    "fix the tests",
		Iterations: 5,
	}
	printDryRun(cfg, &buf)
	out := buf.String()

	if !strings.Contains(out, "fix the tests") {
		t.Error("missing main prompt content")
	}
	if !strings.Contains(out, "iteration 1 of 5") {
		t.Error("missing iteration footer in main prompt")
	}
	if !strings.Contains(out, "[main prompt]") {
		t.Error("missing main prompt section header")
	}
	// No other sections configured — none should appear
	for _, section := range []string{"[agent-pre]", "[cmd-before]", "[agent-before]", "[agent-after]", "[cmd-after]", "[stop-when]", "[agent-post]", "[hooks]"} {
		if strings.Contains(out, section) {
			t.Errorf("unexpected section %q in output", section)
		}
	}
}

func TestPrintDryRun_AllSections(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Content:     "do work",
		Iterations:  3,
		AgentPre:    "pre agent prompt",
		CmdBefore:   "make build",
		AgentBefore: "before agent prompt",
		AgentAfter:  "after agent prompt",
		CmdAfter:    "make test",
		StopWhen:    "test -f done",
		AgentPost:   "post agent prompt",
		Hooks:       []string{"PreToolUse:echo pre", "PostToolUse:echo post"},
	}
	printDryRun(cfg, &buf)
	out := buf.String()

	// All expected sections present
	for _, section := range []string{"[agent-pre]", "[cmd-before]", "[agent-before]", "[main prompt]", "[agent-after]", "[cmd-after]", "[stop-when]", "[agent-post]", "[hooks]"} {
		if !strings.Contains(out, section) {
			t.Errorf("missing section %q", section)
		}
	}

	// Content in each section
	if !strings.Contains(out, "pre agent prompt") {
		t.Error("missing agent-pre content")
	}
	if !strings.Contains(out, "make build") {
		t.Error("missing cmd-before command")
	}
	if !strings.Contains(out, "before agent prompt") {
		t.Error("missing agent-before content")
	}
	if !strings.Contains(out, "do work") {
		t.Error("missing main prompt content")
	}
	if !strings.Contains(out, "after agent prompt") {
		t.Error("missing agent-after content")
	}
	if !strings.Contains(out, "make test") {
		t.Error("missing cmd-after command")
	}
	if !strings.Contains(out, "test -f done") {
		t.Error("missing stop-when command")
	}
	if !strings.Contains(out, "post agent prompt") {
		t.Error("missing agent-post content")
	}
	if !strings.Contains(out, "PreToolUse:echo pre") {
		t.Error("missing hook spec")
	}
}

func TestPrintDryRun_ExecutionOrder(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Content:     "main",
		Iterations:  1,
		AgentPre:    "pre",
		CmdBefore:   "before-cmd",
		AgentBefore: "before-agent",
		AgentAfter:  "after-agent",
		CmdAfter:    "after-cmd",
		StopWhen:    "stop-cmd",
		AgentPost:   "post",
	}
	printDryRun(cfg, &buf)
	out := buf.String()

	// Verify order by finding index of each section header
	order := []string{
		"[agent-pre]",
		"[cmd-before]",
		"[agent-before]",
		"[main prompt]",
		"[agent-after]",
		"[cmd-after]",
		"[stop-when]",
		"[agent-post]",
	}
	prev := 0
	for _, section := range order {
		idx := strings.Index(out, section)
		if idx < 0 {
			t.Fatalf("missing section %q", section)
		}
		if idx < prev {
			t.Errorf("section %q appears before previous section (out of order)", section)
		}
		prev = idx
	}
}

func TestPrintDryRun_WatchMode(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Content:    "process tasks",
		Iterations: 2,
		Watch:      "/tasks",
	}
	printDryRun(cfg, &buf)
	out := buf.String()

	if !strings.Contains(out, "[main prompt]") {
		t.Error("missing main prompt section")
	}
	if !strings.Contains(out, "process tasks") {
		t.Error("missing content in watch prompt")
	}
	// Should show a placeholder for the task file
	if !strings.Contains(out, "<task") {
		t.Error("watch mode dry-run should show task placeholder")
	}
}

func TestPrintDryRun_OmitsEmptySections(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Content:    "do work",
		Iterations: 1,
		CmdBefore:  "make build",
		// Everything else empty
	}
	printDryRun(cfg, &buf)
	out := buf.String()

	if !strings.Contains(out, "[cmd-before]") {
		t.Error("cmd-before section should appear")
	}
	if !strings.Contains(out, "[main prompt]") {
		t.Error("main prompt section should appear")
	}
	for _, absent := range []string{"[agent-pre]", "[agent-before]", "[agent-after]", "[cmd-after]", "[stop-when]", "[agent-post]", "[hooks]"} {
		if strings.Contains(out, absent) {
			t.Errorf("section %q should be omitted when not configured", absent)
		}
	}
}

func TestPrintDryRun_HooksList(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{
		Content:    "work",
		Iterations: 1,
		Hooks:      []string{"PreToolUse:echo hello", "Stop:exit 0"},
	}
	printDryRun(cfg, &buf)
	out := buf.String()

	if !strings.Contains(out, "[hooks]") {
		t.Error("hooks section missing")
	}
	if !strings.Contains(out, "PreToolUse:echo hello") {
		t.Error("first hook missing")
	}
	if !strings.Contains(out, "Stop:exit 0") {
		t.Error("second hook missing")
	}
}

// TestRun_DryRun_Verbose checks that Run() with DryRun=true now shows section headers.
func TestRun_DryRun_Verbose(t *testing.T) {
	var stdout bytes.Buffer
	cfg := Config{
		Content:    "fix the tests",
		Iterations: 10,
		DryRun:     true,
		AgentPre:   "pre prompt",
		CmdBefore:  "make lint",
		Stdout:     &stdout,
	}
	err := Run(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "[main prompt]") {
		t.Error("dry-run verbose missing main prompt section header")
	}
	if !strings.Contains(out, "[agent-pre]") {
		t.Error("dry-run verbose missing agent-pre section")
	}
	if !strings.Contains(out, "[cmd-before]") {
		t.Error("dry-run verbose missing cmd-before section")
	}
	if !strings.Contains(out, "fix the tests") {
		t.Error("dry-run verbose missing main content")
	}
}
