package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ohare93/juggle/internal/agent"
)

// funcRunner delegates Run to a function; useful for ad-hoc test runners.
type funcRunner struct {
	run func(agent.RunOptions) (*agent.RunResult, error)
}

func (r *funcRunner) Run(opts agent.RunOptions) (*agent.RunResult, error) {
	return r.run(opts)
}

// runWatchTaskMultipleTimes simulates the outer loop calling runWatchTask multiple times.
// For testing iteration-related behavior in the new architecture.
func runWatchTaskMultipleTimes(cfg Config, taskFile string, maxIter int, state *runTaskState, stats *runStats) error {
	for i := 1; i <= maxIter; i++ {
		if err := runWatchTask(cfg, taskFile, i, maxIter, state, stats); err != nil {
			if errors.Is(err, errFileGone) {
				// File completed, stop iterating
				return nil
			}
			if errors.Is(err, errRetryIteration) {
				// Retry same iteration (don't increment i)
				i--
				continue
			}
			return err
		}
	}
	return nil
}

func TestScanWatchDir(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("picks first file alphabetically", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "b-task.md"), []byte("b"), 0644)
		os.WriteFile(filepath.Join(dir, "a-task.md"), []byte("a"), 0644)
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "a-task.md" {
			t.Errorf("expected a-task.md, got %q", filepath.Base(got))
		}
	})

	t.Run("skips hidden files", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "visible.md"), []byte("y"), 0644)
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "visible.md" {
			t.Errorf("expected visible.md, got %q", filepath.Base(got))
		}
	})

	t.Run("skips directories", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, "subdir"), 0755)
		os.WriteFile(filepath.Join(dir, "task.md"), []byte("t"), 0644)
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "task.md" {
			t.Errorf("expected task.md, got %q", filepath.Base(got))
		}
	})

	t.Run("numeric prefixes control order", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "02-low.md"), []byte("l"), 0644)
		os.WriteFile(filepath.Join(dir, "01-high.md"), []byte("h"), 0644)
		got, err := ScanWatchDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != "01-high.md" {
			t.Errorf("expected 01-high.md, got %q", filepath.Base(got))
		}
	})
}

func TestRunWatchTask_OnIterDoneCalledPerIteration(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("do work"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok1"},
		&agent.RunResult{Output: "ok2"},
		&agent.RunResult{Output: "ok3"},
	)

	var mu sync.Mutex
	var calls [][2]int // (iter, maxIter)
	cfg := Config{
		Content:    "context",
		Iterations: 3,
		Model:      "sonnet",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
		OnIterDone: func(iter, maxIter int) {
			mu.Lock()
			calls = append(calls, [2]int{iter, maxIter})
			mu.Unlock()
		},
	}

	if err := runWatchTaskMultipleTimes(cfg, taskPath, 3, newRunTaskState(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 OnIterDone calls, got %d", len(calls))
	}
	for i, c := range calls {
		if c[0] != i+1 {
			t.Errorf("call %d: expected iter %d, got %d", i, i+1, c[0])
		}
		if c[1] != 3 {
			t.Errorf("call %d: expected maxIter 3, got %d", i, c[1])
		}
	}
}

func TestRunWatchTask_Iterations(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("implement feature"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "iter1"},
		&agent.RunResult{Output: "iter2"},
	)

	cfg := Config{
		Content:    "context",
		Iterations: 2,
		Model:      "sonnet",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}

	err := runWatchTaskMultipleTimes(cfg, taskPath, 2, newRunTaskState(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(mock.Calls))
	}

	// Both prompts should contain task contents and filename
	for i, call := range mock.Calls {
		if !strings.Contains(call.Prompt, "implement feature") {
			t.Errorf("call %d missing task contents", i)
		}
		if !strings.Contains(call.Prompt, "task.md") {
			t.Errorf("call %d missing filename", i)
		}
	}
}

func TestRunWatchTask_FileDeletedByAgent(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	// Don't create the file — simulate agent already deleted it

	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:    "context",
		Iterations: 5,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}

	err := runWatchTask(cfg, taskPath, 1, 1, newRunTaskState(), nil)
	if !errors.Is(err, errFileGone) {
		t.Fatalf("expected errFileGone, got %v", err)
	}

	// Should return immediately since file doesn't exist
	if len(mock.Calls) != 0 {
		t.Errorf("expected 0 calls when file missing, got %d", len(mock.Calls))
	}
}

