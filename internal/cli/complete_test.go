package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteArgs(t *testing.T) {
	t.Run("non-@ arg returns no completions", func(t *testing.T) {
		completions, directive := completeArgs(nil, nil, "hello")
		if len(completions) != 0 {
			t.Errorf("expected no completions, got %v", completions)
		}
		if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
			t.Error("expected NoFileComp directive")
		}
	})

	t.Run("empty JUGGLE_PROMPTS returns no completions", func(t *testing.T) {
		t.Setenv("JUGGLE_PROMPTS", "")
		completions, _ := completeArgs(nil, nil, "@TD")
		if len(completions) != 0 {
			t.Errorf("expected no completions, got %v", completions)
		}
	})

	t.Run("@ prefix lists matching files", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "TDD.md"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "TDD-strict.md"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "review.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, directive := completeArgs(nil, nil, "@TD")
		if len(completions) != 2 {
			t.Fatalf("expected 2 completions, got %v", completions)
		}
		for _, c := range completions {
			if c != "@TDD" && c != "@TDD-strict" {
				t.Errorf("unexpected completion: %q", c)
			}
		}
		if directive&cobra.ShellCompDirectiveNoSpace == 0 {
			t.Error("expected NoSpace directive")
		}
		if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
			t.Error("expected NoFileComp directive")
		}
	})

	t.Run("case-insensitive matching", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "TDD.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@tdd")
		if len(completions) != 1 || completions[0] != "@TDD" {
			t.Errorf("expected [@TDD], got %v", completions)
		}
	})

	t.Run("just @ lists all files", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.md"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "b.md"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, ".hidden"), []byte(""), 0644)
		os.Mkdir(filepath.Join(dir, "subdir"), 0755)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@")
		if len(completions) != 2 {
			t.Errorf("expected 2 completions (skip hidden and dirs), got %v", completions)
		}
	})

	t.Run("completions use base name without extension", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "review.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@rev")
		if len(completions) != 1 || completions[0] != "@review" {
			t.Errorf("expected [@review], got %v", completions)
		}
	})

	t.Run("deduplicates when file and file.md both exist", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "TDD"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "TDD.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@TDD")
		if len(completions) != 1 {
			t.Errorf("expected 1 completion (deduped), got %v", completions)
		}
	})
}
