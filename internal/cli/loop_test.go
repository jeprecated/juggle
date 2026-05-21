package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jeprecated/juggle/internal/agent"
	"github.com/spf13/cobra"
)

// findLoopCmd returns the loop subcommand or fatals.
func findLoopCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd, _, err := rootCmd.Find([]string{"loop"})
	if err != nil || cmd == nil || cmd.Name() != "loop" {
		t.Fatal("loop subcommand not found on rootCmd")
	}
	return cmd
}

func TestLoopCmdExistsOnRootCmd(t *testing.T) {
	findLoopCmd(t) // fails if not found
}

func TestLoopCmdUse(t *testing.T) {
	cmd := findLoopCmd(t)
	if !strings.HasPrefix(cmd.Use, "loop") {
		t.Errorf("loopCmd.Use should start with 'loop', got %q", cmd.Use)
	}
}

func TestLoopCmdHasIterationsFlag(t *testing.T) {
	cmd := findLoopCmd(t)
	f := cmd.Flags().Lookup("iterations")
	if f == nil {
		t.Fatal("loopCmd missing --iterations flag")
	}
	if cmd.Flags().ShorthandLookup("n") == nil {
		t.Error("loopCmd missing -n shorthand for --iterations")
	}
}

func TestLoopCmdHasDelayFlag(t *testing.T) {
	cmd := findLoopCmd(t)
	if cmd.Flags().Lookup("delay") == nil {
		t.Fatal("loopCmd missing --delay flag")
	}
}

func TestLoopCmdHasResumeFlag(t *testing.T) {
	cmd := findLoopCmd(t)
	if cmd.Flags().Lookup("resume") == nil {
		t.Fatal("loopCmd missing --resume flag")
	}
}

func TestLoopCmdHasContinueFlag(t *testing.T) {
	cmd := findLoopCmd(t)
	if cmd.Flags().Lookup("continue") == nil {
		t.Fatal("loopCmd missing --continue flag")
	}
}

// TestLoopCmdHasSharedFlags checks all flags registered by registerSharedFlags plus loop-only flags.
func TestLoopCmdHasSharedFlags(t *testing.T) {
	cmd := findLoopCmd(t)
	// Shared flags (registered via registerSharedFlags)
	sharedFlags := []string{
		"id", "model", "provider", "trust", "plan",
		"timeout", "max-wait", "dry-run", "show-thinking", "verbose",
		"max-failures", "cmd-before", "cmd-after", "stop-when",
		"agent-pre", "agent-before", "agent-after", "agent-post",
		"hook", "hooks-file", "log", "max-cost", "label",
		"allowed-tools", "disallowed-tools", "max-turns", "mcp-config",
		"on-failure", "retries", "agent-cmd", "command",
		"system-prompt", "retry-prompt", "workdir", "channels", "extra",
		"no-config", "no-log",
	}
	// Loop-only flags (registered directly on loopCmd)
	loopOnlyFlags := []string{"delay", "iterations", "resume", "continue", "fuzz"}
	for _, name := range append(sharedFlags, loopOnlyFlags...) {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("loopCmd missing flag --%s", name)
		}
	}
}

func TestLoopCmdHiddenFlags(t *testing.T) {
	cmd := findLoopCmd(t)
	hiddenFlags := []string{"fuzz", "interactive", "show-thinking"}
	for _, name := range hiddenFlags {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("loopCmd missing flag --%s (should exist but be hidden)", name)
			continue
		}
		if !f.Hidden {
			t.Errorf("loopCmd flag --%s should be hidden", name)
		}
	}
}

func TestLoopCmdHelpShowsFlagGroups(t *testing.T) {
	cmd := findLoopCmd(t)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	defer cmd.SetOut(nil)
	if err := cmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	out := buf.String()
	for _, group := range []string{"Loop Control", "Agent Configuration", "Lifecycle Hooks", "Output"} {
		if !strings.Contains(out, group) {
			t.Errorf("loop help missing group %q", group)
		}
	}
}

func TestLoopCmdRunsPromptWithMockRunner(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "done"})

	var stdout, stderr bytes.Buffer
	cfg := Config{
		Content:    "fix the failing tests",
		Iterations: 1,
		Runner:     mock,
		Stdout:     &stdout,
		Stderr:     &stderr,
	}
	if err := Run(cfg); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(mock.Calls))
	}
	if !strings.Contains(mock.Calls[0].Prompt, "fix the failing tests") {
		t.Errorf("prompt missing content: %q", mock.Calls[0].Prompt)
	}
}

func TestRunFunctionStillWorksDirectly(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	var stdout, stderr bytes.Buffer
	cfg := Config{
		Content:    "do something",
		Iterations: 1,
		Runner:     mock,
		Stdout:     &stdout,
		Stderr:     &stderr,
	}
	if err := Run(cfg); err != nil {
		t.Errorf("Run() error: %v", err)
	}
}

func TestBareJuggleShowsHelp(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(nil)
	rootCmd.SetArgs([]string{})
	err := rootCmd.Execute()
	if err != nil {
		t.Errorf("bare juggle should not error, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "juggle loop") {
		t.Errorf("bare juggle help should mention 'juggle loop', got:\n%s", out)
	}
}

func TestBareJuggleWithArgsErrors(t *testing.T) {
	rootCmd.SetArgs([]string{"some", "prompt"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("bare juggle with args should error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "juggle loop") || !strings.Contains(msg, "juggle queue") {
		t.Errorf("error should suggest loop/queue, got: %s", msg)
	}
}

func TestLoopCmdExtraShorthand(t *testing.T) {
	cmd := findLoopCmd(t)
	if cmd.Flags().ShorthandLookup("X") == nil {
		t.Error("loopCmd missing -X shorthand for --extra")
	}
}

func TestRootCmdExtraShorthand(t *testing.T) {
	if rootCmd.PersistentFlags().ShorthandLookup("X") == nil {
		t.Error("rootCmd.PersistentFlags missing -X shorthand for --extra")
	}
}
