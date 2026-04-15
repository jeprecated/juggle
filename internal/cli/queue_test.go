package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// findQueueCmd returns the queue subcommand or fatals.
func findQueueCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd, _, err := rootCmd.Find([]string{"queue"})
	if err != nil || cmd == nil || cmd.Name() != "queue" {
		t.Fatal("queue subcommand not found on rootCmd")
	}
	return cmd
}

func TestQueueCmdExistsOnRootCmd(t *testing.T) {
	findQueueCmd(t) // fails if not found
}

func TestQueueCmdUse(t *testing.T) {
	cmd := findQueueCmd(t)
	if !strings.HasPrefix(cmd.Use, "queue") {
		t.Errorf("queueCmd.Use should start with 'queue', got %q", cmd.Use)
	}
}

func TestQueueCmdHasWatchFlag(t *testing.T) {
	cmd := findQueueCmd(t)
	if cmd.Flags().Lookup("watch") == nil {
		t.Fatal("queueCmd missing --watch flag")
	}
}

func TestQueueCmdHasOnTouchFlag(t *testing.T) {
	cmd := findQueueCmd(t)
	if cmd.Flags().Lookup("on-touch") == nil {
		t.Fatal("queueCmd missing --on-touch flag")
	}
}

func TestQueueCmdHasEveryFlag(t *testing.T) {
	cmd := findQueueCmd(t)
	if cmd.Flags().Lookup("every") == nil {
		t.Fatal("queueCmd missing --every flag")
	}
}

func TestQueueCmdHasNowFlag(t *testing.T) {
	cmd := findQueueCmd(t)
	if cmd.Flags().Lookup("now") == nil {
		t.Fatal("queueCmd missing --now flag")
	}
}

func TestQueueCmdHasWorkersFlag(t *testing.T) {
	cmd := findQueueCmd(t)
	if cmd.Flags().Lookup("workers") == nil {
		t.Fatal("queueCmd missing --workers flag")
	}
}

func TestQueueCmdHasDashboardFlag(t *testing.T) {
	cmd := findQueueCmd(t)
	if cmd.Flags().Lookup("dashboard") == nil {
		t.Fatal("queueCmd missing --dashboard flag")
	}
}

func TestQueueCmdHasServeFlag(t *testing.T) {
	cmd := findQueueCmd(t)
	if cmd.Flags().Lookup("serve") == nil {
		t.Fatal("queueCmd missing --serve flag")
	}
}

// Queue hides loop-only flags so they don't appear in --help.
func TestQueueCmdHidesLoopOnlyFlags(t *testing.T) {
	cmd := findQueueCmd(t)
	for _, name := range []string{"delay", "iterations", "resume", "continue"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			continue
		}
		if !f.Hidden {
			t.Errorf("queueCmd flag --%s should be hidden", name)
		}
	}
}

// TestQueueCmdHasSharedFlags verifies all flags from registerSharedFlags are present.
func TestQueueCmdHasSharedFlags(t *testing.T) {
	cmd := findQueueCmd(t)
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
	for _, name := range sharedFlags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("queueCmd missing shared flag --%s", name)
		}
	}
}

func TestQueueCmdHelpShowsQueueModeGroup(t *testing.T) {
	cmd := findQueueCmd(t)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	defer cmd.SetOut(nil)
	if err := cmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Queue Mode") {
		t.Errorf("queue help missing 'Queue Mode' group, got:\n%s", out)
	}
}

func TestQueueCmdHelpShowsStandardGroups(t *testing.T) {
	cmd := findQueueCmd(t)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	defer cmd.SetOut(nil)
	if err := cmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	out := buf.String()
	for _, group := range []string{"Agent Configuration", "Lifecycle Hooks", "Output"} {
		if !strings.Contains(out, group) {
			t.Errorf("queue help missing group %q", group)
		}
	}
}

func TestQueueCmdRequiresTrigger(t *testing.T) {
	savedWatch := queueFlags.watch
	savedEvery := queueFlags.every
	savedID := flags.id
	savedServe := queueFlags.serve
	defer func() {
		queueFlags.watch = savedWatch
		queueFlags.every = savedEvery
		flags.id = savedID
		queueFlags.serve = savedServe
	}()

	queueFlags.watch = nil
	queueFlags.every = 0
	queueFlags.serve = ""
	flags.id = ""

	cmd := findQueueCmd(t)
	err := cmd.RunE(cmd, []string{"do the thing"})
	if err == nil {
		t.Fatal("expected error when no trigger flags set, got nil")
	}
	if !strings.Contains(err.Error(), "queue requires at least one trigger") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestQueueCmdServeRequiresID(t *testing.T) {
	savedServe := queueFlags.serve
	savedWatch := queueFlags.watch
	savedEvery := queueFlags.every
	savedID := flags.id
	defer func() {
		queueFlags.serve = savedServe
		queueFlags.watch = savedWatch
		queueFlags.every = savedEvery
		flags.id = savedID
	}()

	queueFlags.serve = ":8080"
	queueFlags.watch = nil
	queueFlags.every = 0
	flags.id = ""

	cmd := findQueueCmd(t)
	err := cmd.RunE(cmd, []string{"do the thing"})
	if err == nil {
		t.Fatal("expected error when --serve without --id, got nil")
	}
	if !strings.Contains(err.Error(), "--serve requires --id") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestQueueCmdExtraShorthand(t *testing.T) {
	cmd := findQueueCmd(t)
	if cmd.Flags().ShorthandLookup("X") == nil {
		t.Error("queueCmd missing -X shorthand for --extra")
	}
}
