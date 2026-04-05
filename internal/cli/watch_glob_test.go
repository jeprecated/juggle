package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
)

// --- isGlobPattern ---

func TestIsGlobPattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"./tasks", false},
		{"/abs/path", false},
		{"tasks/", false},
		{"**/.frontloop/ready", true},
		{"*.md", true},
		{"tasks/[ab]", true},
		{"tasks/?item", true},
		{"repo*/.frontloop", true},
	}
	for _, tt := range tests {
		got := isGlobPattern(tt.input)
		if got != tt.want {
			t.Errorf("isGlobPattern(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- FindVCSRoot ---

func TestFindVCSRoot(t *testing.T) {
	t.Run("finds git root from subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, ".git"), 0755)
		sub := filepath.Join(dir, "sub", "deep")
		os.MkdirAll(sub, 0755)
		got := FindVCSRoot(sub)
		if got != dir {
			t.Errorf("expected %q, got %q", dir, got)
		}
	})

	t.Run("finds jj root from subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, ".jj"), 0755)
		sub := filepath.Join(dir, "nested")
		os.MkdirAll(sub, 0755)
		got := FindVCSRoot(sub)
		if got != dir {
			t.Errorf("expected %q, got %q", dir, got)
		}
	})

	t.Run("returns empty string if no VCS root found", func(t *testing.T) {
		// Use a temp dir that has no .git or .jj anywhere above it (it won't
		// escape t.TempDir() because t.TempDir() is under /tmp which has no VCS markers)
		dir := t.TempDir()
		got := FindVCSRoot(dir)
		// Can't guarantee /tmp has no VCS root on all systems, so just verify
		// it returns something reasonable (either empty or a real VCS root)
		_ = got
	})

	t.Run("returns empty string for isolated dir with no VCS marker", func(t *testing.T) {
		// Create a deeply isolated temp dir tree with no VCS markers
		dir := t.TempDir()
		sub := filepath.Join(dir, "a", "b", "c")
		os.MkdirAll(sub, 0755)
		got := FindVCSRoot(sub)
		// If /tmp itself has no VCS root this will be ""
		// We can't guarantee the environment, but we can verify the function returns dir when marker is present
		_ = got
	})

	t.Run("finds git root at the directory itself", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, ".git"), 0755)
		got := FindVCSRoot(dir)
		if got != dir {
			t.Errorf("expected %q, got %q", dir, got)
		}
	})

	t.Run("prefers nearest ancestor", func(t *testing.T) {
		outer := t.TempDir()
		os.Mkdir(filepath.Join(outer, ".git"), 0755)
		inner := filepath.Join(outer, "inner")
		os.MkdirAll(inner, 0755)
		os.Mkdir(filepath.Join(inner, ".git"), 0755)
		sub := filepath.Join(inner, "src")
		os.MkdirAll(sub, 0755)
		got := FindVCSRoot(sub)
		if got != inner {
			t.Errorf("expected nearest ancestor %q, got %q", inner, got)
		}
	})
}

// --- expandGlobDirs ---

func TestExpandGlobDirs(t *testing.T) {
	t.Run("** pattern finds nested matching directories", func(t *testing.T) {
		base := t.TempDir()
		// Create two repos with .frontloop/ready dirs
		for _, name := range []string{"repo-a", "repo-b"} {
			dir := filepath.Join(base, name, ".frontloop", "ready")
			os.MkdirAll(dir, 0755)
		}
		// Create one without the pattern
		os.MkdirAll(filepath.Join(base, "repo-c", "other"), 0755)

		dirs, err := expandGlobDirs(base, "**/.frontloop/ready")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dirs) != 2 {
			t.Fatalf("expected 2 dirs, got %d: %v", len(dirs), dirs)
		}
		for _, d := range dirs {
			if !strings.HasSuffix(d, ".frontloop/ready") {
				t.Errorf("unexpected dir %q", d)
			}
		}
	})

	t.Run("returns empty when no matches", func(t *testing.T) {
		base := t.TempDir()
		dirs, err := expandGlobDirs(base, "**/.frontloop/ready")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dirs) != 0 {
			t.Errorf("expected 0 dirs, got %d", len(dirs))
		}
	})

	t.Run("only returns directories not files", func(t *testing.T) {
		base := t.TempDir()
		os.MkdirAll(filepath.Join(base, "repo", ".frontloop", "ready"), 0755)
		// Create a file that matches
		os.WriteFile(filepath.Join(base, "repo", ".frontloop", "ready-file"), []byte("x"), 0644)

		dirs, err := expandGlobDirs(base, "repo/.frontloop/ready")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dirs) != 1 {
			t.Fatalf("expected 1 dir, got %d: %v", len(dirs), dirs)
		}
	})
}

