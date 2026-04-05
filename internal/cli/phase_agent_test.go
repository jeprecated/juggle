package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
)

// --- agent-pre ---

func TestRunLoop_AgentPre_RunsOnceBeforeIterations(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "pre done"},
		&agent.RunResult{Output: "iter1"},
		&agent.RunResult{Output: "iter2"},
	)
	cfg := Config{
		Content:    "main task",
		Iterations: 2,
		AgentPre:   "setup prompt",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Fatalf("expected 3 calls (pre + 2 iterations), got %d", len(mock.Calls))
	}
	if !strings.Contains(mock.Calls[0].Prompt, "setup prompt") {
		t.Errorf("call[0] should be agent-pre, got prompt: %q", mock.Calls[0].Prompt)
	}
	if !strings.Contains(mock.Calls[1].Prompt, "main task") {
		t.Errorf("call[1] should be main iteration, got prompt: %q", mock.Calls[1].Prompt)
	}
}

func TestRunLoop_AgentPre_FailureStopsRun(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1, Output: "pre failed"},
		&agent.RunResult{Output: "should not run"},
	)
	cfg := Config{
		Content:    "main task",
		Iterations: 1,
		AgentPre:   "setup prompt",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error when agent-pre fails")
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected only 1 call (pre), got %d", len(mock.Calls))
	}
}

// --- agent-post ---

func TestRunLoop_AgentPost_RunsOnceAfterIterations(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "iter1"},
		&agent.RunResult{Output: "iter2"},
		&agent.RunResult{Output: "post done"},
	)
	cfg := Config{
		Content:    "main task",
		Iterations: 2,
		AgentPost:  "summarize prompt",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 3 {
		t.Fatalf("expected 3 calls (2 iterations + post), got %d", len(mock.Calls))
	}
	if !strings.Contains(mock.Calls[2].Prompt, "summarize prompt") {
		t.Errorf("call[2] should be agent-post, got prompt: %q", mock.Calls[2].Prompt)
	}
}

func TestRunLoop_AgentPost_FailureStopsRun(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "iter1"},
		&agent.RunResult{ExitCode: 1, Output: "post failed"},
	)
	cfg := Config{
		Content:    "main task",
		Iterations: 1,
		AgentPost:  "summarize prompt",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error when agent-post fails")
	}
}

// --- agent-before ---

func TestRunLoop_AgentBefore_RunsBeforeEachIteration(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "before1"},
		&agent.RunResult{Output: "main1"},
		&agent.RunResult{Output: "before2"},
		&agent.RunResult{Output: "main2"},
	)
	cfg := Config{
		Content:     "main task",
		Iterations:  2,
		AgentBefore: "check prompt",
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 4 {
		t.Fatalf("expected 4 calls (before+main × 2), got %d", len(mock.Calls))
	}
	if !strings.Contains(mock.Calls[0].Prompt, "check prompt") {
		t.Errorf("call[0] should be agent-before, got: %q", mock.Calls[0].Prompt)
	}
	if !strings.Contains(mock.Calls[1].Prompt, "main task") {
		t.Errorf("call[1] should be main iteration, got: %q", mock.Calls[1].Prompt)
	}
	if !strings.Contains(mock.Calls[2].Prompt, "check prompt") {
		t.Errorf("call[2] should be agent-before, got: %q", mock.Calls[2].Prompt)
	}
}

func TestRunLoop_AgentBefore_FailureSkipsMainIteration(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{ExitCode: 1, Output: "before failed"},
		&agent.RunResult{Output: "before2 ok"},
		&agent.RunResult{Output: "main2"},
	)
	cfg := Config{
		Content:     "main task",
		Iterations:  2,
		AgentBefore: "check prompt",
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// iter1: before fails → skip main → iter2: before ok → main runs
	if len(mock.Calls) != 3 {
		t.Fatalf("expected 3 calls (before-fail, before-ok, main), got %d", len(mock.Calls))
	}
	if !strings.Contains(mock.Calls[2].Prompt, "main task") {
		t.Errorf("call[2] should be main iteration, got: %q", mock.Calls[2].Prompt)
	}
}

// --- agent-after ---

func TestRunLoop_AgentAfter_RunsAfterEachIteration(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "main1"},
		&agent.RunResult{Output: "after1"},
		&agent.RunResult{Output: "main2"},
		&agent.RunResult{Output: "after2"},
	)
	cfg := Config{
		Content:    "main task",
		Iterations: 2,
		AgentAfter: "tidy prompt",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 4 {
		t.Fatalf("expected 4 calls (main+after × 2), got %d", len(mock.Calls))
	}
	if !strings.Contains(mock.Calls[1].Prompt, "tidy prompt") {
		t.Errorf("call[1] should be agent-after, got: %q", mock.Calls[1].Prompt)
	}
	if !strings.Contains(mock.Calls[3].Prompt, "tidy prompt") {
		t.Errorf("call[3] should be agent-after, got: %q", mock.Calls[3].Prompt)
	}
}

