package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/agent/provider"
)

// NodeResult holds the execution outcome of a node, used to populate WhenContext
// for subsequent nodes.
type NodeResult struct {
	Skipped  bool
	Success  bool
	ExitCode int
}

func nodeResultFromErr(err error) NodeResult {
	if err == nil {
		return NodeResult{Success: true, ExitCode: 0}
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return NodeResult{Success: false, ExitCode: ee.ExitCode()}
	}
	return NodeResult{Success: false, ExitCode: 1}
}

// ExecutorConfig holds the dependencies and runtime settings for a pipeline execution.
type ExecutorConfig struct {
	// Runner is the default agent runner used when no RunnerFactory is set or
	// when a node has no provider specified.
	Runner agent.Runner
	// RunnerFactory resolves a Runner for a given provider name. When set, it is
	// called for any agent node that has a non-empty Provider field (after
	// pipeline defaults have been applied). If nil, Runner is used for all nodes.
	RunnerFactory func(providerName string) (agent.Runner, error)
	Stdout        io.Writer
	Stderr        io.Writer
	ForceCtx      context.Context
	Shutdown      <-chan struct{}
	WorkDir       string
	RunID         string
	Label         string
	RetryBackoffs []time.Duration // override retry backoffs (default: 5s, 15s, 30s)
	SessionID     string          // effective session ID for trigger inbox (empty = no session)
	WakeCh        <-chan struct{} // wake signal channel (nil = no wake checking)
	ReadTrigger   func(sessionID string) (string, error) // reads and consumes trigger message from inbox
	FormatTrigger func(message string) string              // wraps trigger message in XML tags
}

// Executor runs a validated pipeline end-to-end.
type Executor struct {
	pipeline *Pipeline
	cfg      ExecutorConfig
}

// NewExecutor creates an Executor for a validated pipeline.
func NewExecutor(p *Pipeline, cfg ExecutorConfig) *Executor {
	return &Executor{pipeline: p, cfg: cfg}
}

// forceCtx returns ForceCtx if set, otherwise context.Background().
func (e *Executor) forceCtx() context.Context {
	if e.cfg.ForceCtx != nil {
		return e.cfg.ForceCtx
	}
	return context.Background()
}

// Run executes the pipeline from run-start through run-end.
// It fires failure-event nodes if a stop-policy node fails.
func (e *Executor) Run() error {
	iterations := e.pipeline.Iterations
	if iterations == 0 {
		iterations = 1
	}

	if err := e.runEvent(EventRunStart, 0); err != nil {
		_ = e.runEvent(EventFailure, 0)
		return err
	}

	for i := 1; i <= iterations; i++ {
		if err := e.checkShutdown(); err != nil {
			return err
		}

		if err := e.runEvent(EventLoopStart, i); err != nil {
			_ = e.runEvent(EventFailure, i)
			return err
		}

		if err := e.runEvent(EventLoopBody, i); err != nil {
			_ = e.runEvent(EventFailure, i)
			return err
		}

		if err := e.runEvent(EventLoopEnd, i); err != nil {
			_ = e.runEvent(EventFailure, i)
			return err
		}
	}

	if err := e.runEvent(EventRunEnd, 0); err != nil {
		_ = e.runEvent(EventFailure, 0)
		return err
	}

	return nil
}

// runEvent executes all nodes for the given event. If any node has Parallel=true,
// nodes run concurrently respecting After dependencies; otherwise sequential.
func (e *Executor) runEvent(event Event, iteration int) error {
	var nodes []Node
	hasParallel := false
	for _, n := range e.pipeline.Nodes {
		if n.Event != event {
			continue
		}
		nodes = append(nodes, n)
		if n.Parallel {
			hasParallel = true
		}
	}

	if !hasParallel {
		ctx := e.forceCtx()
		prev := NodeResult{Success: true}
		for _, n := range nodes {
			wctx := WhenContext{
				Iteration:    iteration,
				PrevSuccess:  prev.Success,
				PrevExitCode: prev.ExitCode,
			}
			result, err := e.runNodeWithPolicy(ctx, n, wctx)
			if err != nil {
				return err
			}
			if !result.Skipped {
				prev = result
			}
		}
		return nil
	}

	return e.runEventConcurrent(nodes, iteration)
}