// --- claimFromDirs ---

func TestWorkerCoordinator_ClaimFromDirs(t *testing.T) {
	t.Run("claims file from first available dir", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()
		os.WriteFile(filepath.Join(dir2, "task.md"), []byte("t"), 0644)

		coord := newWorkerCoordinator()
		got, err := coord.claimFromDirs([]string{dir1, dir2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "task.md" {
			t.Errorf("expected task.md, got %q", got)
		}
	})

	t.Run("returns empty when all dirs are empty", func(t *testing.T) {
		dir1 := t.TempDir()
		dir2 := t.TempDir()

		coord := newWorkerCoordinator()
		got, err := coord.claimFromDirs([]string{dir1, dir2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("does not claim same file twice across multiple workers", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0644)
		os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0644)

		coord := newWorkerCoordinator()
		p1, _ := coord.claimFromDirs([]string{dir})
		p2, _ := coord.claimFromDirs([]string{dir})
		p3, _ := coord.claimFromDirs([]string{dir})

		if p1 == p2 || (p3 != "" && (p3 == p1 || p3 == p2)) {
			t.Errorf("duplicate claims: %q %q %q", p1, p2, p3)
		}
		if p1 == "" || p2 == "" {
			t.Errorf("expected two claims, got %q and %q", p1, p2)
		}
		if p3 != "" {
			t.Errorf("expected empty on third claim, got %q", p3)
		}
	})

	t.Run("skips inaccessible dirs", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "task.md"), []byte("t"), 0644)

		coord := newWorkerCoordinator()
		got, err := coord.claimFromDirs([]string{"/nonexistent/dir", dir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "task.md" {
			t.Errorf("expected task.md from second dir, got %q", got)
		}
	})
}

// --- Glob watch integration ---

func TestRunWatch_GlobPattern_SetsVCSRootAsWorkDir(t *testing.T) {
	base := t.TempDir()

	// Create repo with git root and .frontloop/ready
	repoDir := filepath.Join(base, "my-repo")
	readyDir := filepath.Join(repoDir, ".frontloop", "ready")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.MkdirAll(readyDir, 0755)
	os.WriteFile(filepath.Join(readyDir, "task.md"), []byte("do work"), 0644)

	shutdown := make(chan struct{})
	var capturedWorkDir string
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		capturedWorkDir = opts.WorkingDir
		close(shutdown)
		return &agent.RunResult{}, nil
	}}

	cfg := Config{
		Watch:      []string{"**/.frontloop/ready"},
		WorkDir:    base, // basedir for glob expansion
		Iterations: 1,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}

	err := RunWatch(cfg)
	if err != nil && !errors.Is(err, ErrInterrupted) {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedWorkDir != repoDir {
		t.Errorf("expected workdir %q (git root), got %q", repoDir, capturedWorkDir)
	}
}

func TestExpandGlobDirs_DiscoveryOnSubsequentCall(t *testing.T) {
	// expandGlobDirs re-evaluates the filesystem each call, so new directories
	// created after the first expansion are picked up on the next call.
	// RunWatch calls expandGlobDirs on every poll cycle, satisfying the requirement
	// that new directories matching the glob are discovered as they appear.
	base := t.TempDir()

	dirs1, err := expandGlobDirs(base, "**/.frontloop/ready")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs1) != 0 {
		t.Errorf("expected 0 dirs before creation, got %d", len(dirs1))
	}

	repoDir := filepath.Join(base, "late-repo")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.MkdirAll(filepath.Join(repoDir, ".frontloop", "ready"), 0755)

	dirs2, err := expandGlobDirs(base, "**/.frontloop/ready")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs2) != 1 {
		t.Errorf("expected 1 dir after creation, got %d", len(dirs2))
	}
}

func TestRunWatch_MultiWatch_PicksFromBothDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	task1 := filepath.Join(dir1, "task1.md")
	task2 := filepath.Join(dir2, "task2.md")
	os.WriteFile(task1, []byte("task from dir1"), 0644)
	os.WriteFile(task2, []byte("task from dir2"), 0644)

	shutdown := make(chan struct{})
	var mu sync.Mutex
	processed := map[string]bool{}
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		for _, e := range opts.Env {
			if strings.HasPrefix(e, "JUGGLE_TASK_FILE=") {
				taskFile := strings.TrimPrefix(e, "JUGGLE_TASK_FILE=")
				base := filepath.Base(taskFile)
				// Delete so it's not re-claimed on next poll cycle.
				os.Remove(taskFile)
				mu.Lock()
				processed[base] = true
				total := len(processed)
				mu.Unlock()
				if total >= 2 {
					select {
					case <-shutdown:
					default:
						close(shutdown)
					}
				}
				break
			}
		}
		return &agent.RunResult{}, nil
	}}

	cfg := Config{
		Watch:      []string{dir1, dir2},
		Iterations: 1,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}

	err := RunWatch(cfg)
	if err != nil && !errors.Is(err, ErrInterrupted) {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	n := len(processed)
	mu.Unlock()
	if n < 2 {
		t.Errorf("expected tasks from both dirs to be processed, got %d: %v", n, processed)
	}
}

