package cli

import (
	"bytes"
	"errors"
	"fmt"
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

func TestRunLoop_TrustMode(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{Content: "test", Iterations: 1, Trust: true, Runner: mock, Stderr: &bytes.Buffer{}}
	RunLoop(cfg)
	if mock.Calls[0].Permission != agent.PermissionBypass {
		t.Errorf("expected bypass, got %s", mock.Calls[0].Permission)
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
		"--watch",
		"--trust",
	}
	for _, want := range checks {
		if !strings.Contains(rootCmd.Example, want) {
			t.Errorf("rootCmd.Example missing %q", want)
		}
	}
}

func TestHelpLongContainsCompletion(t *testing.T) {
	if !strings.Contains(rootCmd.Long, "completion") {
		t.Error("Long description should mention shell completion")
	}
}

func TestSetVersionUpdatesRootCmd(t *testing.T) {
	prev := rootCmd.Version
	defer func() { rootCmd.Version = prev; version = prev }()

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
