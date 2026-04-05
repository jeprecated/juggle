package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestFindPromptDirs(t *testing.T) {
	t.Run("finds dir named prompts at root", func(t *testing.T) {
		root := t.TempDir()
		os.Mkdir(filepath.Join(root, "prompts"), 0755)
		dirs := findPromptDirs(root)
		if len(dirs) != 1 || dirs[0] != filepath.Join(root, "prompts") {
			t.Errorf("expected [prompts dir], got %v", dirs)
		}
	})

	t.Run("finds dir containing prompts in name", func(t *testing.T) {
		root := t.TempDir()
		os.Mkdir(filepath.Join(root, "juggle-prompts"), 0755)
		dirs := findPromptDirs(root)
		if len(dirs) != 1 {
			t.Fatalf("expected 1 dir, got %v", dirs)
		}
	})

	t.Run("finds nested prompts dir", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, "docs", "prompts"), 0755)
		dirs := findPromptDirs(root)
		if len(dirs) != 1 || dirs[0] != filepath.Join(root, "docs", "prompts") {
			t.Errorf("expected [docs/prompts], got %v", dirs)
		}
	})

	t.Run("does not find non-prompts dirs", func(t *testing.T) {
		root := t.TempDir()
		os.Mkdir(filepath.Join(root, "plans"), 0755)
		os.MkdirAll(filepath.Join(root, "docs", "plans"), 0755)
		dirs := findPromptDirs(root)
		if len(dirs) != 0 {
			t.Errorf("expected no dirs, got %v", dirs)
		}
	})

	t.Run("skips hidden dirs", func(t *testing.T) {
		root := t.TempDir()
		os.Mkdir(filepath.Join(root, ".hidden-prompts"), 0755)
		dirs := findPromptDirs(root)
		if len(dirs) != 0 {
			t.Errorf("expected no dirs (hidden skipped), got %v", dirs)
		}
	})

	t.Run("does not descend into hidden dirs", func(t *testing.T) {
		root := t.TempDir()
		os.MkdirAll(filepath.Join(root, ".hidden", "prompts"), 0755)
		dirs := findPromptDirs(root)
		if len(dirs) != 0 {
			t.Errorf("expected no dirs (inside hidden dir), got %v", dirs)
		}
	})

	t.Run("finds multiple prompts dirs", func(t *testing.T) {
		root := t.TempDir()
		os.Mkdir(filepath.Join(root, "prompts"), 0755)
		os.MkdirAll(filepath.Join(root, "docs", "prompts"), 0755)
		dirs := findPromptDirs(root)
		if len(dirs) != 2 {
			t.Errorf("expected 2 dirs, got %v", dirs)
		}
	})
}

