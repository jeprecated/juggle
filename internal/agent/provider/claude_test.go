package provider

import (
	"testing"
)

func TestClaudeHeadlessArgs_Continue(t *testing.T) {
	opts := RunOptions{Continue: true}
	args := claudeHeadlessArgs(opts)

	found := false
	for _, a := range args {
		if a == "--continue" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --continue in args when opts.Continue is true")
	}
}

func TestClaudeHeadlessArgs_NoContinue(t *testing.T) {
	opts := RunOptions{Continue: false}
	args := claudeHeadlessArgs(opts)

	for _, a := range args {
		if a == "--continue" {
			t.Error("did not expect --continue in args when opts.Continue is false")
		}
	}
}