func TestRunWatch_InvalidDirectory(t *testing.T) {
	cfg := Config{
		Watch:  []string{"/nonexistent/directory"},
		Stderr: &bytes.Buffer{},
	}
	err := RunWatch(cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestRunWatch_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notadir")
	os.WriteFile(filePath, []byte("x"), 0644)

	cfg := Config{
		Watch:  []string{filePath},
		Stderr: &bytes.Buffer{},
	}
	err := RunWatch(cfg)
	if err == nil {
		t.Fatal("expected error for non-directory path")
	}
}

func TestRunWatch_PreClosedShutdown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "task.md"), []byte("task"), 0644)

	shutdown := make(chan struct{})
	close(shutdown)
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Watch:    []string{dir},
		Runner:   mock,
		Shutdown: shutdown,
		Stderr:   &bytes.Buffer{},
	}
	err := RunWatch(cfg)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected 0 calls with pre-closed shutdown, got %d", len(mock.Calls))
	}
}

func TestRunWatchTask_ConsecutiveFailuresStop(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0}, // should not be reached
	)
	cfg := Config{
		Content:     "context",
		Iterations:  10,
		OnFailure:   OnFailureContinue,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	err := runWatchTaskMultipleTimes(cfg, taskPath, 10, newRunTaskState(), nil)
	if err == nil {
		t.Fatal("expected error on consecutive failures")
	}
	if !strings.Contains(err.Error(), "3 consecutive failures") {
		t.Errorf("error should mention '3 consecutive failures', got: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls before stop, got %d", len(mock.Calls))
	}
}

func TestRunWatchTask_ConsecutiveFailureCounterResets(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0}, // resets counter
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
	)
	cfg := Config{
		Content:     "context",
		Iterations:  5,
		OnFailure:   OnFailureContinue,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	err := runWatchTaskMultipleTimes(cfg, taskPath, 5, newRunTaskState(), nil)
	if err != nil {
		t.Fatalf("counter should reset on success, got: %v", err)
	}
	if len(mock.Calls) != 5 {
		t.Errorf("expected 5 calls, got %d", len(mock.Calls))
	}
}

func TestBuildRunOptions(t *testing.T) {
	t.Run("headless mode with accept edits", func(t *testing.T) {
		cfg := Config{
			Model:        "sonnet",
			Timeout:      0,
			ShowThinking: false,
		}
		opts := buildRunOptions(cfg, "test prompt")
		if opts.Mode != agent.ModeHeadless {
			t.Errorf("expected headless, got %s", opts.Mode)
		}
		if opts.Permission != agent.PermissionAcceptEdits {
			t.Errorf("expected acceptEdits, got %s", opts.Permission)
		}
		if opts.Prompt != "test prompt" {
			t.Errorf("expected 'test prompt', got %q", opts.Prompt)
		}
		if opts.Model != "sonnet" {
			t.Errorf("expected sonnet, got %q", opts.Model)
		}
	})

	t.Run("plan mode", func(t *testing.T) {
		cfg := Config{Plan: true}
		opts := buildRunOptions(cfg, "prompt")
		if opts.Permission != agent.PermissionPlan {
			t.Errorf("expected plan, got %s", opts.Permission)
		}
	})

	t.Run("interactive mode with bypass", func(t *testing.T) {
		cfg := Config{
			Interactive:  true,
			Trust:        true,
			Model:        "opus",
			ShowThinking: true,
		}
		opts := buildRunOptions(cfg, "prompt")
		if opts.Mode != agent.ModeInteractive {
			t.Errorf("expected interactive, got %s", opts.Mode)
		}
		if opts.Permission != agent.PermissionBypass {
			t.Errorf("expected bypass, got %s", opts.Permission)
		}
		if !opts.ShowThinking {
			t.Error("expected ShowThinking=true")
		}
	})
}

func TestBuildRunOptions_SystemPrompt(t *testing.T) {
	cfg := Config{SystemPrompt: "You are a helpful assistant"}
	opts := buildRunOptions(cfg, "prompt")
	if opts.SystemPrompt != "You are a helpful assistant" {
		t.Errorf("expected SystemPrompt='You are a helpful assistant', got %q", opts.SystemPrompt)
	}
}

func TestBuildRunOptions_WorkDir(t *testing.T) {
	cfg := Config{WorkDir: "/some/dir"}
	opts := buildRunOptions(cfg, "prompt")
	if opts.WorkingDir != "/some/dir" {
		t.Errorf("expected WorkingDir=/some/dir, got %q", opts.WorkingDir)
	}
}

