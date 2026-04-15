package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Note: these tests are not parallel-safe because they mutate rootCmd's output writer.
func TestHelpFlagGroupHeadings(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(os.Stdout)
	if err := rootCmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	out := buf.String()

	groups := []string{
		"Loop Control",
		"Agent Configuration",
		"Lifecycle Hooks",
		"Output",
	}
	for _, g := range groups {
		if !strings.Contains(out, g) {
			t.Errorf("root help output missing group heading %q", g)
		}
	}
}


func TestGroupedHelpWithColorTrueHasANSIInHeadings(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(os.Stdout)
	groupedHelpWithColor(rootCmd, nil, true)
	out := buf.String()
	if !strings.Contains(out, "\033[") {
		t.Error("groupedHelpWithColor(color=true) should contain ANSI escape codes in headings")
	}
	// Heading text must still be present
	if !strings.Contains(out, "Loop Control") {
		t.Error("groupedHelpWithColor(color=true) should still contain group heading text")
	}
}

func TestGroupedHelpWithColorFalseHasNoANSI(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(os.Stdout)
	groupedHelpWithColor(rootCmd, nil, false)
	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Error("groupedHelpWithColor(color=false) should not contain ANSI escape codes")
	}
}

func TestRootHelpListsLoopAndQueueSubcommands(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(os.Stdout)
	if err := rootCmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	out := buf.String()
	for _, sub := range []string{"loop", "queue"} {
		if !strings.Contains(out, sub) {
			t.Errorf("root help should list %q subcommand", sub)
		}
	}
	for _, sub := range []string{"watch", "serve"} {
		if strings.Contains(out, "  "+sub+"  ") {
			t.Errorf("root help should NOT list removed %q subcommand", sub)
		}
	}
}


func TestHelpContainsAllVisibleFlags(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(os.Stdout)
	if err := rootCmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	out := buf.String()

	// One representative flag from each shared group (no watch-specific flags on root)
	flagsWanted := []string{
		"--iterations",
		"--delay",
		"--timeout",
		"--model",
		"--trust",
		"--system-prompt",
		"--cmd-before",
		"--agent-pre",
		"--log",
		"--verbose",
	}
	for _, flag := range flagsWanted {
		if !strings.Contains(out, flag) {
			t.Errorf("root help output missing flag %q", flag)
		}
	}
	// Watch/queue-specific flags should NOT be registered on root command
	for _, name := range []string{"watch", "workers", "dashboard"} {
		if rootCmd.Flags().Lookup(name) != nil {
			t.Errorf("root command should not have flag --%s registered", name)
		}
	}
}
