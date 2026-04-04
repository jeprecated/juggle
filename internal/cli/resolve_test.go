package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveArgs(t *testing.T) {
	t.Run("quoted strings pass through", func(t *testing.T) {
		got, err := ResolveArgs([]string{"hello world", "do stuff"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got[0] != "hello world" || got[1] != "do stuff" {
			t.Errorf("got %v, want [hello world, do stuff]", got)
		}
	})

	t.Run("bare words pass through as prompt text", func(t *testing.T) {
		got, err := ResolveArgs([]string{"what?"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "what?" {
			t.Errorf("got %v, want [what?]", got)
		}
	})

	t.Run("@file reads contents", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "task.md")
		os.WriteFile(path, []byte("fix the tests"), 0644)

		got, err := ResolveArgs([]string{"@" + path})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "fix the tests" {
			t.Errorf("got %v, want [fix the tests]", got)
		}
	})

	t.Run("missing @file returns error", func(t *testing.T) {
		_, err := ResolveArgs([]string{"@/nonexistent/file.md"})
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("mixed args", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "instructions.md")
		os.WriteFile(path, []byte("be careful"), 0644)

		got, err := ResolveArgs([]string{"do stuff", "@" + path, "also this"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 results, got %d", len(got))
		}
		if got[0] != "do stuff" || got[1] != "be careful" || got[2] != "also this" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("empty args returns empty", func(t *testing.T) {
		got, err := ResolveArgs([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("bare @name resolves from JUGGLE_PROMPTS", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "TDD.md"), []byte("test first"), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		got, err := ResolveArgs([]string{"@TDD"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "test first" {
			t.Errorf("got %v, want [test first]", got)
		}
	})

	t.Run("bare @name with extension resolves from JUGGLE_PROMPTS", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "TDD.md"), []byte("test first"), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		got, err := ResolveArgs([]string{"@TDD.md"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "test first" {
			t.Errorf("got %v, want [test first]", got)
		}
	})

	t.Run("bare @name without .md auto-suffixes", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "review.md"), []byte("review code"), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		got, err := ResolveArgs([]string{"@review"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != "review code" {
			t.Errorf("got %v, want [review code]", got)
		}
	})

	t.Run("literal path wins over JUGGLE_PROMPTS", func(t *testing.T) {
		promptsDir := t.TempDir()
		os.WriteFile(filepath.Join(promptsDir, "task.md"), []byte("from prompts"), 0644)
		t.Setenv("JUGGLE_PROMPTS", promptsDir)

		localDir := t.TempDir()
		localFile := filepath.Join(localDir, "task.md")
		os.WriteFile(localFile, []byte("from local"), 0644)

		got, err := ResolveArgs([]string{"@" + localFile})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0] != "from local" {
			t.Errorf("got %q, want literal path to win", got[0])
		}
	})

	t.Run("path with / does not try JUGGLE_PROMPTS", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("JUGGLE_PROMPTS", dir)

		_, err := ResolveArgs([]string{"@./nonexistent/file.md"})
		if err == nil {
			t.Fatal("expected error for path with /")
		}
	})

	t.Run("bare @name without JUGGLE_PROMPTS gives original error", func(t *testing.T) {
		t.Setenv("JUGGLE_PROMPTS", "")

		_, err := ResolveArgs([]string{"@nonexistent"})
		if err == nil {
			t.Fatal("expected error when JUGGLE_PROMPTS unset")
		}
	})

	t.Run("exact name match preferred over .md suffix", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "PLAN"), []byte("exact match"), 0644)
		os.WriteFile(filepath.Join(dir, "PLAN.md"), []byte("md match"), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		got, err := ResolveArgs([]string{"@PLAN"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0] != "exact match" {
			t.Errorf("got %q, want exact name to win over .md suffix", got[0])
		}
	})
}