func TestBuildRunOptions_PassthroughArgs(t *testing.T) {
	cfg := Config{PassthroughArgs: []string{"--max-turns", "50", "--allowedTools", "Bash"}}
	opts := buildRunOptions(cfg, "prompt")
	if len(opts.PassthroughArgs) != 4 {
		t.Fatalf("expected 4 passthrough args, got %d: %v", len(opts.PassthroughArgs), opts.PassthroughArgs)
	}
	if opts.PassthroughArgs[0] != "--max-turns" || opts.PassthroughArgs[1] != "50" {
		t.Errorf("unexpected passthrough args: %v", opts.PassthroughArgs)
	}
}

func TestRunWatchTask_StopWhenExitsZeroStops(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("do work"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "iter1"},
		&agent.RunResult{Output: "iter2"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "context",
		Iterations: 10,
		Runner:     mock,
		Stderr:     &stderr,
		StopWhen:   "true", // always exits 0
	}

	err := runWatchTask(cfg, taskPath, 1, 1, newRunTaskState(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 call before stop-when triggered, got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "stop-when") {
		t.Errorf("expected stop-when message in stderr, got: %s", stderr.String())
	}
}

func TestRunWatchTask_MaxCostGuardTriggersAtThreshold(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("do work"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens}, // should not be reached
	)
	var stderr bytes.Buffer
	stats := &runStats{model: "sonnet"}
	cfg := Config{
		Content:    "context",
		Iterations: 5,
		Model:      "sonnet",
		MaxCost:    costGuardMaxCost,
		Runner:     mock,
		Stderr:     &stderr,
	}

	err := runWatchTask(cfg, taskPath, 1, 1, newRunTaskState(), stats)
	if !errors.Is(err, errCostGuard) {
		t.Fatalf("expected errCostGuard, got: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 call before cost guard triggered, got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "cost guard triggered") {
		t.Errorf("expected 'cost guard triggered' in stderr, got: %s", stderr.String())
	}
}

func TestRunWatch_MaxCostGuardPrintsSummaryAndExitsClean(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "task.md"), []byte("do work"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Watch:      []string{dir},
		Content:    "context",
		Iterations: 5,
		Model:      "sonnet",
		MaxCost:    costGuardMaxCost,
		Runner:     mock,
		Stderr:     &stderr,
	}

	err := RunWatch(cfg)
	if err != nil {
		t.Fatalf("cost guard in watch mode should exit cleanly (nil), got: %v", err)
	}
	if !strings.Contains(stderr.String(), "Run summary") {
		t.Errorf("expected 'Run summary' in stderr on watch cost guard exit, got: %s", stderr.String())
	}
}

// --- OnFailure tests for RunWatch/runWatchTask ---

func TestRunWatchTask_OnFailureStop_HaltsOnFirstFailure(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0}, // should not be reached
	)
	cfg := Config{
		Content:    "context",
		Iterations: 5,
		OnFailure:  OnFailureStop,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := runWatchTask(cfg, taskPath, 1, 1, newRunTaskState(), nil)
	if err == nil {
		t.Fatal("expected error when OnFailureStop and iteration fails")
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 call before stop, got %d", len(mock.Calls))
	}
}

func TestRunWatchTask_OnFailureContinue_SkipsToNext(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0},
		&agent.RunResult{ExitCode: 0},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:     "context",
		Iterations:  3,
		OnFailure:   OnFailureContinue,
		MaxFailures: 5,
		Runner:      mock,
		Stderr:      &stderr,
	}
	err := runWatchTaskMultipleTimes(cfg, taskPath, 3, newRunTaskState(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "continuing") {
		t.Errorf("expected 'continuing' in stderr, got: %s", stderr.String())
	}
}

