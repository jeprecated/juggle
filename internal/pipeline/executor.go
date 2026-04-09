package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/agent/provider"
)

// ExecutorConfig holds the dependencies and runtime settings for a pipeline execution.
type ExecutorConfig struct {
	Runner        agent.Runner
	Stdout        io.Writer
	Stderr        io.Writer
	ForceCtx      context.Context
	Shutdown      <-chan struct{}
	WorkDir       string
	RunID         string
	Label         string
	RetryBackoffs []time.Duration // override retry backoffs (default: 5s, 15s, 30s)
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

// runEvent executes all nodes for the given event in pipeline order.
func (e *Executor) runEvent(event Event, iteration int) error {
	for _, n := range e.pipeline.Nodes {
		if n.Event != event {
			continue
		}
		if err := e.runNodeWithPolicy(n, iteration); err != nil {
			return err
		}
	}
	return nil
}

// runNodeWithPolicy evaluates the when condition and applies the node's failure policy.
func (e *Executor) runNodeWithPolicy(n Node, iteration int) error {
	if n.When != "" {
		run, err := e.evalWhen(n.When, iteration)
		if err != nil {
			return fmt.Errorf("node %q when condition: %w", n.Name, err)
		}
		if !run {
			return nil
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
		lastErr = e.runNode(n, iteration)
		if lastErr == nil {
			return nil
		}
	}

	if policy == FailurePolicyContinue {
		fmt.Fprintf(e.cfg.Stderr, "node %q failed (continuing): %v\n", n.Name, lastErr)
		return nil
	}
	return fmt.Errorf("node %q: %w", n.Name, lastErr)
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
func (e *Executor) runNode(n Node, iteration int) error {
	switch n.Kind {
	case NodeKindAgent:
		return e.runAgentNode(n, iteration)
	case NodeKindCmd:
		return e.runCmdNode(n, iteration)
	default:
		return fmt.Errorf("unknown node kind %q", n.Kind)
	}
}

// runAgentNode executes an agent-kind node using the configured Runner.
func (e *Executor) runAgentNode(n Node, iteration int) error {
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

	ctx := e.cfg.ForceCtx
	if ctx == nil {
		ctx = context.Background()
	}

	opts := agent.RunOptions{
		Prompt:          spec.Prompt,
		Mode:            provider.ModeHeadless,
		Permission:      perm,
		Timeout:         n.Timeout,
		SystemPrompt:    spec.SystemPrompt,
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

	result, err := e.cfg.Runner.Run(opts)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("exit code %d", result.ExitCode)
	}
	return nil
}

// runCmdNode executes a cmd-kind node as a shell command.
func (e *Executor) runCmdNode(n Node, iteration int) error {
	spec := n.Cmd

	shell := spec.Shell
	if shell == "" {
		shell = "sh"
	}

	workDir := n.WorkDir
	if workDir == "" {
		workDir = e.cfg.WorkDir
	}

	ctx := e.cfg.ForceCtx
	if ctx == nil {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, shell, "-c", spec.Command)
	cmd.Dir = workDir
	cmd.Stdout = e.cfg.Stdout
	cmd.Stderr = e.cfg.Stderr
	cmd.Env = append(os.Environ(), e.juggleEnv(iteration)...)
	cmd.Env = append(cmd.Env, spec.Env...)

	return cmd.Run()
}

// evalWhen evaluates a when condition (shell command).
// Returns true if the command exits 0 (node should run), false otherwise.
func (e *Executor) evalWhen(when string, iteration int) (bool, error) {
	ctx := e.cfg.ForceCtx
	if ctx == nil {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", when)
	cmd.Env = append(os.Environ(), e.juggleEnv(iteration)...)

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