// runEventConcurrent executes nodes concurrently, respecting After dependencies
// and the MaxParallelSteps limit. A stop-policy failure cancels all siblings.
func (e *Executor) runEventConcurrent(nodes []Node, iteration int) error {
	ctx, cancel := context.WithCancel(e.forceCtx())
	defer cancel()

	// Forward shutdown signal into the per-event context.
	if e.cfg.Shutdown != nil {
		go func() {
			select {
			case <-e.cfg.Shutdown:
				cancel()
			case <-ctx.Done():
			}
		}()
	}

	// Index nodes by name for dependency lookups.
	nameToIdx := make(map[string]int, len(nodes))
	for i, n := range nodes {
		nameToIdx[n.Name] = i
	}

	type nodeState struct {
		started bool
		done    bool
	}
	states := make([]nodeState, len(nodes))
	nodeResults := make(map[string]NodeResult, len(nodes))

	var mu sync.Mutex
	doneCh := make(chan error, len(nodes))
	launched := 0

	// Semaphore limits concurrency; nil means unlimited.
	var sem chan struct{}
	if e.pipeline.MaxParallelSteps > 0 {
		sem = make(chan struct{}, e.pipeline.MaxParallelSteps)
	}

	var firstStopErr error

	// prevResultFor returns the NodeResult of the first After dependency that has
	// completed, or a default success result if none is available.
	prevResultFor := func(n Node) NodeResult {
		for _, dep := range n.After {
			if r, ok := nodeResults[dep]; ok && !r.Skipped {
				return r
			}
		}
		return NodeResult{Success: true}
	}

	// schedule starts all nodes whose After dependencies are satisfied.
	schedule := func() {
		if ctx.Err() != nil {
			return
		}
		for i, n := range nodes {
			mu.Lock()
			if states[i].started {
				mu.Unlock()
				continue
			}
			allDepsOk := true
			for _, dep := range n.After {
				idx, ok := nameToIdx[dep]
				if !ok {
					continue // dep not in this event; treat as satisfied
				}
				if !states[idx].done {
					allDepsOk = false
					break
				}
			}
			if !allDepsOk {
				mu.Unlock()
				continue
			}
			states[i].started = true
			launched++
			wctx := WhenContext{
				Iteration:    iteration,
				PrevSuccess:  prevResultFor(n).Success,
				PrevExitCode: prevResultFor(n).ExitCode,
			}
			mu.Unlock()

			go func(i int, node Node, wctx WhenContext) {
				// Acquire semaphore slot (respects cancellation).
				if sem != nil {
					select {
					case sem <- struct{}{}:
						defer func() { <-sem }()
					case <-ctx.Done():
						mu.Lock()
						states[i].done = true
						mu.Unlock()
						doneCh <- nil
						return
					}
				}

				var (
					result NodeResult
					err    error
				)
				if ctx.Err() == nil {
					result, err = e.runNodeWithPolicy(ctx, node, wctx)
				}

				mu.Lock()
				states[i].done = true
				nodeResults[node.Name] = result
				if err != nil && firstStopErr == nil {
					firstStopErr = err
					cancel()
				}
				mu.Unlock()

				doneCh <- err
			}(i, n, wctx)
		}
	}

	schedule()

	received := 0
	for received < launched {
		err := <-doneCh
		received++
		if err != nil {
			mu.Lock()
			if firstStopErr == nil {
				firstStopErr = err
			}
			mu.Unlock()
		}
		schedule()
	}

	mu.Lock()
	err := firstStopErr
	mu.Unlock()
	return err
}

// runNodeWithPolicy evaluates the when condition and applies the node's failure policy.
func (e *Executor) runNodeWithPolicy(ctx context.Context, n Node, wctx WhenContext) (NodeResult, error) {
	if n.When != "" {
		run, err := e.evalWhen(ctx, n.When, wctx)
		if err != nil {
			return NodeResult{}, fmt.Errorf("node %q when condition: %w", n.Name, err)
		}
		if !run {
			return NodeResult{Skipped: true}, nil
		}
	}

	policy := n.EffectiveFailurePolicy()
	maxAttempts := 1
	if policy == FailurePolicyRetry && n.Retries > 0 {
		maxAttempts = 1 + n.Retries
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(e.retryBackoff(attempt))
		}
		lastErr = e.runNode(ctx, n, wctx.Iteration)
		if lastErr == nil {
			return NodeResult{Success: true, ExitCode: 0}, nil
		}
	}

	result := nodeResultFromErr(lastErr)
	if policy == FailurePolicyContinue {
		fmt.Fprintf(e.cfg.Stderr, "node %q failed (continuing): %v\n", n.Name, lastErr)
		return result, nil
	}
	return result, fmt.Errorf("node %q: %w", n.Name, lastErr)
}

func (e *Executor) retryBackoff(attempt int) time.Duration {
	if len(e.cfg.RetryBackoffs) > 0 {
		idx := attempt - 1
		if idx < len(e.cfg.RetryBackoffs) {
			return e.cfg.RetryBackoffs[idx]
		}
		return e.cfg.RetryBackoffs[len(e.cfg.RetryBackoffs)-1]
	}
	defaults := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	idx := attempt - 1
	if idx < len(defaults) {
		return defaults[idx]
	}
	return 30 * time.Second
}

// runNode dispatches to the correct executor for the node kind.
func (e *Executor) runNode(ctx context.Context, n Node, iteration int) error {
	switch n.Kind {
	case NodeKindAgent:
		return e.runAgentNode(ctx, n, iteration)
	case NodeKindCmd:
		return e.runCmdNode(ctx, n, iteration)
	default:
		return fmt.Errorf("unknown node kind %q", n.Kind)
	}
}