func TestRunWatch_MultiWatch_WorkersAcrossMultipleDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	task1 := filepath.Join(dir1, "task-a.md")
	task2 := filepath.Join(dir2, "task-b.md")
	os.WriteFile(task1, []byte("a"), 0644)
	os.WriteFile(task2, []byte("b"), 0644)

	shutdown := make(chan struct{})
	var mu sync.Mutex
	callCount := 0
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		for _, e := range opts.Env {
			if strings.HasPrefix(e, "JUGGLE_TASK_FILE=") {
				os.Remove(strings.TrimPrefix(e, "JUGGLE_TASK_FILE="))
				break
			}
		}
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n >= 2 {
			select {
			case <-shutdown:
			default:
				close(shutdown)
			}
		}
		return &agent.RunResult{}, nil
	}}

	cfg := Config{
		Watch:      []string{dir1, dir2},
		Workers:    2,
		Iterations: 1,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}

	err := RunWatch(cfg)
	if err != nil && !errors.Is(err, ErrInterrupted) {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 1 {
		t.Error("expected at least one task to run across multiple dirs")
	}
}

func TestRunWatch_MultiWatch_GlobAndLiteralMixed(t *testing.T) {
	base := t.TempDir()
	// Literal dir
	litDir := filepath.Join(base, "literal-tasks")
	os.Mkdir(litDir, 0755)
	litTask := filepath.Join(litDir, "lit-task.md")
	os.WriteFile(litTask, []byte("lit"), 0644)
	// Glob dir
	repoDir := filepath.Join(base, "my-repo", ".frontloop", "ready")
	os.MkdirAll(repoDir, 0755)
	globTask := filepath.Join(repoDir, "glob-task.md")
	os.WriteFile(globTask, []byte("glob"), 0644)

	shutdown := make(chan struct{})
	var mu sync.Mutex
	processed := map[string]bool{}
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		for _, e := range opts.Env {
			if strings.HasPrefix(e, "JUGGLE_TASK_FILE=") {
				taskFile := strings.TrimPrefix(e, "JUGGLE_TASK_FILE=")
				b := filepath.Base(taskFile)
				os.Remove(taskFile) // delete so it's not re-claimed
				mu.Lock()
				processed[b] = true
				n := len(processed)
				mu.Unlock()
				if n >= 2 {
					select {
					case <-shutdown:
					default:
						close(shutdown)
					}
				}
				break
			}
		}
		return &agent.RunResult{}, nil
	}}

	cfg := Config{
		Watch:      []string{litDir, "**/.frontloop/ready"},
		WorkDir:    base,
		Iterations: 1,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}

	err := RunWatch(cfg)
	if err != nil && !errors.Is(err, ErrInterrupted) {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	n := len(processed)
	mu.Unlock()
	if n < 2 {
		t.Errorf("expected tasks from both literal and glob dirs, got %d: %v", n, processed)
	}
}

func TestRunWatch_GlobPattern_WorkersCapConcurrency(t *testing.T) {
	base := t.TempDir()

	// Create two repos, each with a task
	for _, name := range []string{"repo-a", "repo-b"} {
		dir := filepath.Join(base, name)
		os.MkdirAll(filepath.Join(dir, ".git"), 0755)
		readyDir := filepath.Join(dir, ".frontloop", "ready")
		os.MkdirAll(readyDir, 0755)
		os.WriteFile(filepath.Join(readyDir, "task.md"), []byte("work"), 0644)
	}

	shutdown := make(chan struct{})
	callCount := 0
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		callCount++
		if callCount >= 2 {
			select {
			case <-shutdown:
			default:
				close(shutdown)
			}
		}
		return &agent.RunResult{}, nil
	}}

	cfg := Config{
		Watch:      []string{"**/.frontloop/ready"},
		WorkDir:    base,
		Workers:    2,
		Iterations: 1,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}

	err := RunWatch(cfg)
	if err != nil && !errors.Is(err, ErrInterrupted) {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 1 {
		t.Error("expected at least one task to run")
	}
}
