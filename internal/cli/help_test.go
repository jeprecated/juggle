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
		"Watch Mode",
		"Output",
	}
	for _, g := range groups {
		if !strings.Contains(out, g) {
			t.Errorf("help output missing group heading %q", g)
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

	// One representative flag from each group
	flagsWanted := []string{
		"--iterations",
		"--delay",
		"--timeout",
		"--model",
		"--trust",
		"--system-prompt",
		"--cmd-before",
		"--agent-pre",
		"--watch",
		"--workers",
		"--log",
		"--verbose",
	}
	for _, flag := range flagsWanted {
		if !strings.Contains(out, flag) {
			t.Errorf("help output missing flag %q", flag)
		}
	}
}
