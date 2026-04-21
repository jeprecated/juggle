package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// BuildPhaseContent resolves @file references in args and joins results with \n\n.
// Returns an empty string when args is empty or nil.
func BuildPhaseContent(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	resolved, err := ResolveArgs(args)
	if err != nil {
		return "", err
	}
	return strings.Join(resolved, "\n\n"), nil
}

// phaseEnv holds metadata for a phase agent invocation.
type phaseEnv struct {
	phase     string // pre, before, after, post
	iteration int    // current iteration number (0 for pre/post)
	maxIter   int    // max iterations
	runID     string // stable UUID for the entire run
	model     string // model name
	provider  string // provider name
	label     string // optional run label
}

// envSlice returns the phase environment as KEY=VALUE strings.
func (p phaseEnv) envSlice() []string {
	env := []string{
		"JUGGLE_PHASE=" + p.phase,
		"JUGGLE_ITERATION=" + strconv.Itoa(p.iteration),
		"JUGGLE_MAX_ITERATIONS=" + strconv.Itoa(p.maxIter),
		"JUGGLE_RUN_ID=" + p.runID,
		"JUGGLE_MODEL=" + p.model,
		"JUGGLE_PROVIDER=" + p.provider,
	}
	if p.label != "" {
		env = append(env, "JUGGLE_LABEL="+p.label)
	}
	return env
}

// runPhaseAgent executes a phase agent session with the given prompt.
// It reuses the main Config's Runner with a fresh prompt and phase env vars.
// Phase agents are interruptible by the first shutdown signal (Ctrl+C),
// unlike main iterations which finish their current run before stopping.
// Returns an error if the agent exits non-zero or encounters a runner error.
func runPhaseAgent(cfg Config, prompt string, env phaseEnv, w io.Writer) error {
	opts := buildRunOptions(cfg, prompt)

	// Phase agents are auxiliary — interrupt on first shutdown signal,
	// not just the force-kill context.
	if cfg.Shutdown != nil {
		ctx := opts.Context
		if ctx == nil {
			ctx = context.Background()
		}
		cancelCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			select {
			case <-cfg.Shutdown:
				cancel()
			case <-cancelCtx.Done():
			}
		}()
		opts.Context = cancelCtx
	}

	opts.Env = env.envSlice()

	result, err := cfg.Runner.Run(opts)

	// If shutdown triggered the kill, return ErrInterrupted
	if err != nil && cfg.Shutdown != nil {
		select {
		case <-cfg.Shutdown:
			return ErrInterrupted
		default:
		}
	}

	if err != nil {
		return fmt.Errorf("phase agent (%s) runner error: %w", env.phase, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("phase agent (%s) exited with code %d", env.phase, result.ExitCode)
	}
	return nil
}

// runQueueAgentPost fires the agent-post phase once when the queue drains
// (had tasks, now idle). Resets ranTask to false.
func runQueueAgentPost(cfg Config, ranTask *bool, iteration, maxIter int) error {
	if !*ranTask || cfg.AgentPost == "" {
		return nil
	}
	*ranTask = false
	formatter := NewLoopFormatter(cfg.Stderr)
	formatter.PhaseAgentHeader("post")
	if cfg.Verbose {
		fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentPost)
	}
	env := phaseEnv{phase: "post", iteration: iteration, maxIter: maxIter, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
	if err := runPhaseAgent(cfg, cfg.AgentPost, env, cfg.Stderr); err != nil {
		return fmt.Errorf("agent-post failed: %w", err)
	}
	return nil
}
