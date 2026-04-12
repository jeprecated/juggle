package provider

import (
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

func TestCommandForSpec_OverrideReplacesBinary(t *testing.T) {
	spec := commandSpec{
		Binary:          "claude",
		Args:            []string{"-p", "test"},
		CommandOverride: "echo",
	}
	cmd := commandForSpec(spec)
	name := cmd.Args[0]
	if name != "echo" {
		t.Errorf("expected overridden binary echo, got %q", name)
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
