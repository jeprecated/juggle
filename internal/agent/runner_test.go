package agent

import (
	"testing"
	"time"
)

func TestMockRunner_Run(t *testing.T) {
	t.Run("returns queued responses in order", func(t *testing.T) {
		mock := NewMockRunner(
			&RunResult{Output: "first"},
			&RunResult{Output: "second", ExitCode: 1},
		)

		// First call
		result, err := mock.Run(RunOptions{Prompt: "prompt1", Permission: PermissionAcceptEdits})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Output != "first" {
			t.Errorf("expected output 'first', got '%s'", result.Output)
		}

		// Second call
		result, err = mock.Run(RunOptions{Prompt: "prompt2", Permission: PermissionBypass})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Output != "second" {
			t.Errorf("expected output 'second', got '%s'", result.Output)
		}
		if result.ExitCode != 1 {
			t.Errorf("expected ExitCode=1, got %d", result.ExitCode)
		}
	})

	t.Run("records all calls", func(t *testing.T) {
		mock := NewMockRunner(&RunResult{Output: "ok"})

		mock.Run(RunOptions{Prompt: "prompt1", Permission: PermissionAcceptEdits})
		mock.Run(RunOptions{Prompt: "prompt2", Permission: PermissionBypass})
		mock.Run(RunOptions{Prompt: "prompt3", Permission: PermissionAcceptEdits})

		if len(mock.Calls) != 3 {
			t.Fatalf("expected 3 calls, got %d", len(mock.Calls))
		}

		if mock.Calls[0].Prompt != "prompt1" {
			t.Errorf("expected first prompt 'prompt1', got '%s'", mock.Calls[0].Prompt)
		}
		if mock.Calls[0].Permission != PermissionAcceptEdits {
			t.Error("expected first call Permission=PermissionAcceptEdits")
		}

		if mock.Calls[1].Prompt != "prompt2" {
			t.Errorf("expected second prompt 'prompt2', got '%s'", mock.Calls[1].Prompt)
		}
		if mock.Calls[1].Permission != PermissionBypass {
			t.Error("expected second call Permission=PermissionBypass")
		}
	})

	t.Run("returns default error when exhausted", func(t *testing.T) {
		mock := NewMockRunner(&RunResult{Output: "only one"})

		// First call succeeds
		mock.Run(RunOptions{Prompt: "first"})

		// Second call should return default error result
		result, err := mock.Run(RunOptions{Prompt: "second"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ExitCode != 1 {
			t.Errorf("expected ExitCode=1 when exhausted, got %d", result.ExitCode)
		}
		if result.Output != "No more mock responses" {
			t.Errorf("expected output 'No more mock responses', got '%s'", result.Output)
		}
	})

	t.Run("Reset clears calls and index", func(t *testing.T) {
		mock := NewMockRunner(
			&RunResult{Output: "first"},
			&RunResult{Output: "second"},
		)

		mock.Run(RunOptions{Prompt: "prompt1"})
		mock.Run(RunOptions{Prompt: "prompt2"})

		mock.Reset()

		if len(mock.Calls) != 0 {
			t.Errorf("expected 0 calls after reset, got %d", len(mock.Calls))
		}
		if mock.NextIndex != 0 {
			t.Errorf("expected NextIndex=0 after reset, got %d", mock.NextIndex)
		}

		// Should return first response again
		result, _ := mock.Run(RunOptions{Prompt: "new prompt"})
		if result.Output != "first" {
			t.Errorf("expected 'first' after reset, got '%s'", result.Output)
		}
	})

	t.Run("SetResponses replaces queue", func(t *testing.T) {
		mock := NewMockRunner(&RunResult{Output: "old"})

		mock.Run(RunOptions{Prompt: "prompt"}) // Consume old response

		mock.SetResponses(&RunResult{Output: "new1"}, &RunResult{Output: "new2"})

		result, _ := mock.Run(RunOptions{Prompt: "prompt"})
		if result.Output != "new1" {
			t.Errorf("expected 'new1', got '%s'", result.Output)
		}

		result, _ = mock.Run(RunOptions{Prompt: "prompt"})
		if result.Output != "new2" {
			t.Errorf("expected 'new2', got '%s'", result.Output)
		}
	})

	t.Run("records timeout in call", func(t *testing.T) {
		mock := NewMockRunner(&RunResult{Output: "ok"})

		timeout := 5 * time.Minute
		mock.Run(RunOptions{Prompt: "prompt", Timeout: timeout})

		if len(mock.Calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.Calls))
		}

		if mock.Calls[0].Timeout != timeout {
			t.Errorf("expected timeout %v, got %v", timeout, mock.Calls[0].Timeout)
		}
	})

	t.Run("returns timed out result", func(t *testing.T) {
		mock := NewMockRunner(&RunResult{TimedOut: true, Output: "partial output before timeout"})

		result, err := mock.Run(RunOptions{Prompt: "prompt", Timeout: 5 * time.Minute})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.TimedOut {
			t.Error("expected TimedOut=true")
		}
		if result.Output != "partial output before timeout" {
			t.Errorf("expected 'partial output before timeout', got '%s'", result.Output)
		}
	})

	t.Run("records mode in call", func(t *testing.T) {
		mock := NewMockRunner(&RunResult{Output: "ok"})

		mock.Run(RunOptions{Prompt: "prompt", Mode: ModeInteractive})

		if len(mock.Calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.Calls))
		}

		if mock.Calls[0].Mode != ModeInteractive {
			t.Errorf("expected Mode=ModeInteractive, got %s", mock.Calls[0].Mode)
		}
	})

	t.Run("records permission in call", func(t *testing.T) {
		mock := NewMockRunner(&RunResult{Output: "ok"})

		mock.Run(RunOptions{Prompt: "prompt", Permission: PermissionPlan})

		if len(mock.Calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.Calls))
		}

		if mock.Calls[0].Permission != PermissionPlan {
			t.Errorf("expected Permission=PermissionPlan, got %s", mock.Calls[0].Permission)
		}
	})

	t.Run("records system prompt in call", func(t *testing.T) {
		mock := NewMockRunner(&RunResult{Output: "ok"})

		mock.Run(RunOptions{Prompt: "prompt", SystemPrompt: "You are a helpful assistant"})

		if len(mock.Calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.Calls))
		}

		if mock.Calls[0].SystemPrompt != "You are a helpful assistant" {
			t.Errorf("expected SystemPrompt='You are a helpful assistant', got '%s'", mock.Calls[0].SystemPrompt)
		}
	})
}

