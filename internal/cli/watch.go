package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ohare93/juggle/internal/agent"
)

// ScanWatchDirAll returns paths to all non-hidden regular files in dir.
// Files are in alphabetical order (os.ReadDir sorts by name).
func ScanWatchDirAll(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading watch directory: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	return files, nil
}

// ScanWatchDir returns the path to the first non-hidden regular file.
// Returns empty string if no eligible files found.
// Files are in alphabetical order (os.ReadDir sorts by name).
func ScanWatchDir(dir string) (string, error) {
	files, err := ScanWatchDirAll(dir)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	return files[0], nil
}

// workerCoordinator tracks which files are currently claimed by workers,
// ensuring no two workers pick the same task file simultaneously.
type workerCoordinator struct {
	mu      sync.Mutex
	claimed map[string]bool
}

func newWorkerCoordinator() *workerCoordinator {
	return &workerCoordinator{claimed: make(map[string]bool)}
}

// claim scans dir and atomically claims the first unclaimed file.
// Returns empty string if no unclaimed files are available.
func (c *workerCoordinator) claim(dir string) (string, error) {
	files, err := ScanWatchDirAll(dir)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range files {
		if !c.claimed[f] {
			c.claimed[f] = true
			return f, nil
		}
	}
	return "", nil
}

// release removes a file from the claimed set, making it available again.
func (c *workerCoordinator) release(path string) {
	c.mu.Lock()
	delete(c.claimed, path)
	c.mu.Unlock()
}

// prefixWriter wraps an io.Writer and prepends prefix to each complete line.
// Partial lines (no trailing newline) are buffered until the newline arrives.
type prefixWriter struct {
	mu     sync.Mutex
	w      io.Writer
	prefix string
	buf    []byte
}

func newPrefixWriter(w io.Writer, prefix string) *prefixWriter {
	return &prefixWriter{w: w, prefix: prefix}
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	pw.buf = append(pw.buf, p...)
	for {
		idx := bytes.IndexByte(pw.buf, '\n')
		if idx < 0 {
			break
		}
		line := pw.buf[:idx+1]
		fmt.Fprintf(pw.w, "%s%s", pw.prefix, line)
		pw.buf = pw.buf[idx+1:]
	}
	return len(p), nil
}

// workerIDRunner wraps a Runner and injects JUGGLE_WORKER_ID into every Run call.
type workerIDRunner struct {
	inner    agent.Runner
	workerID int
}

func (r *workerIDRunner) Run(opts agent.RunOptions) (*agent.RunResult, error) {
	opts.Env = append(opts.Env, fmt.Sprintf("JUGGLE_WORKER_ID=%d", r.workerID))
	return r.inner.Run(opts)
}

