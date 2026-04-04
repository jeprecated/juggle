package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ohare93/juggle/internal/agent"
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
