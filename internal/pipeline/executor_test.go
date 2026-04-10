package pipeline_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/pipeline"
)

// --- test helpers ---

func okResult() *agent.RunResult { return &agent.RunResult{ExitCode: 0} }
func failResult() *agent.RunResult { return &agent.RunResult{ExitCode: 1} }

func baseExecCfg(runner agent.Runner) pipeline.ExecutorConfig {
	return pipeline.ExecutorConfig{
		Runner:        runner,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
		ForceCtx:      context.Background(),
		RetryBackoffs: []time.Duration{0, 0, 0},
	}
}

// minAgentNode returns a minimal loop-body agent node.
func minAgentNode(name string) pipeline.Node {
	return pipeline.Node{
		Name:  name,
		Kind:  pipeline.NodeKindAgent,
		Event: pipeline.EventLoopBody,
		Agent: &pipeline.AgentSpec{Prompt: "do work"},
	}
}

// execCmdNode returns a cmd node for an event with the given shell command.
func execCmdNode(name string, event pipeline.Event, command string) pipeline.Node {
	return pipeline.Node{
		Name:  name,
		Kind:  pipeline.NodeKindCmd,
		Event: event,
		Cmd:   &pipeline.CmdSpec{Command: command},
	}
}

// normalizedPipeline builds a pipeline with the given nodes and normalizes it.
// All pipelines must have exactly one loop-body agent node.
func normalizedPipeline(t *testing.T, iterations int, nodes ...pipeline.Node) *pipeline.Pipeline {
	t.Helper()
	p := &pipeline.Pipeline{Iterations: iterations, Nodes: nodes}
	if err := pipeline.Normalize(p); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return p
}

// appendCmdNode returns a cmd node that appends `label\n` to a file.
func appendCmdNode(name string, event pipeline.Event, label, file string) pipeline.Node {
	return execCmdNode(name, event, fmt.Sprintf(`printf '%s\n' >> %s`, label, file))
}

// --- tests ---

func TestExecutor_HappyPath_RunsAgentEachIteration(t *testing.T) {
	runner := agent.NewMockRunner(okResult(), okResult())
	p := normalizedPipeline(t, 2, minAgentNode("main"))

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(runner.Calls) != 2 {
		t.Errorf("expected 2 runner calls, got %d", len(runner.Calls))
	}
}

