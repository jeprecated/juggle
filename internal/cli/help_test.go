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

	// Root command shows shared flag groups; Watch Mode is on the watch subcommand.
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
	if strings.Contains(out, "Watch Mode") {
		t.Error("root help should not show Watch Mode group (it belongs to the watch subcommand)")
	}
}

func TestWatchCmdHelpFlagGroupHeadings(t *testing.T) {
	watchSubCmd, _, err := rootCmd.Find([]string{"watch"})
	if err != nil || watchSubCmd == nil {
		t.Skip("watch subcommand not found")
	}
	var buf bytes.Buffer
	watchSubCmd.SetOut(&buf)
	defer watchSubCmd.SetOut(os.Stdout)
	if err := watchSubCmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	out := buf.String()

	groups := []string{"Watch Mode", "Loop Control", "Agent Configuration"}
	for _, g := range groups {
		if !strings.Contains(out, g) {
			t.Errorf("watch subcommand help missing group heading %q", g)
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
	// Watch-specific flags should NOT appear on root command help
	for _, flag := range []string{"--watch", "--workers", "--dashboard"} {
		if strings.Contains(out, flag) {
			t.Errorf("root help should not show watch-specific flag %q", flag)
		}
	}
}
