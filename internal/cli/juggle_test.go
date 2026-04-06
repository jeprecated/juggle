package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ohare93/juggle/internal/agent"
)

// closeOnFirstCallRunner closes shutdown after its first Run call.
// Used to simulate a signal arriving after one iteration completes.
type closeOnFirstCallRunner struct {
	shutdown chan struct{}
	result   *agent.RunResult
	calls    int
	once     sync.Once
}

func (r *closeOnFirstCallRunner) Run(opts agent.RunOptions) (*agent.RunResult, error) {
	r.calls++
	r.once.Do(func() { close(r.shutdown) })
	if r.result != nil {
		return r.result, nil
	}
	return &agent.RunResult{Output: "ok"}, nil
}

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
		got := BuildWatchPrompt("task data", "instructions", "tasks/task-001.md", 2, 5)
		if !strings.Contains(got, "<task>\nfile: tasks/task-001.md\ntask data\n</task>") {
			t.Error("missing task section with file path")
		}
		if !strings.Contains(got, "instructions") {
			t.Error("missing content")
		}
		if !strings.Contains(got, "iteration 2 of 5") {
			t.Error("missing iteration in footer")
		}
		if !strings.Contains(got, "processing tasks/task-001.md") {
			t.Error("missing relative path in footer")
		}
	})
}

func TestRun_DryRun(t *testing.T) {
	var stdout bytes.Buffer
	cfg := Config{
		Content:    "fix the tests",
		Iterations: 10,
		DryRun:     true,
		Stdout:     &stdout,
	}
	err := Run(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "fix the tests") {
		t.Error("dry-run output missing content")
	}
	if !strings.Contains(output, "iteration 1 of 10") {
		t.Error("dry-run output missing footer")
	}
}

func TestRun_WorkDirNotExist(t *testing.T) {
	cfg := Config{
		Content:  "do work",
		WorkDir:  "/nonexistent/path/that/does/not/exist",
		Stderr:   &bytes.Buffer{},
		Runner:   agent.NewMockRunner(&agent.RunResult{}),
	}
	err := Run(cfg)
	if err == nil {
		t.Fatal("expected error for non-existent workdir")
	}
	if !strings.Contains(err.Error(), "workdir") {
		t.Errorf("error should mention 'workdir', got: %v", err)
	}
}

func TestRun_WorkDirSetsAgentWorkingDir(t *testing.T) {
	dir := t.TempDir()
	var capturedOpts agent.RunOptions
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		capturedOpts = opts
		return &agent.RunResult{}, nil
	}}
	cfg := Config{
		Content:    "do work",
		Iterations: 1,
		WorkDir:    dir,
		Runner:     runner,
		Stderr:     &bytes.Buffer{},
	}
	if err := Run(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOpts.WorkingDir != dir {
		t.Errorf("expected WorkingDir=%q, got %q", dir, capturedOpts.WorkingDir)
	}
}

func TestRunLoop_Iterations(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "done1"},
		&agent.RunResult{Output: "done2"},
		&agent.RunResult{Output: "done3"},
	)
	cfg := Config{
		Content:    "do work",
		Iterations: 3,
		Model:      "sonnet",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(mock.Calls))
	}
	for i, call := range mock.Calls {
		expected := fmt.Sprintf("iteration %d of 3", i+1)
		if !strings.Contains(call.Prompt, expected) {
			t.Errorf("call %d: prompt missing %q", i, expected)
		}
	}
}

func TestRunLoop_HeadlessMode(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{Content: "test", Iterations: 1, Model: "sonnet", Runner: mock, Stderr: &bytes.Buffer{}}
	RunLoop(cfg)
	if mock.Calls[0].Mode != agent.ModeHeadless {
		t.Errorf("expected headless, got %s", mock.Calls[0].Mode)
	}
	if mock.Calls[0].Permission != agent.PermissionAcceptEdits {
		t.Errorf("expected acceptEdits, got %s", mock.Calls[0].Permission)
	}
}

