package provider

import (
	"os"
	"strings"
	"testing"
)

func TestCommandForSpec_UsesDefaultBinary(t *testing.T) {
	spec := commandSpec{Binary: "claude", Args: []string{"-p", "test"}}
	cmd := commandForSpec(spec)
	if cmd.Path != "" {
		// Just verify the binary name is resolved (may be absolute path)
		name := cmd.Args[0]
		if name != "claude" {
			t.Errorf("expected binary claude, got %q", name)
		}
	}
}

func TestCommandForSpec_OverrideUsesShell(t *testing.T) {
	spec := commandSpec{
		Binary:          "claude",
		Args:            []string{"-p", "test"},
		CommandOverride: "cz",
	}
	cmd := commandForSpec(spec)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	// Should invoke the user's shell
	if cmd.Args[0] != shell {
		t.Errorf("expected shell %q, got %q", shell, cmd.Args[0])
	}
	// Should pass -c (non-interactive to avoid TTY hangs)
	if cmd.Args[1] != "-c" {
		t.Errorf("expected -c flag, got %q", cmd.Args[1])
	}
	// The command string should contain the override and original args
	cmdStr := cmd.Args[2]
	if !strings.Contains(cmdStr, "cz -p test") {
		t.Errorf("expected command string to contain 'cz -p test', got %q", cmdStr)
	}
}

func TestCommandForSpec_OverrideEmptyUsesDefault(t *testing.T) {
	spec := commandSpec{
		Binary:          "claude",
		Args:            []string{"-p", "test"},
		CommandOverride: "",
	}
	cmd := commandForSpec(spec)
	name := cmd.Args[0]
	if name != "claude" {
		t.Errorf("expected default binary claude, got %q", name)
	}
}

func TestCommandForSpec_OverrideQuotesSpecialArgs(t *testing.T) {
	spec := commandSpec{
		Binary:          "claude",
		Args:            []string{"-p", "hello world", "--flag"},
		CommandOverride: "my-agent",
	}
	cmd := commandForSpec(spec)
	cmdStr := cmd.Args[2]
	if !strings.Contains(cmdStr, "my-agent -p 'hello world' --flag") {
		t.Errorf("expected command to contain properly quoted args, got %q", cmdStr)
	}
}

func TestClaudeHeadlessSpec_PropagatesCommandOverride(t *testing.T) {
	opts := RunOptions{
		Prompt:          "test",
		CommandOverride: "cc",
	}
	spec := claudeHeadlessSpec(opts)
	if spec.CommandOverride != "cc" {
		t.Errorf("expected CommandOverride='cc', got %q", spec.CommandOverride)
	}
	if spec.Binary != "claude" {
		t.Errorf("expected Binary='claude', got %q", spec.Binary)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"simple", "simple"},
		{"hello world", "'hello world'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
		{"no-special", "no-special"},
		{"has $var", "'has $var'"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := shellQuote(tc.input)
			if got != tc.expect {
				t.Errorf("shellQuote(%q) = %q, want %q", tc.input, got, tc.expect)
			}
		})
	}
}

func TestShellBasename(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"/bin/zsh", "zsh"},
		{"/usr/bin/zsh", "zsh"},
		{"/nix/store/abc123/bin/zsh-5.9", "zsh"},
		{"/bin/bash", "bash"},
		{"/usr/local/bin/fish", "fish"},
		{"zsh", "zsh"},
		{"/bin/sh", "sh"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := shellBasename(tc.input)
			if got != tc.expect {
				t.Errorf("shellBasename(%q) = %q, want %q", tc.input, got, tc.expect)
			}
		})
	}
}

func TestAppendFlag(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		flag   string
		value  string
		expect []string
	}{
		{"flag with value", []string{}, "--model", "sonnet", []string{"--model", "sonnet"}},
		{"flag without value", []string{}, "--trust", "", []string{"--trust"}},
		{"empty flag skipped", []string{"a"}, "", "", []string{"a"}},
		{"appended to existing", []string{"-p"}, "--model", "opus", []string{"-p", "--model", "opus"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := appendFlag(tc.args, tc.flag, tc.value)
			if len(got) != len(tc.expect) {
				t.Fatalf("expected %v, got %v", tc.expect, got)
			}
			for i := range got {
				if got[i] != tc.expect[i] {
					t.Errorf("index %d: expected %q, got %q", i, tc.expect[i], got[i])
				}
			}
		})
	}
}