func TestRunWatchTask_OnFailureRetry_RetriesBeforeAdvancing(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task.md")
	os.WriteFile(taskPath, []byte("task content"), 0644)

	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1}, // iter 1, attempt 1
		&agent.RunResult{ExitCode: 1}, // iter 1, retry 1
		&agent.RunResult{ExitCode: 0}, // iter 1, retry 2 (success)
		&agent.RunResult{ExitCode: 0}, // iter 2
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:       "context",
		Iterations:    2,
		OnFailure:     OnFailureRetry,
		Retries:       2,
		MaxFailures:   5,
		RetryBackoffs: []time.Duration{time.Millisecond, time.Millisecond},
		Runner:        mock,
		Stderr:        &stderr,
	}
	err := runWatchTaskMultipleTimes(cfg, taskPath, 2, newRunTaskState(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 4 {
		t.Errorf("expected 4 calls (fail+retry+success+iter2), got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "retrying") {
		t.Errorf("expected 'retrying' in stderr, got: %s", stderr.String())
	}
}

// --- Workers tests ---

func TestScanWatchDirAll_ReturnsAllFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	files, err := ScanWatchDirAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files (no hidden/dirs), got %d: %v", len(files), files)
	}
}

func TestScanWatchDirAll_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := ScanWatchDirAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty, got %v", files)
	}
}

func TestWorkerCoordinator_ClaimsAreMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0644)

	coord := newWorkerCoordinator()

	path1, err := coord.claim(dir)
	if err != nil || path1 == "" {
		t.Fatalf("expected first claim, got %q, %v", path1, err)
	}

	path2, err := coord.claim(dir)
	if err != nil || path2 == "" {
		t.Fatalf("expected second claim, got %q, %v", path2, err)
	}

	if path1 == path2 {
		t.Errorf("two workers claimed same file %q", path1)
	}

	// All claimed — third returns empty
	path3, err := coord.claim(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path3 != "" {
		t.Errorf("expected empty when all files claimed, got %q", path3)
	}
}

func TestWorkerCoordinator_ReleaseAllowsReclaim(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "task.md"), []byte("t"), 0644)

	coord := newWorkerCoordinator()
	path1, _ := coord.claim(dir)
	if path1 == "" {
		t.Fatal("expected initial claim")
	}

	// Still claimed — cannot reclaim
	path2, _ := coord.claim(dir)
	if path2 != "" {
		t.Errorf("expected empty while claimed, got %q", path2)
	}

	coord.release(path1)

	// Now available again
	path3, _ := coord.claim(dir)
	if path3 == "" {
		t.Error("expected reclaim after release")
	}
}

func TestRun_WorkersWithoutWatch_IsError(t *testing.T) {
	cfg := Config{
		Content: "prompt",
		Workers: 2,
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		Runner:  agent.NewMockRunner(&agent.RunResult{}),
	}
	err := Run(cfg)
	if err == nil {
		t.Fatal("expected error for --workers without --watch")
	}
	if !strings.Contains(err.Error(), "workers") {
		t.Errorf("error should mention --workers, got: %v", err)
	}
}

func TestRunWatchWorkers_WorkerIDInEnv(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "task.md"), []byte("task content"), 0644)

	shutdown := make(chan struct{})
	var mu sync.Mutex
	var calls []agent.RunOptions

	runner := &funcRunner{func(opts agent.RunOptions) (*agent.RunResult, error) {
		mu.Lock()
		calls = append(calls, opts)
		mu.Unlock()
		select {
		case <-shutdown:
		default:
			close(shutdown)
		}
		return &agent.RunResult{}, nil
	}}

	cfg := Config{
		Watch:      []string{dir},
		Workers:    2,
		Iterations: 5,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}

	RunWatch(cfg) //nolint:errcheck

	mu.Lock()
	c := append([]agent.RunOptions{}, calls...)
	mu.Unlock()

	if len(c) == 0 {
		t.Fatal("expected at least one call")
	}
	for i, call := range c {
		hasWorkerID := false
		for _, e := range call.Env {
			if strings.HasPrefix(e, "JUGGLE_WORKER_ID=") {
				hasWorkerID = true
				break
			}
		}
		if !hasWorkerID {
			t.Errorf("call %d missing JUGGLE_WORKER_ID in env: %v", i, call.Env)
		}
	}
}