func TestRunLoop_SystemPrompt(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:      "test",
		Iterations:   1,
		Runner:       mock,
		Stderr:       &bytes.Buffer{},
		SystemPrompt: "You are an expert coder",
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.Calls[0].SystemPrompt != "You are an expert coder" {
		t.Errorf("SystemPrompt = %q, want 'You are an expert coder'", mock.Calls[0].SystemPrompt)
	}
}

func TestRunLoop_PassthroughArgs(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:         "test",
		Iterations:      1,
		Runner:          mock,
		Stderr:          &bytes.Buffer{},
		PassthroughArgs: []string{"--max-turns", "50"},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.Calls))
	}
	got := mock.Calls[0].PassthroughArgs
	if len(got) != 2 || got[0] != "--max-turns" || got[1] != "50" {
		t.Errorf("PassthroughArgs = %v, want [--max-turns 50]", got)
	}
}

func TestSplitPassthroughArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		dashLen      int
		wantNormal   []string
		wantPassthru []string
	}{
		{"no dash", []string{"@task.md"}, -1, []string{"@task.md"}, nil},
		{"dash with extras", []string{"@task.md", "--max-turns", "50"}, 1, []string{"@task.md"}, []string{"--max-turns", "50"}},
		{"dash nothing after", []string{"@task.md"}, 1, []string{"@task.md"}, nil},
		{"only passthrough", []string{"--verbose", "true"}, 0, nil, []string{"--verbose", "true"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			normal, passthru := splitPassthroughArgs(tc.args, tc.dashLen)
			if len(normal) != len(tc.wantNormal) {
				t.Errorf("normal = %v, want %v", normal, tc.wantNormal)
			}
			if len(passthru) != len(tc.wantPassthru) {
				t.Errorf("passthru = %v, want %v", passthru, tc.wantPassthru)
			}
		})
	}
}

func TestRunLoop_TrustMode(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{Content: "test", Iterations: 1, Trust: true, Runner: mock, Stderr: &bytes.Buffer{}}
	RunLoop(cfg)
	if mock.Calls[0].Permission != agent.PermissionBypass {
		t.Errorf("expected bypass, got %s", mock.Calls[0].Permission)
	}
}

func TestRunLoop_PlanMode(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{Content: "test", Iterations: 1, Plan: true, Runner: mock, Stderr: &bytes.Buffer{}}
	RunLoop(cfg)
	if mock.Calls[0].Permission != agent.PermissionPlan {
		t.Errorf("expected plan, got %s", mock.Calls[0].Permission)
	}
}

func TestRunLoop_InteractiveMode(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{Content: "test", Iterations: 1, Interactive: true, Runner: mock, Stderr: &bytes.Buffer{}}
	RunLoop(cfg)
	if mock.Calls[0].Mode != agent.ModeInteractive {
		t.Errorf("expected interactive, got %s", mock.Calls[0].Mode)
	}
}

func TestRunLoop_SetsModelAndTimeout(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content: "test", Iterations: 1, Model: "opus",
		Timeout: 5 * time.Minute, ShowThinking: true,
		Runner: mock, Stderr: &bytes.Buffer{},
	}
	RunLoop(cfg)
	call := mock.Calls[0]
	if call.Model != "opus" {
		t.Errorf("model = %q, want opus", call.Model)
	}
	if call.Timeout != 5*time.Minute {
		t.Errorf("timeout = %v, want 5m", call.Timeout)
	}
	if !call.ShowThinking {
		t.Error("expected ShowThinking=true")
	}
}

func TestRunLoop_RateLimitRetry(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{RateLimited: true, RetryAfter: 1 * time.Millisecond},
		&agent.RunResult{Output: "success"},
	)
	cfg := Config{Content: "test", Iterations: 1, Runner: mock, Stderr: &bytes.Buffer{}}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 2 {
		t.Fatalf("expected 2 calls (rate limit + retry), got %d", len(mock.Calls))
	}
}