func TestExecutor_EventOrdering_AllEventsFire(t *testing.T) {
	dir := t.TempDir()
	orderFile := filepath.Join(dir, "order")

	runner := agent.NewMockRunner(okResult())
	p := normalizedPipeline(t, 1,
		appendCmdNode("rs", pipeline.EventRunStart, "run-start", orderFile),
		appendCmdNode("ls", pipeline.EventLoopStart, "loop-start", orderFile),
		// loop-body agent is required
		pipeline.Node{
			Name:     "lb",
			Kind:     pipeline.NodeKindAgent,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Agent:    &pipeline.AgentSpec{Prompt: "body"},
		},
		appendCmdNode("le", pipeline.EventLoopEnd, "loop-end", orderFile),
		appendCmdNode("re", pipeline.EventRunEnd, "run-end", orderFile),
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(orderFile)
	if err != nil {
		t.Fatalf("read order file: %v", err)
	}
	got := string(data)
	want := "run-start\nloop-start\nloop-end\nrun-end\n"
	if got != want {
		t.Errorf("event order:\ngot:  %q\nwant: %q", got, want)
	}
	if len(runner.Calls) != 1 {
		t.Errorf("expected 1 agent call, got %d", len(runner.Calls))
	}
}

func TestExecutor_EventOrdering_LoopEventsRepeatPerIteration(t *testing.T) {
	dir := t.TempDir()
	orderFile := filepath.Join(dir, "order")

	runner := agent.NewMockRunner(okResult(), okResult())
	p := normalizedPipeline(t, 2,
		appendCmdNode("ls", pipeline.EventLoopStart, "S", orderFile),
		pipeline.Node{
			Name:     "lb",
			Kind:     pipeline.NodeKindAgent,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Agent:    &pipeline.AgentSpec{Prompt: "body"},
		},
		appendCmdNode("le", pipeline.EventLoopEnd, "E", orderFile),
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, _ := os.ReadFile(orderFile)
	got := string(data)
	want := "S\nE\nS\nE\n" // 2 iterations: loop-start then loop-end each time
	if got != want {
		t.Errorf("loop event repeat:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestExecutor_ConditionalSkip_WhenFalseSkipsNode(t *testing.T) {
	dir := t.TempDir()
	touchFile := filepath.Join(dir, "touched")

	runner := agent.NewMockRunner(okResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:  "skip-me",
			Kind:  pipeline.NodeKindCmd,
			Event: pipeline.EventRunStart,
			When:  "false",
			Cmd:   &pipeline.CmdSpec{Command: fmt.Sprintf("touch %s", touchFile)},
		},
		minAgentNode("main"),
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(touchFile); !os.IsNotExist(err) {
		t.Error("expected node to be skipped (file should not exist)")
	}
}

func TestExecutor_ConditionalRun_WhenTrueRunsNode(t *testing.T) {
	dir := t.TempDir()
	touchFile := filepath.Join(dir, "touched")

	runner := agent.NewMockRunner(okResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:  "run-me",
			Kind:  pipeline.NodeKindCmd,
			Event: pipeline.EventRunStart,
			When:  "true",
			Cmd:   &pipeline.CmdSpec{Command: fmt.Sprintf("touch %s", touchFile)},
		},
		minAgentNode("main"),
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(touchFile); os.IsNotExist(err) {
		t.Error("expected node to run (file should exist)")
	}
}

func TestExecutor_FailurePolicyStop_ReturnsError(t *testing.T) {
	runner := agent.NewMockRunner(failResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:      "main",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventLoopBody,
			OnFailure: pipeline.FailurePolicyStop,
			Agent:     &pipeline.AgentSpec{Prompt: "do work"},
		},
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err == nil {
		t.Error("expected error from failing stop-policy node")
	}
}

func TestExecutor_FailurePolicyContinue_DoesNotReturnError(t *testing.T) {
	runner := agent.NewMockRunner(failResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:      "main",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventLoopBody,
			OnFailure: pipeline.FailurePolicyContinue,
			Agent:     &pipeline.AgentSpec{Prompt: "do work"},
		},
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Errorf("expected no error from continue-policy node, got: %v", err)
	}
}

func TestExecutor_FailurePolicyRetry_RetriesUntilSuccess(t *testing.T) {
	// fail twice, then succeed on the 3rd attempt
	runner := agent.NewMockRunner(failResult(), failResult(), okResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:      "main",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventLoopBody,
			OnFailure: pipeline.FailurePolicyRetry,
			Retries:   2,
			Agent:     &pipeline.AgentSpec{Prompt: "do work"},
		},
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(runner.Calls) != 3 {
		t.Errorf("expected 3 runner calls (2 failures + 1 success), got %d", len(runner.Calls))
	}
}

func TestExecutor_FailurePolicyRetry_ExhaustedReturnsError(t *testing.T) {
	// fail all 3 attempts
	runner := agent.NewMockRunner(failResult(), failResult(), failResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:      "main",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventLoopBody,
			OnFailure: pipeline.FailurePolicyRetry,
			Retries:   2,
			Agent:     &pipeline.AgentSpec{Prompt: "do work"},
		},
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err == nil {
		t.Error("expected error after retry exhaustion")
	}
	if len(runner.Calls) != 3 {
		t.Errorf("expected 3 runner calls, got %d", len(runner.Calls))
	}
}

func TestExecutor_CmdNode_ExecutesShellCommand(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out")

	runner := agent.NewMockRunner(okResult())
	p := normalizedPipeline(t, 1,
		execCmdNode("write", pipeline.EventRunStart, fmt.Sprintf(`echo hello > %s`, outFile)),
		minAgentNode("main"),
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", string(data))
	}
}

func TestExecutor_FailureEvent_FiresWhenNodeFails(t *testing.T) {
	dir := t.TempDir()
	touchFile := filepath.Join(dir, "failure-fired")

	// Main agent fails; failure event node should run
	runner := agent.NewMockRunner(failResult())
	p := &pipeline.Pipeline{
		Iterations: 1,
		Nodes: []pipeline.Node{
			{
				Name:      "main",
				Kind:      pipeline.NodeKindAgent,
				Event:     pipeline.EventLoopBody,
				OnFailure: pipeline.FailurePolicyStop,
				Agent:     &pipeline.AgentSpec{Prompt: "do work"},
			},
			{
				Name:     "on-fail",
				Kind:     pipeline.NodeKindCmd,
				Event:    pipeline.EventFailure,
				Parallel: true, // no implicit dep on main (different conceptual flow)
				Cmd:      &pipeline.CmdSpec{Command: fmt.Sprintf("touch %s", touchFile)},
			},
		},
	}
	if err := pipeline.Normalize(p); err != nil {
		t.Fatalf("normalize: %v", err)
	}

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	err := exec.Run()
	if err == nil {
		t.Error("expected error from failing stop-policy node")
	}

	if _, statErr := os.Stat(touchFile); os.IsNotExist(statErr) {
		t.Error("expected failure event node to have fired (touch file should exist)")
	}
}

func TestExecutor_AgentNode_PromptPassedToRunner(t *testing.T) {
	runner := agent.NewMockRunner(okResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:  "main",
			Kind:  pipeline.NodeKindAgent,
			Event: pipeline.EventLoopBody,
			Agent: &pipeline.AgentSpec{Prompt: "my specific prompt"},
		},
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 runner call")
	}
	if runner.Calls[0].Prompt != "my specific prompt" {
		t.Errorf("expected prompt %q, got %q", "my specific prompt", runner.Calls[0].Prompt)
	}
}

func TestExecutor_CmdNode_EnvVarsAvailable(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "env-out")

	runner := agent.NewMockRunner(okResult())
	p := normalizedPipeline(t, 1,
		execCmdNode("check-env", pipeline.EventRunStart,
			fmt.Sprintf(`echo "$JUGGLE_ITERATION" > %s`, outFile)),
		minAgentNode("main"),
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, _ := os.ReadFile(outFile)
	// run-start is iteration 0
	if string(data) != "0\n" {
		t.Errorf("expected JUGGLE_ITERATION=0 for run-start, got %q", string(data))
	}
}

func TestExecutor_cmdNodeTimeout(t *testing.T) {
	runner := agent.NewMockRunner(okResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:    "slow",
			Kind:    pipeline.NodeKindCmd,
			Event:   pipeline.EventRunStart,
			Timeout: 100 * time.Millisecond,
			Cmd:     &pipeline.CmdSpec{Command: "sleep 10"},
		},
		minAgentNode("main"),
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	err := exec.Run()
	if err == nil {
		t.Fatal("expected error from timed-out cmd node")
	}
}

// --- parallel execution tests ---

func TestExecutor_Parallel_BothNodesComplete(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a")
	fileB := filepath.Join(dir, "b")

	runner := agent.NewMockRunner(okResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:     "main",
			Kind:     pipeline.NodeKindAgent,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Agent:    &pipeline.AgentSpec{Prompt: "body"},
		},
		pipeline.Node{
			Name:     "side-a",
			Kind:     pipeline.NodeKindCmd,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Cmd:      &pipeline.CmdSpec{Command: fmt.Sprintf("touch %s", fileA)},
		},
		pipeline.Node{
			Name:     "side-b",
			Kind:     pipeline.NodeKindCmd,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Cmd:      &pipeline.CmdSpec{Command: fmt.Sprintf("touch %s", fileB)},
		},
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(fileA); os.IsNotExist(err) {
		t.Error("node 'side-a' did not run")
	}
	if _, err := os.Stat(fileB); os.IsNotExist(err) {
		t.Error("node 'side-b' did not run")
	}
}

func TestExecutor_Parallel_DependentNodeRunsAfterDep(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a")
	fileC := filepath.Join(dir, "c")

	runner := agent.NewMockRunner(okResult())
	// node-a creates fileA; node-c (After=[node-a]) verifies fileA exists then creates fileC.
	// If node-c ran before node-a, fileA would not exist and [ -f ] exits 1, so fileC is not created.
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:     "main",
			Kind:     pipeline.NodeKindAgent,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Agent:    &pipeline.AgentSpec{Prompt: "body"},
		},
		pipeline.Node{
			Name:     "node-a",
			Kind:     pipeline.NodeKindCmd,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Cmd:      &pipeline.CmdSpec{Command: fmt.Sprintf("touch %s", fileA)},
		},
		pipeline.Node{
			Name:     "node-c",
			Kind:     pipeline.NodeKindCmd,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			After:    []string{"node-a"},
			Cmd:      &pipeline.CmdSpec{Command: fmt.Sprintf("[ -f %s ] && touch %s", fileA, fileC)},
		},
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(fileA); os.IsNotExist(err) {
		t.Error("node 'node-a' did not run")
	}
	if _, err := os.Stat(fileC); os.IsNotExist(err) {
		t.Error("node 'node-c' did not run or ran before 'node-a'")
	}
}

func TestExecutor_Parallel_StopFailurePropagatesError(t *testing.T) {
	runner := agent.NewMockRunner(failResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:      "main",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventLoopBody,
			Parallel:  true,
			OnFailure: pipeline.FailurePolicyStop,
			Agent:     &pipeline.AgentSpec{Prompt: "body"},
		},
		pipeline.Node{
			Name:     "side",
			Kind:     pipeline.NodeKindCmd,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Cmd:      &pipeline.CmdSpec{Command: "true"},
		},
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err == nil {
		t.Error("expected error from failing stop-policy parallel node")
	}
}

func TestExecutor_Parallel_ContinueFailureAllowsSiblings(t *testing.T) {
	dir := t.TempDir()
	sideFile := filepath.Join(dir, "side")

	runner := agent.NewMockRunner(failResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:      "main",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventLoopBody,
			Parallel:  true,
			OnFailure: pipeline.FailurePolicyContinue,
			Agent:     &pipeline.AgentSpec{Prompt: "body"},
		},
		pipeline.Node{
			Name:     "side",
			Kind:     pipeline.NodeKindCmd,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Cmd:      &pipeline.CmdSpec{Command: fmt.Sprintf("touch %s", sideFile)},
		},
	)

	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Errorf("expected no error from continue-policy failure, got: %v", err)
	}
	if _, err := os.Stat(sideFile); os.IsNotExist(err) {
		t.Error("side node should have run despite main failing with continue policy")
	}
}

