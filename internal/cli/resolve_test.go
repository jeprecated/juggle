package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
)

func TestReadStdin(t *testing.T) {
	t.Run("returns content when not a TTY", func(t *testing.T) {
		got, err := ReadStdin(strings.NewReader("fix the tests"), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "fix the tests" {
			t.Errorf("got %q, want %q", got, "fix the tests")
		}
	})

	t.Run("returns empty when TTY", func(t *testing.T) {
		got, err := ReadStdin(strings.NewReader("ignored"), true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty string for TTY", got)
		}
	})

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		got, err := ReadStdin(strings.NewReader("  hello world  \n"), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "hello world" {
			t.Errorf("got %q, want %q", got, "hello world")
		}
	})

	t.Run("returns empty for blank stdin", func(t *testing.T) {
		got, err := ReadStdin(strings.NewReader("   \n  "), false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("got %q, want empty for blank input", got)
		}
	})

	t.Run("appends after positional args in Run content", func(t *testing.T) {
		mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
		var stderr strings.Builder
		cfg := Config{
			Content:    "arg content\n\nstdin content",
			Iterations: 1,
			Runner:     mock,
			Stderr:     &stderr,
		}
		if err := RunLoop(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		prompt := mock.Calls[0].Prompt
		argIdx := strings.Index(prompt, "arg content")
		stdinIdx := strings.Index(prompt, "stdin content")
		if argIdx < 0 {
			t.Error("prompt missing arg content")
		}
		if stdinIdx < 0 {
			t.Error("prompt missing stdin content")
		}
		if argIdx > stdinIdx {
			t.Error("expected arg content before stdin content")
		}
	})
}

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

	t.Run("@alias resolves to file that declares that alias", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "subagent.md"), []byte("---\naliases: [subagents]\n---\nSubagent prompt."), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		got, err := ResolveArgs([]string{"@subagents"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0] != "Subagent prompt." {
			t.Errorf("got %q, want alias to resolve to file body", got[0])
		}
	})

	t.Run("alias resolution is case-insensitive", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "subagent.md"), []byte("---\naliases: [subagents]\n---\nSubagent prompt."), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		got, err := ResolveArgs([]string{"@SUBAGENTS"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0] != "Subagent prompt." {
			t.Errorf("got %q, want case-insensitive alias match", got[0])
		}
	})

	t.Run("base name wins over alias on collision", func(t *testing.T) {
		dir := t.TempDir()
		// file named "sub.md" exists, AND "subagent.md" declares alias "sub"
		os.WriteFile(filepath.Join(dir, "sub.md"), []byte("base name content"), 0644)
		os.WriteFile(filepath.Join(dir, "subagent.md"), []byte("---\naliases: [sub]\n---\nAlias target."), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		got, err := ResolveArgs([]string{"@sub"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0] != "base name content" {
			t.Errorf("got %q, want base name to win over alias", got[0])
		}
	})

	t.Run("duplicate alias across two files returns error", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.md"), []byte("---\naliases: [shared]\n---\nA."), 0644)
		os.WriteFile(filepath.Join(dir, "b.md"), []byte("---\naliases: [shared]\n---\nB."), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		_, err := ResolveArgs([]string{"@shared"})
		if err == nil {
			t.Fatal("expected error for duplicate alias")
		}
		if !strings.Contains(err.Error(), "a.md") || !strings.Contains(err.Error(), "b.md") {
			t.Errorf("error should name both files, got: %v", err)
		}
	})

	t.Run("frontmatter is stripped from JUGGLE_PROMPTS files", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "tdd.md"), []byte("---\naliases: []\n---\nTest-driven content."), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		got, err := ResolveArgs([]string{"@tdd"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(got[0], "---") {
			t.Errorf("got %q, frontmatter should be stripped", got[0])
		}
		if got[0] != "Test-driven content." {
			t.Errorf("got %q, want body only", got[0])
		}
	})

	t.Run("literal file path does not strip frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		content := "---\naliases: [foo]\n---\nBody."
		path := filepath.Join(dir, "file.md")
		os.WriteFile(path, []byte(content), 0644)

		got, err := ResolveArgs([]string{"@" + path})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0] != content {
			t.Errorf("got %q, want literal file untouched", got[0])
		}
	})

	t.Run("file without frontmatter in JUGGLE_PROMPTS works unchanged", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "plain.md"), []byte("plain content, no frontmatter"), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		got, err := ResolveArgs([]string{"@plain"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0] != "plain content, no frontmatter" {
			t.Errorf("got %q, want unchanged content", got[0])
		}
	})
}