func TestRunLoop_MaxWaitExceeded(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{RateLimited: true, RetryAfter: 1 * time.Hour},
	)
	cfg := Config{Content: "test", Iterations: 1, MaxWait: 1 * time.Second, Runner: mock, Stderr: &bytes.Buffer{}}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error when max wait exceeded")
	}
	if !strings.Contains(err.Error(), "max-wait") {
		t.Errorf("error should mention max-wait, got: %v", err)
	}
}

func TestRunLoop_OverloadExhausted(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{OverloadExhausted: true})
	cfg := Config{Content: "test", Iterations: 1, Runner: mock, Stderr: &bytes.Buffer{}}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error on overload")
	}
}

func TestRunLoop_NilShutdownRunsNormally(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok1"},
		&agent.RunResult{Output: "ok2"},
	)
	cfg := Config{
		Content:    "test",
		Iterations: 2,
		Runner:     mock,
		Shutdown:   nil,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 2 {
		t.Errorf("expected 2 calls, got %d", len(mock.Calls))
	}
}

func TestRunLoop_PreClosedShutdownRunsNoIterations(t *testing.T) {
	shutdown := make(chan struct{})
	close(shutdown)
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:    "test",
		Iterations: 3,
		Runner:     mock,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected 0 calls with pre-closed shutdown, got %d", len(mock.Calls))
	}
}

func TestRunLoop_ShutdownPreventNextIteration(t *testing.T) {
	shutdown := make(chan struct{})
	runner := &closeOnFirstCallRunner{shutdown: shutdown}
	cfg := Config{
		Content:    "test",
		Iterations: 5,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if runner.calls != 1 {
		t.Errorf("expected 1 call, got %d", runner.calls)
	}
}

func TestRunLoop_ShutdownPrintsSummary(t *testing.T) {
	shutdown := make(chan struct{})
	runner := &closeOnFirstCallRunner{
		shutdown: shutdown,
		result:   &agent.RunResult{InputTokens: 100, OutputTokens: 50},
	}
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 5,
		Runner:     runner,
		Shutdown:   shutdown,
		Stderr:     &stderr,
	}
	RunLoop(cfg)
	if !strings.Contains(stderr.String(), "Run summary") {
		t.Errorf("expected 'Run summary' in stderr, got: %s", stderr.String())
	}
}

func TestHelpExamplesExist(t *testing.T) {
	checks := []string{
		`"fix the failing tests"`,
		"@task.md",
		"juggle watch",
		"--trust",
	}
	for _, want := range checks {
		if !strings.Contains(rootCmd.Example, want) {
			t.Errorf("rootCmd.Example missing %q", want)
		}
	}
}

func TestWatchSubcommand_Exists(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"watch"})
	if err != nil {
		t.Fatalf("finding watch subcommand: %v", err)
	}
	if cmd == nil || cmd.Name() != "watch" {
		t.Error("watch subcommand not registered under rootCmd")
	}
}

func TestWatchSubcommand_HasWatchSpecificFlags(t *testing.T) {
	cmd, _, _ := rootCmd.Find([]string{"watch"})
	for _, name := range []string{"dir", "workers", "dashboard"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("watch subcommand missing --%s flag", name)
		}
	}
}

func TestRootCmd_WatchFlagsRemoved(t *testing.T) {
	for _, name := range []string{"watch", "workers", "dashboard"} {
		if rootCmd.Flags().Lookup(name) != nil {
			t.Errorf("root command should not have --%s flag", name)
		}
		if rootCmd.PersistentFlags().Lookup(name) != nil {
			t.Errorf("root command persistent flags should not have --%s flag", name)
		}
	}
}

func TestWatchSubcommand_InheritsSharedFlags(t *testing.T) {
	// Shared flags are on root's PersistentFlags; cobra inherits them to all subcommands.
	pf := rootCmd.PersistentFlags()
	for _, name := range []string{"iterations", "model", "trust", "dry-run", "cmd-before", "cmd-after"} {
		if pf.Lookup(name) == nil {
			t.Errorf("shared flag --%s should be on root's persistent flags (inherited by watch)", name)
		}
	}
}