func TestExecutor_Parallel_MaxParallelSteps_Respected(t *testing.T) {
	// With MaxParallelSteps=1, two 50ms sleep nodes run sequentially: total ≥90ms.
	runner := agent.NewMockRunner(okResult())
	p := &pipeline.Pipeline{
		Iterations:       1,
		MaxParallelSteps: 1,
		Nodes: []pipeline.Node{
			{Name: "main", Kind: pipeline.NodeKindAgent, Event: pipeline.EventLoopBody, Parallel: true,
				Agent: &pipeline.AgentSpec{Prompt: "body"}},
			{Name: "sleep1", Kind: pipeline.NodeKindCmd, Event: pipeline.EventLoopBody, Parallel: true,
				Cmd: &pipeline.CmdSpec{Command: "sleep 0.05"}},
			{Name: "sleep2", Kind: pipeline.NodeKindCmd, Event: pipeline.EventLoopBody, Parallel: true,
				Cmd: &pipeline.CmdSpec{Command: "sleep 0.05"}},
		},
	}
	if err := pipeline.Normalize(p); err != nil {
		t.Fatalf("normalize: %v", err)
	}

	start := time.Now()
	exec := pipeline.NewExecutor(p, baseExecCfg(runner))
	if err := exec.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 90*time.Millisecond {
		t.Errorf("expected ≥90ms with MaxParallelSteps=1 for two 50ms nodes, got %v", elapsed)
	}
}

func TestExecutor_Parallel_ShutdownCancelsRunning(t *testing.T) {
	shutdown := make(chan struct{})

	runner := agent.NewMockRunner(okResult())
	p := normalizedPipeline(t, 1,
		pipeline.Node{
			Name:      "slow",
			Kind:      pipeline.NodeKindCmd,
			Event:     pipeline.EventLoopBody,
			Parallel:  true,
			OnFailure: pipeline.FailurePolicyStop,
			Cmd:       &pipeline.CmdSpec{Command: "sleep 30"},
		},
		pipeline.Node{
			Name:     "main",
			Kind:     pipeline.NodeKindAgent,
			Event:    pipeline.EventLoopBody,
			Parallel: true,
			Agent:    &pipeline.AgentSpec{Prompt: "body"},
		},
	)

	cfg := baseExecCfg(runner)
	cfg.Shutdown = shutdown

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(shutdown)
	}()

	start := time.Now()
	exec := pipeline.NewExecutor(p, cfg)
	err := exec.Run()
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error from shutdown cancellation")
	}
	if elapsed > 2*time.Second {
		t.Errorf("expected cancellation within 2 seconds, took %v", elapsed)
	}
}
