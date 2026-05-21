package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jeprecated/juggle/internal/agent"
)

// TestRunLoop_PipelineGateOff_OldPath verifies the old path is used when env var is unset.
// The old path error format is "iteration N failed (exit code N)".
func TestRunLoop_PipelineGateOff_OldPath(t *testing.T) {
	t.Setenv("JUGGLE_USE_PIPELINE", "")
	mock := agent.NewMockRunner(&agent.RunResult{ExitCode: 1})
	cfg := Config{
		Content:    "do work",
		Iterations: 1,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error from failed iteration")
	}
	if !strings.Contains(err.Error(), "iteration 1 failed") {
		t.Errorf("expected old-path error format, got: %v", err)
	}
}

// TestRunLoop_PipelineGateOn_RunsViaPipeline verifies the pipeline executor is used
// when JUGGLE_USE_PIPELINE=1. The pipeline error format is "node \"main\": exit code N".
func TestRunLoop_PipelineGateOn_RunsViaPipeline(t *testing.T) {
	t.Setenv("JUGGLE_USE_PIPELINE", "1")
	mock := agent.NewMockRunner(&agent.RunResult{ExitCode: 1})
	cfg := Config{
		Content:    "do work",
		Iterations: 1,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error from failed pipeline node")
	}
	if !strings.Contains(err.Error(), "node") {
		t.Errorf("expected pipeline-path error format (node ...), got: %v", err)
	}
}

// TestRunLoop_PipelineGateOn_MultipleIterations verifies N agent calls for N iterations.
func TestRunLoop_PipelineGateOn_MultipleIterations(t *testing.T) {
	t.Setenv("JUGGLE_USE_PIPELINE", "1")
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 0},
		&agent.RunResult{ExitCode: 0},
		&agent.RunResult{ExitCode: 0},
	)
	cfg := Config{
		Content:    "do work",
		Iterations: 3,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Errorf("expected 3 runner calls, got %d", len(mock.Calls))
	}
}

// TestRunLoop_PipelineGateOn_OtherEnvValues verifies non-"1" values do not activate the gate.
func TestRunLoop_PipelineGateOn_OtherEnvValues(t *testing.T) {
	for _, val := range []string{"true", "yes", "on", "TRUE"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv("JUGGLE_USE_PIPELINE", val)
			mock := agent.NewMockRunner(&agent.RunResult{ExitCode: 1})
			cfg := Config{
				Content:    "do work",
				Iterations: 1,
				Runner:     mock,
				Stderr:     &bytes.Buffer{},
			}
			err := RunLoop(cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			// Old path error format — gate must NOT have activated
			if !strings.Contains(err.Error(), "iteration 1 failed") {
				t.Errorf("val=%q: expected old-path error, got: %v", val, err)
			}
		})
	}
}

// TestRunLoop_PipelineGateOn_HooksRunViaExecutor verifies that lifecycle hooks
// are executed through the pipeline executor when the gate is active.
func TestRunLoop_PipelineGateOn_HooksRunViaExecutor(t *testing.T) {
	t.Setenv("JUGGLE_USE_PIPELINE", "1")

	// Both the hook agent and the main agent will call the runner.
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 0}, // agent-pre
		&agent.RunResult{ExitCode: 0}, // main
	)
	cfg := Config{
		Content:    "do work",
		Iterations: 1,
		AgentPre:   "setup the environment",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// agent-pre + main = 2 calls
	if len(mock.Calls) != 2 {
		t.Errorf("expected 2 runner calls (agent-pre + main), got %d", len(mock.Calls))
	}
}
