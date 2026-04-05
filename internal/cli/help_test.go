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
