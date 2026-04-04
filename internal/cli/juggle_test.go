package cli

import (
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	t.Run("includes content and footer", func(t *testing.T) {
		got := BuildPrompt("fix the tests", 1, 10)
		if !strings.Contains(got, "fix the tests") {
			t.Error("missing content")
		}
		if !strings.Contains(got, "iteration 1 of 10") {
			t.Error("missing footer")
		}
	})

	t.Run("unlimited iterations", func(t *testing.T) {
		got := BuildPrompt("content", 3, 0)
		if !strings.Contains(got, "iteration 3 of unlimited") {
			t.Error("expected 'unlimited' for max=0")
		}
	})

	t.Run("content separated from footer by ---", func(t *testing.T) {
		got := BuildPrompt("my content", 1, 1)
		if !strings.Contains(got, "my content\n\n---\n") {
			t.Error("expected content separated from footer by blank line and ---")
		}
	})
}

func TestBuildWatchPrompt(t *testing.T) {
	t.Run("includes task, content, and footer", func(t *testing.T) {
		got := BuildWatchPrompt("task data", "instructions", "task-001.md", 2, 5)
		if !strings.Contains(got, "<task>\ntask data\n</task>") {
			t.Error("missing task section")
		}
		if !strings.Contains(got, "instructions") {
			t.Error("missing content")
		}
		if !strings.Contains(got, "iteration 2 of 5") {
			t.Error("missing iteration in footer")
		}
		if !strings.Contains(got, "processing task-001.md") {
			t.Error("missing filename in footer")
		}
	})
}