func TestWatchSubcommand_DryRun(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	cfg := Config{
		Content: "do the work",
		Watch:   []string{dir},
		DryRun:  true,
		Stdout:  &out,
		Stderr:  &bytes.Buffer{},
	}
	err := Run(cfg)
	if err != nil {
		t.Fatalf("watch --dry-run should not error, got: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "main prompt") {
		t.Errorf("dry-run output should contain 'main prompt', got:\n%s", output)
	}
	if !strings.Contains(output, "do the work") {
		t.Errorf("dry-run output should contain the prompt content, got:\n%s", output)
	}
}

func TestSetVersionUpdatesRootCmd(t *testing.T) {
	prev := rootCmd.Version
	defer func() { rootCmd.Version = prev }()

	SetVersion("9.9.9")
	if rootCmd.Version != "9.9.9" {
		t.Errorf("rootCmd.Version = %q, want 9.9.9", rootCmd.Version)
	}
}

func TestNoArgsExecuteShowsHelp(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{})
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("no-args should show help without error, got: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "fix the failing tests") {
		t.Errorf("help output should contain examples, got:\n%s", output)
	}
}

func TestRunLoop_ConsecutiveFailuresStop(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0}, // should not be reached
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:     "test",
		Iterations:  10,
		OnFailure:   OnFailureContinue,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &stderr,
	}
	err := RunLoop(cfg)
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

func TestRunLoop_ConsecutiveFailureCounterResets(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0}, // success resets counter
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
	)
	cfg := Config{
		Content:     "test",
		Iterations:  5,
		OnFailure:   OnFailureContinue,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("counter should reset on success, got error: %v", err)
	}
	if len(mock.Calls) != 5 {
		t.Errorf("expected 5 calls, got %d", len(mock.Calls))
	}
}

func TestRunLoop_MaxFailuresZeroDisablesCheck(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
	)
	cfg := Config{
		Content:     "test",
		Iterations:  5,
		OnFailure:   OnFailureContinue,
		MaxFailures: 0, // disabled
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("MaxFailures=0 should disable check, got: %v", err)
	}
	if len(mock.Calls) != 5 {
		t.Errorf("expected 5 calls, got %d", len(mock.Calls))
	}
}

func TestRunLoop_RateLimitNotCountedAsFailure(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{RateLimited: true, RetryAfter: 1 * time.Millisecond},
		&agent.RunResult{ExitCode: 0}, // iter 3 retry: success resets to 0
	)
	cfg := Config{
		Content:     "test",
		Iterations:  3,
		OnFailure:   OnFailureContinue,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("rate-limited retry should not count as failure: %v", err)
	}
	if len(mock.Calls) != 4 {
		t.Errorf("expected 4 calls (fail+fail+rate-limit+success), got %d", len(mock.Calls))
	}
}

func TestRunLoop_StopWhenExitsZeroStops(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "done1"},
		&agent.RunResult{Output: "done2"},
		&agent.RunResult{Output: "done3"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 10,
		Runner:     mock,
		Stderr:     &stderr,
		StopWhen:   "true", // always exits 0
	}
	err := RunLoop(cfg)
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

func TestRunLoop_StopWhenNonZeroContinues(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "done1"},
		&agent.RunResult{Output: "done2"},
		&agent.RunResult{Output: "done3"},
	)
	cfg := Config{
		Content:    "test",
		Iterations: 3,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
		StopWhen:   "false", // always exits non-zero
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls when stop-when never exits 0, got %d", len(mock.Calls))
	}
}

func TestRunLoop_StopWhenIterationsWinsFirst(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "done1"},
	)
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
		StopWhen:   "false", // never exits 0
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(mock.Calls))
	}
}