func TestRunWatchWorkers_NoDuplicateTaskSelection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "task1.md"), []byte("t1"), 0644)
	os.WriteFile(filepath.Join(dir, "task2.md"), []byte("t2"), 0644)

	shutdown := make(chan struct{})
	var mu sync.Mutex
	active := make(map[string]int) // filename → count of concurrent runs
	var duplicateDetected bool
	totalCalls := 0

	runner := &funcRunner{func(opts agent.RunOptions) (*agent.RunResult, error) {
		var taskFile string
		for _, e := range opts.Env {
			if strings.HasPrefix(e, "JUGGLE_TASK_FILE=") {
				taskFile = filepath.Base(strings.TrimPrefix(e, "JUGGLE_TASK_FILE="))
			}
		}

		mu.Lock()
		active[taskFile]++
		if active[taskFile] > 1 {
			duplicateDetected = true
		}
		totalCalls++
		n := totalCalls
		mu.Unlock()

		// Hold briefly so both workers can be active simultaneously
		time.Sleep(2 * time.Millisecond)

		mu.Lock()
		active[taskFile]--
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
		Watch:      []string{dir},
		Workers:    2,
		Iterations: 10,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}

	err := RunWatch(cfg)
	if err != nil && !errors.Is(err, ErrInterrupted) {
		t.Fatalf("unexpected error: %v", err)
	}

	if duplicateDetected {
		t.Error("two workers claimed the same task file simultaneously")
	}
	if totalCalls < 1 {
		t.Error("expected at least one Run call")
	}
}

func TestRun_WatchRelativeToWorkDir(t *testing.T) {
	workdir := t.TempDir()
	watchSubdir := "tasks"
	watchFull := filepath.Join(workdir, watchSubdir)
	if err := os.Mkdir(watchFull, 0755); err != nil {
		t.Fatal(err)
	}
	// Add a task file so the watcher has something to process
	taskPath := filepath.Join(watchFull, "task.md")
	os.WriteFile(taskPath, []byte("do work"), 0644)

	shutdown := make(chan struct{})
	callCount := 0
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		callCount++
		os.Remove(taskPath) // remove so watcher exits
		close(shutdown)
		return &agent.RunResult{}, nil
	}}

	cfg := Config{
		Content:    "instructions",
		Watch:      []string{watchSubdir}, // relative
		WorkDir:    workdir,
		Iterations: 1,
		Runner:     runner,
		Stderr:     &bytes.Buffer{},
		Shutdown:   shutdown,
	}

	err := Run(cfg)
	if err != nil && !errors.Is(err, ErrInterrupted) {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount == 0 {
		t.Error("expected at least one Run call (watch subdir not resolved correctly)")
	}
}

func TestRun_WatchAbsoluteNotChangedByWorkDir(t *testing.T) {
	workdir := t.TempDir()
	watchdir := t.TempDir() // separate absolute path
	taskPath := filepath.Join(watchdir, "task.md")
	os.WriteFile(taskPath, []byte("do work"), 0644)

	shutdown := make(chan struct{})
	callCount := 0
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		callCount++
		os.Remove(taskPath)
		close(shutdown)
		return &agent.RunResult{}, nil
	}}

	cfg := Config{
		Content:    "instructions",
		Watch:      []string{watchdir}, // absolute — should not be joined to workdir
		WorkDir:    workdir,
		Iterations: 1,
		Runner:     runner,
		Stderr:     &bytes.Buffer{},
		Shutdown:   shutdown,
	}

	err := Run(cfg)
	if err != nil && !errors.Is(err, ErrInterrupted) {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount == 0 {
		t.Error("expected at least one Run call (absolute watch path should work with workdir)")
	}
}

func TestRun_MultiWatch_RelativePathsResolvedAgainstWorkDir(t *testing.T) {
	workdir := t.TempDir()
	sub1 := "tasks1"
	sub2 := "tasks2"
	dir1 := filepath.Join(workdir, sub1)
	dir2 := filepath.Join(workdir, sub2)
	os.Mkdir(dir1, 0755)
	os.Mkdir(dir2, 0755)
	taskPath := filepath.Join(dir1, "task.md")
	os.WriteFile(taskPath, []byte("work"), 0644)

	shutdown := make(chan struct{})
	callCount := 0
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		// Delete the task file so it's not re-claimed, then signal shutdown.
		for _, e := range opts.Env {
			if strings.HasPrefix(e, "JUGGLE_TASK_FILE=") {
				os.Remove(strings.TrimPrefix(e, "JUGGLE_TASK_FILE="))
				break
			}
		}
		callCount++
		close(shutdown)
		return &agent.RunResult{}, nil
	}}

	cfg := Config{
		Watch:      []string{sub1, sub2}, // relative paths
		WorkDir:    workdir,
		Iterations: 1,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}

	err := Run(cfg)
	if err != nil && !errors.Is(err, ErrInterrupted) {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount == 0 {
		t.Error("expected at least one task to run (relative multi-watch paths not resolved)")
	}
}