// resolveRunner returns the runner for an agent node. If the spec has a
// non-empty Provider and RunnerFactory is configured, the factory is called.
// Otherwise it falls back to the default Runner.
func (e *Executor) resolveRunner(spec *AgentSpec) (agent.Runner, error) {
	if spec.Provider != "" && e.cfg.RunnerFactory != nil {
		return e.cfg.RunnerFactory(spec.Provider)
	}
	if e.cfg.Runner == nil {
		return nil, fmt.Errorf("no runner configured and node has no provider")
	}
	return e.cfg.Runner, nil
}

// runAgentNode executes an agent-kind node using the configured Runner.
func (e *Executor) runAgentNode(ctx context.Context, n Node, iteration int) error {
	spec := n.Agent

	perm := provider.PermissionAcceptEdits
	if spec.Plan {
		perm = provider.PermissionPlan
	} else if spec.Trust {
		perm = provider.PermissionBypass
	}

	workDir := n.WorkDir
	if workDir == "" {
		workDir = e.cfg.WorkDir
	}

	prompt := spec.Prompt
	if e.cfg.SessionID != "" && e.cfg.ReadTrigger != nil {
		if msg, err := e.cfg.ReadTrigger(e.cfg.SessionID); err == nil && msg != "" {
			if e.cfg.FormatTrigger != nil {
				prompt = prompt + "\n\n" + e.cfg.FormatTrigger(msg)
			} else {
				prompt = prompt + "\n\n" + msg
			}
		}
	}

	opts := agent.RunOptions{
		Prompt:          prompt,
		Mode:            provider.ModeHeadless,
		Permission:      perm,
		Timeout:         n.Timeout,
		SystemPrompt:        spec.SystemPrompt,
		Model:           spec.Model,
		WorkingDir:      workDir,
		AllowedTools:    spec.AllowedTools,
		DisallowedTools: spec.DisallowedTools,
		MaxTurns:        spec.MaxTurns,
		MCPConfig:       spec.MCPConfig,
		PassthroughArgs: spec.Passthrough,
		Env:             e.juggleEnv(iteration),
		Context:         ctx,
	}

	runner, err := e.resolveRunner(spec)
	if err != nil {
		return fmt.Errorf("resolve runner for node %q: %w", n.Name, err)
	}

	result, err := runner.Run(opts)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("exit code %d", result.ExitCode)
	}
	return nil
}

// runCmdNode executes a cmd-kind node as a shell command.
func (e *Executor) runCmdNode(ctx context.Context, n Node, iteration int) error {
	spec := n.Cmd

	shell := spec.Shell
	if shell == "" {
		shell = "sh"
	}

	workDir := n.WorkDir
	if workDir == "" {
		workDir = e.cfg.WorkDir
	}

	if n.Timeout > 0 {
		var cancelFn context.CancelFunc
		ctx, cancelFn = context.WithTimeout(ctx, n.Timeout)
		defer cancelFn()
	}

	cmd := exec.CommandContext(ctx, shell, "-c", spec.Command)
	cmd.Dir = workDir
	cmd.Stdout = e.cfg.Stdout
	cmd.Stderr = e.cfg.Stderr
	cmd.Env = append(os.Environ(), e.juggleEnv(iteration)...)
	cmd.Env = append(cmd.Env, spec.Env...)

	return cmd.Run()
}

// evalWhen evaluates a when condition. Structured expressions matching the
// grammar (iteration, success, exit_code) are evaluated in-process; all others
// fall back to shell execution.
func (e *Executor) evalWhen(ctx context.Context, when string, wctx WhenContext) (bool, error) {
	result, matched, err := EvalStructured(when, wctx)
	if err != nil {
		return false, err
	}
	if matched {
		return result, nil
	}

	// Shell fallback.
	cmd := exec.CommandContext(ctx, "sh", "-c", when)
	cmd.Dir = e.cfg.WorkDir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Env = append(os.Environ(), e.juggleEnv(wctx.Iteration)...)

	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// checkShutdown returns an error if the shutdown signal has been received.
func (e *Executor) checkShutdown() error {
	if e.cfg.Shutdown == nil {
		return nil
	}
	select {
	case <-e.cfg.Shutdown:
		return fmt.Errorf("interrupted")
	default:
		return nil
	}
}

// juggleEnv returns JUGGLE_* environment variables for the given iteration.
func (e *Executor) juggleEnv(iteration int) []string {
	env := []string{
		fmt.Sprintf("JUGGLE_ITERATION=%d", iteration),
		fmt.Sprintf("JUGGLE_MAX_ITERATIONS=%d", e.pipeline.Iterations),
		fmt.Sprintf("JUGGLE_RUN_ID=%s", e.cfg.RunID),
	}
	if e.cfg.Label != "" {
		env = append(env, fmt.Sprintf("JUGGLE_LABEL=%s", e.cfg.Label))
	}
	return env
}