func TestRunLoop_QuotaExhaustedWithResetTime(t *testing.T) {
	resetAt := time.Now().Add(10 * time.Millisecond)
	mock := agent.NewMockRunner(
		&agent.RunResult{QuotaExhausted: true, QuotaResetsAt: resetAt},
		&agent.RunResult{Output: "success"},
	)
	var stderr bytes.Buffer
	cfg := Config{Content: "test", Iterations: 1, Runner: mock, Stderr: &stderr}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 2 {
		t.Fatalf("expected 2 calls (quota + retry), got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "usage quota hit") {
		t.Errorf("expected 'usage quota hit' in stderr, got: %s", stderr.String())
	}
}

func TestRunLoop_QuotaExhaustedLogsWaitUntil(t *testing.T) {
	resetAt := time.Now().Add(10 * time.Millisecond)
	mock := agent.NewMockRunner(
		&agent.RunResult{QuotaExhausted: true, QuotaResetsAt: resetAt},
		&agent.RunResult{Output: "success"},
	)
	var stderr bytes.Buffer
	cfg := Config{Content: "test", Iterations: 1, Runner: mock, Stderr: &stderr}
	RunLoop(cfg)
	if !strings.Contains(stderr.String(), "waiting until") {
		t.Errorf("expected 'waiting until' in stderr, got: %s", stderr.String())
	}
}

func TestRunLoop_QuotaExhaustedNoResetTime_UsesRetryAfter(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{QuotaExhausted: true, RetryAfter: 1 * time.Millisecond},
		&agent.RunResult{Output: "success"},
	)
	cfg := Config{Content: "test", Iterations: 1, Runner: mock, Stderr: &bytes.Buffer{}}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("expected retry after quota, got error: %v", err)
	}
	if len(mock.Calls) != 2 {
		t.Fatalf("expected 2 calls (quota + retry), got %d", len(mock.Calls))
	}
}

func TestRunLoop_QuotaExhaustedMaxWaitExceeded(t *testing.T) {
	resetAt := time.Now().Add(1 * time.Hour)
	mock := agent.NewMockRunner(
		&agent.RunResult{QuotaExhausted: true, QuotaResetsAt: resetAt},
	)
	cfg := Config{Content: "test", Iterations: 1, MaxWait: 1 * time.Second, Runner: mock, Stderr: &bytes.Buffer{}}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error when quota reset exceeds max-wait")
	}
	if !strings.Contains(err.Error(), "max-wait") {
		t.Errorf("error should mention max-wait, got: %v", err)
	}
}

func TestRunLoop_QuotaNotCountedAsFailure(t *testing.T) {
	resetAt := time.Now().Add(1 * time.Millisecond)
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{QuotaExhausted: true, QuotaResetsAt: resetAt},
		&agent.RunResult{ExitCode: 0}, // retry of iter 3
	)
	cfg := Config{
		Content:     "test",
		Iterations:  3,
		OnFailure:   OnFailureContinue,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("quota retry should not count as failure: %v", err)
	}
}

func TestComputeDelay(t *testing.T) {
	t.Run("zero delay and fuzz", func(t *testing.T) {
		if d := computeDelay(0, 0); d != 0 {
			t.Errorf("expected 0, got %v", d)
		}
	})
	t.Run("delay only", func(t *testing.T) {
		if d := computeDelay(5, 0); d != 5*time.Minute {
			t.Errorf("expected 5m, got %v", d)
		}
	})
	t.Run("delay with fuzz stays non-negative", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			if d := computeDelay(1, 2); d < 0 {
				t.Fatalf("delay went negative: %v", d)
			}
		}
	})
}

func TestPrintRunSummary_IncludesCost(t *testing.T) {
	var buf bytes.Buffer
	stats := runStats{
		iterations:   2,
		inputTokens:  1000,
		outputTokens: 500,
		start:        time.Now().Add(-5 * time.Second),
		model:        "sonnet",
	}
	printRunSummary(&buf, stats)
	output := buf.String()
	if !strings.Contains(output, "$") {
		t.Errorf("expected cost estimate (with $) in summary, got: %s", output)
	}
}

func TestRunLoop_PrintsSummaryOnCleanCompletion(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "done1"},
		&agent.RunResult{Output: "done2"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 2,
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Run summary") {
		t.Errorf("expected 'Run summary' in stderr on clean completion, got: %s", stderr.String())
	}
}