func TestRunLoop_AgentAfter_FailureLogsWarningAndContinues(t *testing.T) {
	var stderr bytes.Buffer
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "main1"},
		&agent.RunResult{ExitCode: 1, Output: "after failed"},
		&agent.RunResult{Output: "main2"},
		&agent.RunResult{Output: "after2"},
	)
	cfg := Config{
		Content:    "main task",
		Iterations: 2,
		AgentAfter: "tidy prompt",
		Runner:     mock,
		Stderr:     &stderr,
	}
	err := RunLoop(cfg)
	if err != nil {
		t.Fatalf("agent-after failure should not stop loop, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "agent-after") {
		t.Errorf("expected warning about agent-after failure in stderr, got: %q", stderr.String())
	}
	if len(mock.Calls) != 4 {
		t.Fatalf("expected 4 calls (all iterations continued), got %d", len(mock.Calls))
	}
}

// --- env vars ---

func TestRunLoop_PhaseAgent_ReceivesJugglePhaseEnv(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "pre"},
		&agent.RunResult{Output: "before"},
		&agent.RunResult{Output: "main"},
		&agent.RunResult{Output: "after"},
		&agent.RunResult{Output: "post"},
	)
	cfg := Config{
		Content:     "main task",
		Iterations:  1,
		AgentPre:    "pre prompt",
		AgentBefore: "before prompt",
		AgentAfter:  "after prompt",
		AgentPost:   "post prompt",
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 5 {
		t.Fatalf("expected 5 calls, got %d", len(mock.Calls))
	}
	checkEnv := func(idx int, wantPhase string) {
		t.Helper()
		for _, e := range mock.Calls[idx].Env {
			if e == "JUGGLE_PHASE="+wantPhase {
				return
			}
		}
		t.Errorf("call[%d] missing JUGGLE_PHASE=%s, env: %v", idx, wantPhase, mock.Calls[idx].Env)
	}
	checkEnv(0, "pre")
	checkEnv(1, "before")
	// call[2] is main — no phase env required
	checkEnv(3, "after")
	checkEnv(4, "post")
}

func TestRunLoop_PhaseAgent_ReceivesJuggleIterationEnv(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "before"},
		&agent.RunResult{Output: "main"},
		&agent.RunResult{Output: "after"},
	)
	cfg := Config{
		Content:     "main task",
		Iterations:  1,
		AgentBefore: "before prompt",
		AgentAfter:  "after prompt",
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checkEnv := func(idx int, wantVar string) {
		t.Helper()
		for _, e := range mock.Calls[idx].Env {
			if strings.HasPrefix(e, wantVar+"=") {
				return
			}
		}
		t.Errorf("call[%d] missing %s env var, env: %v", idx, wantVar, mock.Calls[idx].Env)
	}
	checkEnv(0, "JUGGLE_ITERATION")
	checkEnv(2, "JUGGLE_ITERATION")
}

func TestRunLoop_PhaseAgent_ReceivesRunLevelEnvVars(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "pre"},
		&agent.RunResult{Output: "before"},
		&agent.RunResult{Output: "main"},
		&agent.RunResult{Output: "after"},
		&agent.RunResult{Output: "post"},
	)
	cfg := Config{
		Content:     "main task",
		Iterations:  1,
		AgentPre:    "pre prompt",
		AgentBefore: "before prompt",
		AgentAfter:  "after prompt",
		AgentPost:   "post prompt",
		RunID:       "test-run-id",
		Model:       "test-model",
		Provider:    "test-provider",
		Label:       "test-label",
		Runner:      mock,
		Stderr:      &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 5 {
		t.Fatalf("expected 5 calls, got %d", len(mock.Calls))
	}
	checkHasVar := func(idx int, name, value string) {
		t.Helper()
		want := name + "=" + value
		for _, e := range mock.Calls[idx].Env {
			if e == want {
				return
			}
		}
		t.Errorf("call[%d] missing %s, env: %v", idx, want, mock.Calls[idx].Env)
	}
	// call[0]=pre, call[1]=before, call[2]=main, call[3]=after, call[4]=post
	for _, idx := range []int{0, 1, 3, 4} {
		checkHasVar(idx, "JUGGLE_RUN_ID", "test-run-id")
		checkHasVar(idx, "JUGGLE_MODEL", "test-model")
		checkHasVar(idx, "JUGGLE_PROVIDER", "test-provider")
		checkHasVar(idx, "JUGGLE_LABEL", "test-label")
	}
}

func TestPhaseEnv_EnvSlice_NoLabelOmitsLabelVar(t *testing.T) {
	env := phaseEnv{phase: "pre", iteration: 0, maxIter: 1, runID: "r", model: "m", provider: "p", label: ""}
	for _, e := range env.envSlice() {
		if strings.HasPrefix(e, "JUGGLE_LABEL=") {
			t.Errorf("JUGGLE_LABEL should not be set when label is empty, got: %q", e)
		}
	}
}