func TestCompleteArgsWithCwdPrompts(t *testing.T) {
	t.Run("finds md files in cwd prompts subdir", func(t *testing.T) {
		root := t.TempDir()
		os.Mkdir(filepath.Join(root, "prompts"), 0755)
		os.WriteFile(filepath.Join(root, "prompts", "mytemplate.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", "")

		orig, _ := os.Getwd()
		os.Chdir(root)
		defer os.Chdir(orig)

		completions, _ := completeArgs(nil, nil, "@my")
		if len(completions) != 1 || completions[0] != "@mytemplate" {
			t.Errorf("expected [@mytemplate], got %v", completions)
		}
	})

	t.Run("deduplicates across JUGGLE_PROMPTS and cwd prompts", func(t *testing.T) {
		root := t.TempDir()
		promptsEnv := t.TempDir()
		os.Mkdir(filepath.Join(root, "prompts"), 0755)
		os.WriteFile(filepath.Join(root, "prompts", "shared.md"), []byte(""), 0644)
		os.WriteFile(filepath.Join(promptsEnv, "shared.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", promptsEnv)

		orig, _ := os.Getwd()
		os.Chdir(root)
		defer os.Chdir(orig)

		completions, _ := completeArgs(nil, nil, "@")
		count := 0
		for _, c := range completions {
			if c == "@shared" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected @shared exactly once, got %v", completions)
		}
	})

	t.Run("only md files from cwd prompts dirs", func(t *testing.T) {
		root := t.TempDir()
		os.Mkdir(filepath.Join(root, "prompts"), 0755)
		os.WriteFile(filepath.Join(root, "prompts", "good.md"), []byte(""), 0644)
		os.WriteFile(filepath.Join(root, "prompts", "skip.txt"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", "")

		orig, _ := os.Getwd()
		os.Chdir(root)
		defer os.Chdir(orig)

		completions, _ := completeArgs(nil, nil, "@")
		if len(completions) != 1 || completions[0] != "@good" {
			t.Errorf("expected [@good] only, got %v", completions)
		}
	})
}

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

	t.Run("alias appears as completion with source hint", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "subagent.md"), []byte("---\naliases: [subagents, sub]\n---\nBody."), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@sub")
		// Should contain: @subagent, @subagents (→ subagent), @sub (→ subagent)
		var hasBase, hasAliasSubagents, hasAliasSub bool
		for _, c := range completions {
			switch c {
			case "@subagent":
				hasBase = true
			case "@subagents\t(→ subagent)":
				hasAliasSubagents = true
			case "@sub\t(→ subagent)":
				hasAliasSub = true
			}
		}
		if !hasBase {
			t.Errorf("expected @subagent base completion, got %v", completions)
		}
		if !hasAliasSubagents {
			t.Errorf("expected @subagents (→ subagent) alias completion, got %v", completions)
		}
		if !hasAliasSub {
			t.Errorf("expected @sub (→ subagent) alias completion, got %v", completions)
		}
	})

	t.Run("alias completion is case-insensitive", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "myfile.md"), []byte("---\naliases: [MyAlias]\n---\nBody."), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@myal")
		found := false
		for _, c := range completions {
			if c == "@MyAlias\t(→ myfile)" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected @MyAlias (→ myfile) completion, got %v", completions)
		}
	})

	t.Run("alias not shown if base name already matches (base name wins)", func(t *testing.T) {
		dir := t.TempDir()
		// sub.md exists, subagent.md declares alias "sub"
		os.WriteFile(filepath.Join(dir, "sub.md"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "subagent.md"), []byte("---\naliases: [sub]\n---\nBody."), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@sub")
		for _, c := range completions {
			if c == "@sub\t(→ subagent)" {
				t.Errorf("alias completion should not appear when base name already provides @sub, got %v", completions)
			}
		}
	})
}

func TestCompleteArgsRecursive(t *testing.T) {
	t.Run("@ lists nested files with relative path prefix", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "workflows"), 0755)
		os.WriteFile(filepath.Join(dir, "workflows", "fix.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@")
		found := false
		for _, c := range completions {
			if c == "@workflows/fix" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected @workflows/fix in completions, got %v", completions)
		}
	})

	t.Run("bare partial matches nested file by base name", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "workflows"), 0755)
		os.WriteFile(filepath.Join(dir, "workflows", "fix.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@fix")
		found := false
		for _, c := range completions {
			if c == "@workflows/fix" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected @workflows/fix when typing @fix, got %v", completions)
		}
	})

	t.Run("partial with path prefix matches files in that subdir", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "workflows"), 0755)
		os.WriteFile(filepath.Join(dir, "workflows", "fix.md"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "workflows", "review.md"), []byte(""), 0644)
		os.WriteFile(filepath.Join(dir, "root.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@workflows/")
		for _, c := range completions {
			if c == "@root" {
				t.Errorf("@root should not appear for @workflows/ prefix, got %v", completions)
			}
		}
		var hasWFFix, hasWFReview bool
		for _, c := range completions {
			if c == "@workflows/fix" {
				hasWFFix = true
			}
			if c == "@workflows/review" {
				hasWFReview = true
			}
		}
		if !hasWFFix || !hasWFReview {
			t.Errorf("expected @workflows/fix and @workflows/review, got %v", completions)
		}
	})

	t.Run("hidden dirs skipped in recursive completion", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, ".hidden"), 0755)
		os.WriteFile(filepath.Join(dir, ".hidden", "secret.md"), []byte(""), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@")
		for _, c := range completions {
			if strings.Contains(c, "secret") || strings.Contains(c, ".hidden") {
				t.Errorf("hidden dir file should not appear, got %v", completions)
			}
		}
	})

	t.Run("alias in nested file appears in completions", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "workflows"), 0755)
		os.WriteFile(filepath.Join(dir, "workflows", "fix.md"), []byte("---\naliases: [wf]\n---\nBody."), 0644)
		t.Setenv("JUGGLE_PROMPTS", dir)

		completions, _ := completeArgs(nil, nil, "@wf")
		found := false
		for _, c := range completions {
			if c == "@wf\t(→ workflows/fix)" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected @wf (→ workflows/fix) alias completion, got %v", completions)
		}
	})
}