func TestRunLoop_StopWhenPrintsSummary(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "done"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 10,
		Runner:     mock,
		Stderr:     &stderr,
		StopWhen:   "true",
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Run summary") {
		t.Errorf("expected 'Run summary' in stderr on stop-when, got: %s", stderr.String())
	}
}

func TestRunLoop_NoSummaryOnError(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:     "test",
		Iterations:  10,
		OnFailure:   OnFailureContinue,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &stderr,
	}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(stderr.String(), "Run summary") {
		t.Errorf("expected no 'Run summary' on error exit, got: %s", stderr.String())
	}
}

func TestRunLoop_SummaryAccumulatesTokens(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: 100, OutputTokens: 50, CacheTokens: 20},
		&agent.RunResult{InputTokens: 200, OutputTokens: 100, CacheTokens: 40},
		&agent.RunResult{InputTokens: 300, OutputTokens: 150, CacheTokens: 60},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 3,
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := stderr.String()
	if !strings.Contains(output, "600 in") {
		t.Errorf("expected '600 in' (accumulated input tokens) in summary, got: %s", output)
	}
	if !strings.Contains(output, "300 out") {
		t.Errorf("expected '300 out' (accumulated output tokens) in summary, got: %s", output)
	}
}

func TestRunLoop_LogFileWritesSummary(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "juggle.log")
	mock := agent.NewMockRunner(&agent.RunResult{Output: "done"})
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		Runner:     mock,
		Stderr:     &stderr,
		Log:        logFile,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	contents, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("log file not created: %v", err)
	}
	if !strings.Contains(string(contents), `"type":"summary"`) {
		t.Errorf("expected JSON summary line in log file, got: %s", contents)
	}
}

// sonnet: (input*3 + output*15) / 1_000_000
// 100 in + 100 out = $0.0018; use MaxCost=0.001 to trigger after first iteration
const costGuardMaxCost = 0.001
const costGuardTokens = 100 // input and output tokens per iteration

func TestRunLoop_MaxCostGuardTriggersAtThreshold(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens}, // should not be reached
	)
	cfg := Config{
		Content:    "test",
		Iterations: 5,
		Model:      "sonnet",
		MaxCost:    costGuardMaxCost,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("cost guard should exit cleanly (nil error), got: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 call before cost guard triggered, got %d", len(mock.Calls))
	}
}

func TestRunLoop_MaxCostGuardDoesNotTriggerBelowThreshold(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: 1, OutputTokens: 1},
		&agent.RunResult{InputTokens: 1, OutputTokens: 1},
		&agent.RunResult{InputTokens: 1, OutputTokens: 1},
	)
	cfg := Config{
		Content:    "test",
		Iterations: 3,
		Model:      "sonnet",
		MaxCost:    100.0, // $100 threshold — won't be hit
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls when under threshold, got %d", len(mock.Calls))
	}
}

func TestRunLoop_MaxCostGuardLogsMessage(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 5,
		Model:      "sonnet",
		MaxCost:    costGuardMaxCost,
		Runner:     mock,
		Stderr:     &stderr,
	}
	RunLoop(cfg)
	output := stderr.String()
	if !strings.Contains(output, "cost guard triggered") {
		t.Errorf("expected 'cost guard triggered' in stderr, got: %s", output)
	}
	if !strings.Contains(output, "--max-cost") {
		t.Errorf("expected '--max-cost' in stderr, got: %s", output)
	}
}

func TestRunLoop_MaxCostGuardPrintsSummary(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "test",
		Iterations: 5,
		Model:      "sonnet",
		MaxCost:    costGuardMaxCost,
		Runner:     mock,
		Stderr:     &stderr,
	}
	RunLoop(cfg)
	if !strings.Contains(stderr.String(), "Run summary") {
		t.Errorf("expected 'Run summary' in stderr on cost guard exit, got: %s", stderr.String())
	}
}