func TestPhaseEnv_EnvSlice_LabelIncludedWhenSet(t *testing.T) {
	env := phaseEnv{phase: "pre", iteration: 0, maxIter: 1, runID: "r", model: "m", provider: "p", label: "my-label"}
	found := false
	for _, e := range env.envSlice() {
		if e == "JUGGLE_LABEL=my-label" {
			found = true
		}
	}
	if !found {
		t.Error("expected JUGGLE_LABEL=my-label in env slice")
	}
}

// --- BuildPhaseContent (multi-value merging helper) ---

func TestBuildPhaseContent_JoinsResolvedValues(t *testing.T) {
	result, err := BuildPhaseContent([]string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello\n\nworld" {
		t.Errorf("expected 'hello\\n\\nworld', got %q", result)
	}
}

func TestBuildPhaseContent_EmptySliceReturnsEmpty(t *testing.T) {
	result, err := BuildPhaseContent(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestBuildPhaseContent_SingleValueNoJoin(t *testing.T) {
	result, err := BuildPhaseContent([]string{"only value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "only value" {
		t.Errorf("expected 'only value', got %q", result)
	}
}

// --- lifecycle headers ---

func TestRunLoop_AgentPre_PrintsHeader(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "pre done"},
		&agent.RunResult{Output: "main done"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "main task",
		Iterations: 1,
		AgentPre:   "setup prompt",
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Agent Pre") {
		t.Errorf("expected 'Agent Pre' header in stderr, got: %q", stderr.String())
	}
}

func TestRunLoop_AgentBefore_PrintsHeader(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "before done"},
		&agent.RunResult{Output: "main done"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:     "main task",
		Iterations:  1,
		AgentBefore: "check prompt",
		Runner:      mock,
		Stderr:      &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Agent Before") {
		t.Errorf("expected 'Agent Before' header in stderr, got: %q", stderr.String())
	}
}

func TestRunLoop_AgentAfter_PrintsHeader(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "main done"},
		&agent.RunResult{Output: "after done"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "main task",
		Iterations: 1,
		AgentAfter: "tidy prompt",
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Agent After") {
		t.Errorf("expected 'Agent After' header in stderr, got: %q", stderr.String())
	}
}

func TestRunLoop_AgentPost_PrintsHeader(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "main done"},
		&agent.RunResult{Output: "post done"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "main task",
		Iterations: 1,
		AgentPost:  "summarize prompt",
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Agent Post") {
		t.Errorf("expected 'Agent Post' header in stderr, got: %q", stderr.String())
	}
}

// --- shutdown interrupt ---

// blockingRunner blocks until its RunOptions.Context is cancelled.
type blockingRunner struct {
	called chan struct{}
}

func (b *blockingRunner) Run(opts agent.RunOptions) (*agent.RunResult, error) {
	close(b.called)
	if opts.Context != nil {
		<-opts.Context.Done()
	}
	return nil, fmt.Errorf("context cancelled: %w", opts.Context.Err())
}

func TestRunPhaseAgent_ShutdownInterrupts(t *testing.T) {
	shutdown := make(chan struct{})
	runner := &blockingRunner{called: make(chan struct{})}

	cfg := Config{
		Runner:   runner,
		Shutdown: shutdown,
		ForceCtx: context.Background(),
		Stderr:   &bytes.Buffer{},
	}
	env := phaseEnv{phase: "pre", runID: "test"}

	errCh := make(chan error, 1)
	go func() {
		errCh <- runPhaseAgent(cfg, "test prompt", env, cfg.Stderr)
	}()

	<-runner.called
	close(shutdown)

	err := <-errCh
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
}

func TestRunPhaseAgent_NormalCompletionWithShutdown(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "done"})
	cfg := Config{
		Runner:   mock,
		Shutdown: make(chan struct{}),
		ForceCtx: context.Background(),
		Stderr:   &bytes.Buffer{},
	}
	env := phaseEnv{phase: "before", runID: "test"}

	err := runPhaseAgent(cfg, "test prompt", env, cfg.Stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.Calls))
	}
}

func TestRunPhaseAgent_NilShutdownStillWorks(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "done"})
	cfg := Config{
		Runner: mock,
		Stderr: &bytes.Buffer{},
	}
	env := phaseEnv{phase: "post", runID: "test"}

	err := runPhaseAgent(cfg, "test prompt", env, cfg.Stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunLoop_AgentPre_VerbosePrintsPrompt(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "pre done"},
		&agent.RunResult{Output: "main done"},
	)
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "main task",
		Iterations: 1,
		AgentPre:   "my setup prompt",
		Verbose:    true,
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "my setup prompt") {
		t.Errorf("expected prompt text in verbose stderr, got: %q", stderr.String())
	}
}
