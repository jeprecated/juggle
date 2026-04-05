package cli

import (
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
// Returns an error if the agent exits non-zero or encounters a runner error.
func runPhaseAgent(cfg Config, prompt string, env phaseEnv, w io.Writer) error {
	opts := buildRunOptions(cfg, prompt)
	opts.Env = env.envSlice()

	result, err := cfg.Runner.Run(opts)
	if err != nil {
		return fmt.Errorf("phase agent (%s) runner error: %w", env.phase, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("phase agent (%s) exited with code %d", env.phase, result.ExitCode)
	}
	return nil
}