func TestRunLoop_MaxCostZeroDisabled(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
		&agent.RunResult{InputTokens: costGuardTokens, OutputTokens: costGuardTokens},
	)
	cfg := Config{
		Content:    "test",
		Iterations: 3,
		Model:      "sonnet",
		MaxCost:    0, // disabled
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("MaxCost=0 should disable guard, got: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls when MaxCost=0, got %d", len(mock.Calls))
	}
}

func TestRun_PlanAndTrustMutuallyExclusive(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		Plan:       true,
		Trust:      true,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
		Stdout:     &bytes.Buffer{},
	}
	err := Run(cfg)
	if err == nil {
		t.Fatal("expected error when both --plan and --trust are set")
	}
	if !strings.Contains(err.Error(), "plan") || !strings.Contains(err.Error(), "trust") {
		t.Errorf("error should mention both flags, got: %v", err)
	}
}

func TestRun_AllowedAndDisallowedToolsBothSetError(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:         "test",
		Iterations:      1,
		AllowedTools:    []string{"Bash", "Read"},
		DisallowedTools: []string{"Write"},
		Runner:          mock,
		Stderr:          &bytes.Buffer{},
		Stdout:          &bytes.Buffer{},
	}
	err := Run(cfg)
	if err == nil {
		t.Fatal("expected error when both AllowedTools and DisallowedTools are set")
	}
	if !strings.Contains(err.Error(), "allowed-tools") || !strings.Contains(err.Error(), "disallowed-tools") {
		t.Errorf("error should mention both flags, got: %v", err)
	}
}

func TestRun_NegativeRetriesError(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		Retries:    -1,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
		Stdout:     &bytes.Buffer{},
	}
	err := Run(cfg)
	if err == nil {
		t.Fatal("expected error when --retries is negative")
	}
	if !strings.Contains(err.Error(), "retries") {
		t.Errorf("error should mention --retries, got: %v", err)
	}
}

func TestRunLoop_AllowedToolsPassedToRunner(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:      "test",
		Iterations:   1,
		AllowedTools: []string{"Bash", "Read", "Grep"},
		Runner:       mock,
		Stderr:       &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) == 0 {
		t.Fatal("no calls made")
	}
	call := mock.Calls[0]
	want := []string{"Bash", "Read", "Grep"}
	if len(call.AllowedTools) != len(want) {
		t.Fatalf("AllowedTools = %v, want %v", call.AllowedTools, want)
	}
	for i, v := range want {
		if call.AllowedTools[i] != v {
			t.Errorf("AllowedTools[%d] = %q, want %q", i, call.AllowedTools[i], v)
		}
	}
}

func TestRunLoop_DisallowedToolsPassedToRunner(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:         "test",
		Iterations:      1,
		DisallowedTools: []string{"Write", "Edit"},
		Runner:          mock,
		Stderr:          &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) == 0 {
		t.Fatal("no calls made")
	}
	call := mock.Calls[0]
	want := []string{"Write", "Edit"}
	if len(call.DisallowedTools) != len(want) {
		t.Fatalf("DisallowedTools = %v, want %v", call.DisallowedTools, want)
	}
	for i, v := range want {
		if call.DisallowedTools[i] != v {
			t.Errorf("DisallowedTools[%d] = %q, want %q", i, call.DisallowedTools[i], v)
		}
	}
}

func TestRunLoop_MaxTurnsPassedToRunner(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		MaxTurns:   50,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) == 0 {
		t.Fatal("no calls made")
	}
	if mock.Calls[0].MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50", mock.Calls[0].MaxTurns)
	}
}

func TestRunLoop_MaxTurns_Zero_NotPassed(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		MaxTurns:   0,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) == 0 {
		t.Fatal("no calls made")
	}
	if mock.Calls[0].MaxTurns != 0 {
		t.Errorf("MaxTurns = %d, want 0", mock.Calls[0].MaxTurns)
	}
}

// --- OnFailure tests ---