// RunWatch processes task files from a watched directory.
// For each file, reads contents, runs iterations, then picks next.
// Idles when empty, polling at delay interval (minimum 30 seconds).
// When cfg.Workers > 1, spawns that many concurrent worker goroutines.
func RunWatch(cfg Config) error {
	info, err := os.Stat(cfg.Watch)
	if err != nil {
		return fmt.Errorf("watch directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("watch path is not a directory: %s", cfg.Watch)
	}

	if cfg.Workers > 1 {
		return runWatchWorkers(cfg)
	}

	// Calculate poll delay: max(delay_minutes, 30s)
	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	stats := runStats{start: time.Now(), model: cfg.Model}

	for {
		// Check shutdown before starting each scan/task cycle
		select {
		case <-cfg.Shutdown:
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		taskPath, err := ScanWatchDir(cfg.Watch)
		if err != nil {
			return err
		}

		if taskPath == "" {
			fmt.Fprintf(cfg.Stderr, "Watch directory empty, polling in %v...\n", pollDelay)
			// Interruptible poll sleep
			select {
			case <-time.After(pollDelay):
			case <-cfg.Shutdown:
				// Handled at top of next loop iteration
			}
			continue
		}

		filename := filepath.Base(taskPath)
		fmt.Fprintf(cfg.Stderr, "Processing task: %s\n", filename)

		if err := runWatchTask(cfg, taskPath, filename, &stats); err != nil {
			if errors.Is(err, ErrInterrupted) {
				writeSummary(cfg, stats)
				return ErrInterrupted
			}
			if errors.Is(err, errCostGuard) {
				writeSummary(cfg, stats)
				return nil
			}
			fmt.Fprintf(cfg.Stderr, "Error processing %s: %v\n", filename, err)
			continue
		}
	}
}

// runWatchTask runs the iteration loop for a single watch task file.
// Re-reads the task file each iteration to pick up agent-appended progress.
// stats is updated with completed iteration metrics (may be nil).
func runWatchTask(cfg Config, taskFile, filename string, stats *runStats) error {
	if cfg.RunID == "" {
		cfg.RunID = generateRunID()
	}
	max := cfg.Iterations
	formatter := NewLoopFormatter(cfg.Stderr)
	consecutiveFailures := 0
	retryCount := 0

	onFailure := cfg.OnFailure
	if onFailure == "" {
		onFailure = OnFailureStop
	}

	// Rate limit backoff state
	const initialBackoff = 30 * time.Second
	const maxBackoff = 10 * time.Minute
	backoff := initialBackoff

	// Run agent-pre once before the task loop (failure stops the task)
	if cfg.AgentPre != "" {
		env := phaseEnv{phase: "pre", iteration: 0, maxIter: max}
		if err := runPhaseAgent(cfg, cfg.AgentPre, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-pre failed: %w", err)
		}
	}

	for i := 1; max == 0 || i <= max; i++ {
		// Check shutdown flag before starting each new iteration
		select {
		case <-cfg.Shutdown:
			return ErrInterrupted
		default:
		}

		formatter.IterationHeader(i, max, filename)

		// Run agent-before; skip iteration on failure
		if cfg.AgentBefore != "" {
			env := phaseEnv{phase: "before", iteration: i, maxIter: max}
			if err := runPhaseAgent(cfg, cfg.AgentBefore, env, cfg.Stderr); err != nil {
				fmt.Fprintf(cfg.Stderr, "agent-before failed (skipping iteration %d): %v\n", i, err)
				continue
			}
		}

		// Run cmd-before; skip iteration on failure
		if cfg.CmdBefore != "" {
			if err := runHook(cfg.CmdBefore, hookEnv{iteration: i, maxIterations: max}, cfg.Stderr); err != nil {
				fmt.Fprintf(cfg.Stderr, "cmd-before failed (skipping iteration %d): %v\n", i, err)
				continue
			}
		}

		start := time.Now()

		// Re-read task file each iteration
		contents, err := os.ReadFile(taskFile)
		if err != nil {
			// File gone (agent deleted it) means task complete
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("reading task file %s: %w", filename, err)
		}

		prompt := BuildWatchPrompt(string(contents), cfg.Content, filename, i, max)
		opts := buildRunOptions(cfg, prompt)
		opts.Env = append(opts.Env, buildJuggleEnv(cfg.RunID, i, max, cfg.Label, cfg.Model, cfg.Provider, taskFile, -1)...)

		result, err := cfg.Runner.Run(opts)
		if err != nil {
			return fmt.Errorf("runner error on iteration %d of %s: %w", i, filename, err)
		}

		// Handle overload exhausted
		if result.OverloadExhausted {
			return fmt.Errorf("agent exhausted overload retries on iteration %d of %s", i, filename)
		}

		// Handle quota/usage exhaustion — sleep until window resets
		if result.QuotaExhausted {
			var wait time.Duration
			var waitMsg string
			if !result.QuotaResetsAt.IsZero() {
				wait = time.Until(result.QuotaResetsAt)
				if wait < 0 {
					wait = 0
				}
				resetStr := result.QuotaResetsAt.Format("15:04:05")
				waitMsg = fmt.Sprintf("usage quota hit, waiting until %s (%s) for window reset", resetStr, formatWaitDuration(wait))
			} else if result.RetryAfter > 0 {
				wait = result.RetryAfter
				waitMsg = fmt.Sprintf("usage quota hit (reset time unknown), waiting %s", formatWaitDuration(wait))
			} else {
				wait = backoff
				waitMsg = fmt.Sprintf("usage quota hit (reset time unknown), waiting %s", formatWaitDuration(wait))
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}

			if cfg.MaxWait > 0 && wait > cfg.MaxWait {
				return fmt.Errorf("usage quota hit: wait %v exceeds max-wait %v", wait, cfg.MaxWait)
			}

			fmt.Fprintln(cfg.Stderr, waitMsg)
			select {
			case <-time.After(wait):
			case <-cfg.Shutdown:
				return ErrInterrupted
			}

			// Retry same iteration
			i--
			continue
		}

		// Handle rate limiting with exponential backoff
		if result.RateLimited {
			wait := backoff
			if result.RetryAfter > 0 {
				wait = result.RetryAfter
			}

			// Check max-wait
			if cfg.MaxWait > 0 && wait > cfg.MaxWait {
				return fmt.Errorf("rate limited: wait %v exceeds max-wait %v", wait, cfg.MaxWait)
			}

			fmt.Fprintf(cfg.Stderr, "rate limited, waiting %v before retry\n", wait)
			select {
			case <-time.After(wait):
			case <-cfg.Shutdown:
				return ErrInterrupted
			}

			// Double backoff for next time, cap at maxBackoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			// Retry same iteration
			i--
			continue
		}

		// Handle non-zero exit code according to --on-failure mode
		if result.ExitCode != 0 {
			switch onFailure {
			case OnFailureStop:
				return fmt.Errorf("iteration %d failed (exit code %d)", i, result.ExitCode)

			case OnFailureRetry:
				maxRetries := cfg.Retries
				if maxRetries == 0 {
					maxRetries = 2
				}
				if retryCount < maxRetries {
					bd := retryBackoffFor(retryCount, cfg.RetryBackoffs)
					fmt.Fprintf(cfg.Stderr, "iteration %d failed (exit code %d), retrying in %v (attempt %d/%d)\n",
						i, result.ExitCode, bd, retryCount+1, maxRetries)
					select {
					case <-time.After(bd):
					case <-cfg.Shutdown:
						return ErrInterrupted
					}
					retryCount++
					i-- // retry same iteration
					continue
				}
				// Retries exhausted — treat as a continued failure
				retryCount = 0
				fmt.Fprintf(cfg.Stderr, "iteration %d failed after %d retries, continuing\n", i, maxRetries)
				// fall through to consecutive failure tracking

			case OnFailureContinue:
				fmt.Fprintf(cfg.Stderr, "iteration %d failed (exit code %d), continuing\n", i, result.ExitCode)
			}

			// Consecutive failure tracking (continue/retry-exhausted)
			consecutiveFailures++
			if cfg.MaxFailures > 0 && consecutiveFailures >= cfg.MaxFailures {
				return fmt.Errorf("stopping: %d consecutive failures", consecutiveFailures)
			}
		} else {
			consecutiveFailures = 0
			retryCount = 0
		}

		// Reset backoff, accumulate stats, and print status
		backoff = initialBackoff
		if stats != nil {
			stats.iterations++
			stats.inputTokens += result.InputTokens
			stats.outputTokens += result.OutputTokens
			stats.cacheTokens += result.CacheTokens
		}
		formatter.IterationStatus(time.Since(start), result.InputTokens, result.OutputTokens, result.CacheTokens)

		// Run agent-after; log warning on failure but continue
		if cfg.AgentAfter != "" {
			env := phaseEnv{phase: "after", iteration: i, maxIter: max}
			if err := runPhaseAgent(cfg, cfg.AgentAfter, env, cfg.Stderr); err != nil {
				fmt.Fprintf(cfg.Stderr, "agent-after failed (iteration %d): %v\n", i, err)
			}
		}

		// Run cmd-after; log warning on failure but continue
		if cfg.CmdAfter != "" {
			afterEnv := hookEnv{
				iteration:     i,
				maxIterations: max,
				exitCode:      result.ExitCode,
				inputTokens:   result.InputTokens,
				outputTokens:  result.OutputTokens,
			}
			if err := runHook(cfg.CmdAfter, afterEnv, cfg.Stderr); err != nil {
				fmt.Fprintf(cfg.Stderr, "cmd-after failed (iteration %d): %v\n", i, err)
			}
		}

		// Run stop-when; exit 0 means stop gracefully
		if cfg.StopWhen != "" {
			stopEnv := hookEnv{
				iteration:     i,
				maxIterations: max,
				exitCode:      result.ExitCode,
				inputTokens:   result.InputTokens,
				outputTokens:  result.OutputTokens,
			}
			if err := runHook(cfg.StopWhen, stopEnv, cfg.Stderr); err == nil {
				fmt.Fprintf(cfg.Stderr, "stop-when condition met after iteration %d, stopping\n", i)
				return nil
			}
		}

		// Check cost guard after each iteration, before delay sleep
		if cfg.MaxCost > 0 && stats != nil {
			cost := estimateCost(stats.inputTokens, stats.outputTokens, stats.model)
			if cost > cfg.MaxCost {
				fmt.Fprintf(cfg.Stderr, "cost guard triggered: estimated $%.4f exceeds --max-cost $%.4f\n", cost, cfg.MaxCost)
				return errCostGuard
			}
		}

		// Wait between iterations (skip after last), interruptible by shutdown
		if (max == 0 || i < max) && (cfg.Delay > 0 || cfg.Fuzz > 0) {
			d := computeDelay(cfg.Delay, cfg.Fuzz)
			if d > 0 {
				fmt.Fprintf(cfg.Stderr, "waiting %v before next iteration\n", d)
				select {
				case <-time.After(d):
				case <-cfg.Shutdown:
					return ErrInterrupted
				}
			}
		}
	}

	// Run agent-post once after the task loop (failure stops the task)
	if cfg.AgentPost != "" {
		env := phaseEnv{phase: "post", iteration: 0, maxIter: max}
		if err := runPhaseAgent(cfg, cfg.AgentPost, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-post failed: %w", err)
		}
	}

	return nil
}

// runWatchWorkers spawns cfg.Workers goroutines, each picking task files from
// the watch directory via a shared coordinator to prevent duplicate selection.
// Each worker prefixes its stderr output with [worker-N].
func runWatchWorkers(cfg Config) error {
	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	coord := newWorkerCoordinator()
	errs := make(chan error, cfg.Workers)
	var wg sync.WaitGroup

	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerCfg := cfg
			workerCfg.Stderr = newPrefixWriter(cfg.Stderr, fmt.Sprintf("[worker-%d] ", workerID))
			workerCfg.Runner = &workerIDRunner{inner: cfg.Runner, workerID: workerID}
			if err := runWorkerLoop(workerCfg, coord, pollDelay); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}
	return nil
}

// runWorkerLoop is the per-worker pick→process→repeat loop.
func runWorkerLoop(cfg Config, coord *workerCoordinator, pollDelay time.Duration) error {
	stats := runStats{start: time.Now(), model: cfg.Model}

	for {
		select {
		case <-cfg.Shutdown:
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		taskPath, err := coord.claim(cfg.Watch)
		if err != nil {
			return err
		}

		if taskPath == "" {
			fmt.Fprintf(cfg.Stderr, "No tasks available, polling in %v...\n", pollDelay)
			select {
			case <-time.After(pollDelay):
			case <-cfg.Shutdown:
			}
			continue
		}

		filename := filepath.Base(taskPath)
		fmt.Fprintf(cfg.Stderr, "Processing task: %s\n", filename)

		if err := runWatchTask(cfg, taskPath, filename, &stats); err != nil {
			coord.release(taskPath)
			if errors.Is(err, ErrInterrupted) {
				writeSummary(cfg, stats)
				return ErrInterrupted
			}
			if errors.Is(err, errCostGuard) {
				writeSummary(cfg, stats)
				return nil
			}
			fmt.Fprintf(cfg.Stderr, "Error processing %s: %v\n", filename, err)
			continue
		}
		coord.release(taskPath)
	}
}

// buildRunOptions creates RunOptions from Config and a prompt.
func buildRunOptions(cfg Config, prompt string) agent.RunOptions {
	mode := agent.ModeHeadless
	if cfg.Interactive {
		mode = agent.ModeInteractive
	}

	perm := agent.PermissionAcceptEdits
	if cfg.Plan {
		perm = agent.PermissionPlan
	} else if cfg.Trust {
		perm = agent.PermissionBypass
	}

	return agent.RunOptions{
		Prompt:            prompt,
		Mode:              mode,
		Permission:        perm,
		Model:             cfg.Model,
		Timeout:           cfg.Timeout,
		ShowThinking:      cfg.ShowThinking,
		Verbose:           cfg.Verbose,
		Context:           cfg.ForceCtx,
		HooksSettingsFile: cfg.HooksSettingsFile,
		AllowedTools:      cfg.AllowedTools,
		DisallowedTools:   cfg.DisallowedTools,
		MaxTurns:          cfg.MaxTurns,
		MCPConfig:         cfg.MCPConfig,
		PassthroughArgs:   cfg.PassthroughArgs,
		SystemPrompt:      cfg.SystemPrompt,
		WorkingDir:        cfg.WorkDir,
	}
}
