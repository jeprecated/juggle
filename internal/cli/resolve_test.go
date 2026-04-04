package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveArgs(t *testing.T) {
	t.Run("raw strings pass through", func(t *testing.T) {
		got, err := ResolveArgs([]string{"hello", "world"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
			t.Errorf("got %v, want [hello world]", got)
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
}