func TestRunLoop_OnFailureStop_HaltsOnFirstFailure(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0}, // should not be reached
	)
	cfg := Config{
		Content:    "test",
		Iterations: 5,
		OnFailure:  OnFailureStop,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error when OnFailureStop and iteration fails")
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 call before stop, got %d", len(mock.Calls))
	}
}

func TestRunLoop_OnFailureContinue_LogsAndContinues(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0},
		&agent.RunResult{ExitCode: 0},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:     "test",
		Iterations:  3,
		OnFailure:   OnFailureContinue,
		MaxFailures: 5,
		Runner:      mock,
		Stderr:      &stderr,
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "continuing") {
		t.Errorf("expected 'continuing' message in stderr, got: %s", stderr.String())
	}
}

func TestRunLoop_OnFailureContinue_MaxFailuresStopsLoop(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 0}, // should not be reached
	)
	cfg := Config{
		Content:     "test",
		Iterations:  10,
		OnFailure:   OnFailureContinue,
		MaxFailures: 3,
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error when consecutive failures hit MaxFailures")
	}
	if !strings.Contains(err.Error(), "3 consecutive failures") {
		t.Errorf("error should mention '3 consecutive failures', got: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls before stop, got %d", len(mock.Calls))
	}
}

func TestRunLoop_OnFailureRetry_RetriesBeforeAdvancing(t *testing.T) {
	// iteration 1: fail, retry1 fail, retry2 fail (exhausted) → continue to iter 2
	// iteration 2: succeed
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1}, // iter 1, attempt 1
		&agent.RunResult{ExitCode: 1}, // iter 1, retry 1
		&agent.RunResult{ExitCode: 1}, // iter 1, retry 2 (exhausted)
		&agent.RunResult{ExitCode: 0}, // iter 2
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:       "test",
		Iterations:    2,
		OnFailure:     OnFailureRetry,
		Retries:       2,
		MaxFailures:   5,
		RetryBackoffs: []time.Duration{time.Millisecond, time.Millisecond},
		Runner:        mock,
		Stderr:        &stderr,
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 4 {
		t.Errorf("expected 4 calls (3 attempts iter1 + 1 iter2), got %d", len(mock.Calls))
	}
	if !strings.Contains(stderr.String(), "retrying") {
		t.Errorf("expected 'retrying' in stderr, got: %s", stderr.String())
	}
}

func TestRunLoop_OnFailureRetry_SuccessOnRetry(t *testing.T) {
	// iteration 1: fail, retry succeeds
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1}, // iter 1, attempt 1
		&agent.RunResult{ExitCode: 0}, // iter 1, retry 1 (success)
		&agent.RunResult{ExitCode: 0}, // iter 2
	)
	cfg := Config{
		Content:       "test",
		Iterations:    2,
		OnFailure:     OnFailureRetry,
		Retries:       2,
		MaxFailures:   5,
		RetryBackoffs: []time.Duration{time.Millisecond, time.Millisecond},
		Runner:        mock,
		Stderr:        &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 calls (fail+retry+iter2), got %d", len(mock.Calls))
	}
}

func TestRunLoop_OnFailureRetry_ExhaustionIncrementsFailureCounter(t *testing.T) {
	// Each iteration exhausts retries → consecutive failures → MaxFailures stops loop
	mock := agent.NewMockRunner(
		// iter 1: fail x3
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		// iter 2: fail x3
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
		&agent.RunResult{ExitCode: 1},
	)
	cfg := Config{
		Content:       "test",
		Iterations:    10,
		OnFailure:     OnFailureRetry,
		Retries:       2,
		MaxFailures:   2,
		RetryBackoffs: []time.Duration{time.Millisecond, time.Millisecond},
		Runner:        mock,
		Stderr:        &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error after MaxFailures consecutive retry-exhausted iterations")
	}
	if !strings.Contains(err.Error(), "2 consecutive failures") {
		t.Errorf("error should mention '2 consecutive failures', got: %v", err)
	}
	if len(mock.Calls) != 6 {
		t.Errorf("expected 6 calls (3 per iter × 2 iters), got %d", len(mock.Calls))
	}
}