func TestRunOptions_Modes(t *testing.T) {
	t.Run("ModeHeadless is default", func(t *testing.T) {
		opts := RunOptions{Prompt: "test"}
		if opts.Mode != "" && opts.Mode != ModeHeadless {
			// Empty string should be treated as headless
			t.Errorf("expected Mode to be empty or ModeHeadless, got %s", opts.Mode)
		}
	})

	t.Run("PermissionAcceptEdits is common default", func(t *testing.T) {
		opts := RunOptions{Prompt: "test", Permission: PermissionAcceptEdits}
		if opts.Permission != PermissionAcceptEdits {
			t.Errorf("expected Permission=PermissionAcceptEdits, got %s", opts.Permission)
		}
	})

	t.Run("PermissionPlan for refine mode", func(t *testing.T) {
		opts := RunOptions{Prompt: "test", Permission: PermissionPlan}
		if opts.Permission != PermissionPlan {
			t.Errorf("expected Permission=PermissionPlan, got %s", opts.Permission)
		}
	})

	t.Run("PermissionBypass for trust mode", func(t *testing.T) {
		opts := RunOptions{Prompt: "test", Permission: PermissionBypass}
		if opts.Permission != PermissionBypass {
			t.Errorf("expected Permission=PermissionBypass, got %s", opts.Permission)
		}
	})
}

func TestMockRunner_RateLimited(t *testing.T) {
	t.Run("returns rate limited result", func(t *testing.T) {
		mock := NewMockRunner(&RunResult{
			Output:      "Rate limit exceeded",
			RateLimited: true,
			RetryAfter:  30 * time.Second,
		})

		result, err := mock.Run(RunOptions{Prompt: "prompt"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.RateLimited {
			t.Error("expected RateLimited=true")
		}
		if result.RetryAfter != 30*time.Second {
			t.Errorf("expected RetryAfter=30s, got %v", result.RetryAfter)
		}
	})
}
