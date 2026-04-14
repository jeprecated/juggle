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

// runTaskState tracks state for running watch tasks across iterations.
type runTaskState struct {
	consecutiveFailures int
	retryCount          int
	backoff             time.Duration
}

// newRunTaskState creates an initialized runTaskState.
func newRunTaskState() *runTaskState {
	return &runTaskState{
		backoff: 30 * time.Second,
	}
}

// resetBackoff resets the backoff to the initial value.
func (s *runTaskState) resetBackoff() {
	s.backoff = 30 * time.Second
}

// incrementBackoff doubles the backoff, capped at 10 minutes.
func (s *runTaskState) incrementBackoff() {
	s.backoff *= 2
	const maxBackoff = 10 * time.Minute
	if s.backoff > maxBackoff {
		s.backoff = maxBackoff
	}
}

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

// touchTracker tracks file modification times for --on-touch mode.
// It detects both new files and files whose mtime has changed.
type touchTracker struct {
	mu     sync.Mutex
	mtimes map[string]time.Time
}

func newTouchTracker() *touchTracker {
	return &touchTracker{mtimes: make(map[string]time.Time)}
}

// scanTouchDir returns the first new or touched file in dir.
// A file is considered "touched" if its mtime differs from the last recorded value.
func (t *touchTracker) scanTouchDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading watch directory: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		prev, seen := t.mtimes[path]
		if !seen || !info.ModTime().Equal(prev) {
			t.mtimes[path] = info.ModTime()
			return path, nil
		}
	}
	return "", nil
}

// scanTouchDirAll returns all new or touched files in dir.
func (t *touchTracker) scanTouchDirAll(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading watch directory: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		prev, seen := t.mtimes[path]
		if !seen || !info.ModTime().Equal(prev) {
			t.mtimes[path] = info.ModTime()
			files = append(files, path)
		}
	}
	return files, nil
}

// claimTouchDir atomically claims the first new or touched file not already claimed.
func (t *touchTracker) claimTouchDir(dir string, coord *workerCoordinator) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading watch directory: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	coord.mu.Lock()
	defer coord.mu.Unlock()
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if coord.claimed[path] {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		prev, seen := t.mtimes[path]
		if !seen || !info.ModTime().Equal(prev) {
			t.mtimes[path] = info.ModTime()
			coord.claimed[path] = true
			return path, nil
		}
	}
	return "", nil
}

// update records the current mtime for a file path.
func (t *touchTracker) update(path string, mod time.Time) {
	t.mu.Lock()
	t.mtimes[path] = mod
	t.mu.Unlock()
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

// RunWatch processes task files from watched directories or glob patterns.
// For each file, reads contents, runs iterations, then picks next.
// Idles when empty, polling at delay interval (minimum 30 seconds).
// Routing:
//   - len(Watch) == 1, glob pattern → runGlobWatch (expands glob each cycle)
//   - len(Watch) == 1, plain dir   → single-dir watch loop
//   - len(Watch) > 1               → runMultiWatch (merges all dirs via claimFromDirs)
func RunWatch(cfg Config) error {
	if len(cfg.Watch) > 1 {
		return runMultiWatch(cfg)
	}

	watchDir := cfg.Watch[0]

	if isGlobPattern(watchDir) {
		return runGlobWatch(cfg)
	}

	info, err := os.Stat(watchDir)
	if err != nil {
		return fmt.Errorf("watch directory: %w", err)
	}

	if !info.IsDir() {
		if !cfg.OnTouch {
			return fmt.Errorf("watch path is not a directory: %s (use --on-touch to watch a file for changes)", watchDir)
		}
		return runTouchWatch(cfg)
	}

	if cfg.Workers > 1 {
		return runWatchWorkers(cfg)
	}

	// Calculate poll delay: max(delay_minutes, 30s)
	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	var dash *workerDashboard
	if cfg.Dashboard {
		dash = setupWorkerDashboard(watchDir, 1, cfg.Stderr)
		defer dash.stop()
		if logFile, closeLog := dash.openWorkerLog(0); logFile != nil {
			cfg.Stderr = logFile
			defer closeLog()
		}
	}

	stats := runStats{runID: cfg.RunID, start: time.Now(), model: cfg.Model}

	// Run agent-pre once before the iteration loop
	if cfg.AgentPre != "" {
		formatter := NewLoopFormatter(cfg.Stderr)
		formatter.PhaseAgentHeader("pre")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentPre)
		}
		env := phaseEnv{phase: "pre", iteration: 0, maxIter: cfg.Iterations, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentPre, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-pre failed: %w", err)
		}
	}

	max := cfg.Iterations
	taskState := newRunTaskState()

	var touchTrack *touchTracker
	if cfg.OnTouch {
		touchTrack = newTouchTracker()
	}

	for i := 1; max == 0 || i <= max; i++ {
		select {
		case <-cfg.Shutdown:
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		var triggerMsg string
		if cfg.EffectiveID != "" {
			if msg, err := ReadTrigger(cfg.EffectiveID); err == nil && msg != "" {
				triggerMsg = msg
			}
		}

		var taskPath string
		if triggerMsg == "" {
			if cfg.OnTouch {
				taskPath, err = touchTrack.scanTouchDir(watchDir)
			} else {
				taskPath, err = ScanWatchDir(watchDir)
			}
			if err != nil {
				return err
			}
		}

		if taskPath == "" && triggerMsg == "" {
			if dash != nil {
				dash.dash.Update(0, WorkerState{Status: WorkerIdle, LogFile: dash.logFiles[0]})
			}
			pollWaitWithWake(cfg.Stderr, fmt.Sprintf("Watching %s", watchDir), pollDelay, cfg.Shutdown, wakeCh(&cfg))
			continue
		}

		taskCfg := cfg
		var filename string
		if triggerMsg != "" {
			filename = "trigger"
			taskCfg.Content = taskCfg.Content + "\n\n" + FormatTrigger(triggerMsg)
		} else {
			filename = filepath.Base(taskPath)
		}
		if dash != nil {
			logFile := dash.logFiles[0]
			dash.dash.Update(0, WorkerState{
				Status:    WorkerActive,
				TaskName:  filename,
				Iteration: i,
				MaxIter:   max,
				LogFile:   logFile,
			})
			taskCfg.OnIterDone = func(iter, maxIter int) {
				dash.dash.Update(0, WorkerState{
					Status:    WorkerActive,
					TaskName:  filename,
					Iteration: iter,
					MaxIter:   maxIter,
					LogFile:   logFile,
				})
			}
		}
		fmt.Fprintf(cfg.Stderr, "Processing task: %s\n", filename)

		if triggerMsg != "" {
			err = runTriggerTask(taskCfg, i, max, taskState, &stats)
		} else {
			err = runWatchTask(taskCfg, taskPath, i, max, taskState, &stats)
		}
		if dash != nil {
			dash.dash.Update(0, WorkerState{Status: WorkerIdle, LogFile: dash.logFiles[0]})
		}
		if err != nil {
			if errors.Is(err, ErrInterrupted) {
				writeSummary(cfg, stats)
				return ErrInterrupted
			}
			if errors.Is(err, errCostGuard) {
				writeSummary(cfg, stats)
				return nil
			}
			if errors.Is(err, errFileGone) {
				// File completed by agent, continue to next iteration
				continue
			}
			fmt.Fprintf(cfg.Stderr, "Error processing %s: %v\n", filename, err)
			continue
		}

		// Wait between iterations (skip after last), interruptible by shutdown or wake
		if (max == 0 || i < max) && (cfg.Delay > 0 || cfg.Fuzz > 0) {
			d := computeDelay(cfg.Delay, cfg.Fuzz)
			if d > 0 {
				fmt.Fprintf(cfg.Stderr, "waiting %v before next iteration\n", d)
				select {
				case <-time.After(d):
				case <-cfg.Shutdown:
					writeSummary(cfg, stats)
					return ErrInterrupted
				case <-wakeCh(&cfg):
					fmt.Fprintf(cfg.Stderr, "wake signal received, starting next iteration\n")
				}
			}
		}
	}

	// Run agent-post once after the iteration loop
	if cfg.AgentPost != "" {
		formatter := NewLoopFormatter(cfg.Stderr)
		formatter.PhaseAgentHeader("post")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentPost)
		}
		env := phaseEnv{phase: "post", iteration: 0, maxIter: max, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentPost, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-post failed: %w", err)
		}
	}

	writeSummary(cfg, stats)
	return nil
}

// runTriggerTask runs a single iteration triggered by an external message.
// It mirrors runWatchTask but skips the file-read step since the trigger
// content is already appended to cfg.Content.
func runTriggerTask(cfg Config, iteration, maxIter int, state *runTaskState, stats *runStats) error {
	if cfg.RunID == "" {
		cfg.RunID = generateRunID()
	}

	select {
	case <-cfg.Shutdown:
		return ErrInterrupted
	default:
	}

	formatter := NewLoopFormatter(cfg.Stderr)
	formatter.IterationHeader(iteration, maxIter, "trigger", cfg.Label)

	onFailure := cfg.OnFailure
	if onFailure == "" {
		onFailure = OnFailureStop
	}

	if cfg.AgentBefore != "" {
		formatter.PhaseAgentHeader("before")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentBefore)
		}
		env := phaseEnv{phase: "before", iteration: iteration, maxIter: maxIter, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentBefore, env, cfg.Stderr); err != nil {
			if errors.Is(err, ErrInterrupted) {
				return ErrInterrupted
			}
			return fmt.Errorf("agent-before failed (iteration %d): %w", iteration, err)
		}
	}

	if cfg.CmdBefore != "" {
		formatter.CmdHookMarker("cmd-before", cfg.CmdBefore)
		if err := runHook(cfg.CmdBefore, hookEnv{iteration: iteration, maxIterations: maxIter, runID: cfg.RunID, label: cfg.Label, model: cfg.Model, provider: cfg.Provider}, cfg.Stderr); err != nil {
			fmt.Fprintf(cfg.Stderr, "cmd-before failed (iteration %d): %v\n", iteration, err)
			return nil
		}
	}

	start := time.Now()

	content := cfg.Content
	if state.retryCount > 0 && cfg.RetryPrompt != "" {
		content = cfg.RetryPrompt + "\n\n" + content
	}
	prompt := BuildPrompt(content, iteration, maxIter)
	printVerboseProviderCommand(cfg, prompt)
	opts := buildRunOptions(cfg, prompt)
	opts.Env = append(opts.Env, buildJuggleEnv(cfg.RunID, iteration, maxIter, cfg.Label, cfg.Model, cfg.Provider, "", cfg.WorkerID)...)

	writeIterStartLog(cfg.Log, iterStartLogEntry{
		RunID:     cfg.RunID,
		Iteration: iteration,
		WorkerID:  cfg.WorkerID,
	})

	result, err := cfg.Runner.Run(opts)
	if err != nil {
		return fmt.Errorf("runner error on trigger iteration %d: %w", iteration, err)
	}

	if result.OverloadExhausted {
		return fmt.Errorf("agent exhausted overload retries on trigger iteration %d", iteration)
	}

	if result.ExitCode != 0 {
		switch onFailure {
		case OnFailureRetry:
			maxRetries := cfg.Retries
			if maxRetries == 0 {
				maxRetries = 2
			}
			if state.retryCount < maxRetries {
				state.retryCount++
				return nil
			}
			state.retryCount = 0
		case OnFailureContinue:
		default:
			return fmt.Errorf("trigger iteration %d failed (exit code %d)", iteration, result.ExitCode)
		}
	} else {
		state.retryCount = 0
	}

	stats.iterations++
	stats.inputTokens += result.InputTokens
	stats.outputTokens += result.OutputTokens
	stats.cacheTokens += result.CacheTokens
	formatter.IterationStatus(time.Since(start), result.InputTokens, result.OutputTokens, result.CacheTokens)

	writeIterationLog(cfg.Log, iterationLogEntry{
		RunID:        cfg.RunID,
		Timestamp:    time.Now().UTC(),
		Iteration:    iteration,
		WorkerID:     cfg.WorkerID,
		DurationMs:   time.Since(start).Milliseconds(),
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		CacheTokens:  result.CacheTokens,
		ExitCode:     result.ExitCode,
	})

	if cfg.AgentAfter != "" {
		formatter.PhaseAgentHeader("after")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentAfter)
		}
		env := phaseEnv{phase: "after", iteration: iteration, maxIter: maxIter, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentAfter, env, cfg.Stderr); err != nil {
			if errors.Is(err, ErrInterrupted) {
				return ErrInterrupted
			}
			fmt.Fprintf(cfg.Stderr, "agent-after failed (iteration %d): %v\n", iteration, err)
		}
	}

	if cfg.CmdAfter != "" {
		formatter.CmdHookMarker("cmd-after", cfg.CmdAfter)
		afterEnv := hookEnv{
			iteration:     iteration,
			maxIterations: maxIter,
			exitCode:      result.ExitCode,
			inputTokens:   result.InputTokens,
			outputTokens:  result.OutputTokens,
			runID:         cfg.RunID,
			label:         cfg.Label,
			model:         cfg.Model,
			provider:      cfg.Provider,
		}
		if err := runHook(cfg.CmdAfter, afterEnv, cfg.Stderr); err != nil {
			fmt.Fprintf(cfg.Stderr, "cmd-after failed (iteration %d): %v\n", iteration, err)
		}
	}

	if cfg.MaxCost > 0 && stats != nil {
		cost := estimateCost(stats.inputTokens, stats.outputTokens, stats.model)
		if cost > cfg.MaxCost {
			return errCostGuard
		}
	}

	return nil
}

// runWatchTask runs a single iteration of a watch task file.
// The iteration number is provided by the caller (global iteration counter).
// state tracks consecutive failures, retry count, and backoff across iterations.
// stats is updated with completed iteration metrics (may be nil).
// Returns special errors:
//   - errFileGone: file was deleted by the agent, task is complete
//   - errCostGuard: max-cost threshold exceeded
//   - other errors: task failed, logged by caller
func runWatchTask(cfg Config, taskFile string, iteration, maxIter int, state *runTaskState, stats *runStats) error {
	// Compute relative path for use in prompts; fall back to absolute path.
	taskRelPath := taskFile
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, taskFile); err == nil {
			taskRelPath = rel
		}
	}
	if cfg.RunID == "" {
		cfg.RunID = generateRunID()
	}

	// Check shutdown flag before starting the iteration
	select {
	case <-cfg.Shutdown:
		return ErrInterrupted
	default:
	}

	formatter := NewLoopFormatter(cfg.Stderr)
	formatter.IterationHeader(iteration, maxIter, taskRelPath, cfg.Label)

	onFailure := cfg.OnFailure
	if onFailure == "" {
		onFailure = OnFailureStop
	}

	// Run agent-before; skip iteration on failure
	if cfg.AgentBefore != "" {
		formatter.PhaseAgentHeader("before")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentBefore)
		}
		env := phaseEnv{phase: "before", iteration: iteration, maxIter: maxIter, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentBefore, env, cfg.Stderr); err != nil {
			if errors.Is(err, ErrInterrupted) {
				return ErrInterrupted
			}
			return fmt.Errorf("agent-before failed (iteration %d): %w", iteration, err)
		}
	}

	// Run cmd-before; log warning but don't fail on error
	if cfg.CmdBefore != "" {
		formatter.CmdHookMarker("cmd-before", cfg.CmdBefore)
		if err := runHook(cfg.CmdBefore, hookEnv{iteration: iteration, maxIterations: maxIter, runID: cfg.RunID, label: cfg.Label, model: cfg.Model, provider: cfg.Provider}, cfg.Stderr); err != nil {
			fmt.Fprintf(cfg.Stderr, "cmd-before failed (iteration %d): %v\n", iteration, err)
			return nil // skip iteration, continue to next
		}
	}

	start := time.Now()

	// Re-read task file
	contents, err := os.ReadFile(taskFile)
	if err != nil {
		// File gone (agent deleted it) means task complete
		if os.IsNotExist(err) {
			return errFileGone
		}
		return fmt.Errorf("reading task file %s: %w", taskRelPath, err)
	}

	content := cfg.Content
	if state.retryCount > 0 && cfg.RetryPrompt != "" {
		content = cfg.RetryPrompt + "\n\n" + content
	}
	prompt := BuildWatchPrompt(string(contents), content, taskRelPath, iteration, maxIter)
	printVerboseProviderCommand(cfg, prompt)
	opts := buildRunOptions(cfg, prompt)
	opts.Env = append(opts.Env, buildJuggleEnv(cfg.RunID, iteration, maxIter, cfg.Label, cfg.Model, cfg.Provider, taskFile, -1)...)

	writeIterStartLog(cfg.Log, iterStartLogEntry{
		RunID:     cfg.RunID,
		Iteration: iteration,
		WorkerID:  cfg.WorkerID,
		TaskFile:  taskFile,
	})

	result, err := cfg.Runner.Run(opts)
	if err != nil {
		return fmt.Errorf("runner error on iteration %d of %s: %w", iteration, taskRelPath, err)
	}

	// Handle overload exhausted
	if result.OverloadExhausted {
		return fmt.Errorf("agent exhausted overload retries on iteration %d of %s", iteration, taskRelPath)
	}

	// Handle quota/usage exhaustion — wait and continue to next iteration
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
			wait = state.backoff
			waitMsg = fmt.Sprintf("usage quota hit (reset time unknown), waiting %s", formatWaitDuration(wait))
			state.incrementBackoff()
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

		// Quota recovered, continue to next iteration
		return nil
	}

	// Handle rate limiting — wait and continue to next iteration
	if result.RateLimited {
		wait := state.backoff
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

		state.incrementBackoff()
		return nil
	}

	// Handle non-zero exit code according to --on-failure mode
	if result.ExitCode != 0 {
		switch onFailure {
		case OnFailureStop:
			return fmt.Errorf("iteration %d failed (exit code %d)", iteration, result.ExitCode)

		case OnFailureRetry:
			maxRetries := cfg.Retries
			if maxRetries == 0 {
				maxRetries = 2
			}
			if state.retryCount < maxRetries {
				bd := retryBackoffFor(state.retryCount, cfg.RetryBackoffs)
				fmt.Fprintf(cfg.Stderr, "iteration %d failed (exit code %d), next attempt in %v (attempt %d/%d)\n",
					iteration, result.ExitCode, bd, state.retryCount+1, maxRetries)
				select {
				case <-time.After(bd):
				case <-cfg.Shutdown:
					return ErrInterrupted
				}
				state.retryCount++
			} else {
				state.retryCount = 0
				fmt.Fprintf(cfg.Stderr, "iteration %d failed after %d retries, continuing\n", iteration, maxRetries)
			}

		case OnFailureContinue:
			fmt.Fprintf(cfg.Stderr, "iteration %d failed (exit code %d), continuing\n", iteration, result.ExitCode)
		}

		// Consecutive failure tracking (continue/retry-exhausted)
		state.consecutiveFailures++
		if cfg.MaxFailures > 0 && state.consecutiveFailures >= cfg.MaxFailures {
			return fmt.Errorf("stopping: %d consecutive failures", state.consecutiveFailures)
		}
	} else {
		state.consecutiveFailures = 0
		state.retryCount = 0
	}

	// Reset backoff, accumulate stats, and print status
	state.resetBackoff()
	if stats != nil {
		stats.iterations++
		stats.inputTokens += result.InputTokens
		stats.outputTokens += result.OutputTokens
		stats.cacheTokens += result.CacheTokens
	}
	formatter.IterationStatus(time.Since(start), result.InputTokens, result.OutputTokens, result.CacheTokens)
	if cfg.OnIterDone != nil {
		cfg.OnIterDone(iteration, maxIter)
	}

	var errStr *string
	if result.Error != nil {
		s := result.Error.Error()
		errStr = &s
	}
	writeIterationLog(cfg.Log, iterationLogEntry{
		RunID:        cfg.RunID,
		Iteration:    iteration,
		WorkerID:     cfg.WorkerID,
		DurationMs:   time.Since(start).Milliseconds(),
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		CacheTokens:  result.CacheTokens,
		ExitCode:     result.ExitCode,
		RateLimited:  result.RateLimited,
		Error:        errStr,
	})

	// Run agent-after; log warning on failure but continue
	if cfg.AgentAfter != "" {
		formatter.PhaseAgentHeader("after")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentAfter)
		}
		env := phaseEnv{phase: "after", iteration: iteration, maxIter: maxIter, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentAfter, env, cfg.Stderr); err != nil {
			if errors.Is(err, ErrInterrupted) {
				return ErrInterrupted
			}
			fmt.Fprintf(cfg.Stderr, "agent-after failed (iteration %d): %v\n", iteration, err)
		}
	}

	// Run cmd-after; log warning on failure but continue
	if cfg.CmdAfter != "" {
		formatter.CmdHookMarker("cmd-after", cfg.CmdAfter)
		afterEnv := hookEnv{
			iteration:     iteration,
			maxIterations: maxIter,
			exitCode:      result.ExitCode,
			inputTokens:   result.InputTokens,
			outputTokens:  result.OutputTokens,
			runID:         cfg.RunID,
			label:         cfg.Label,
			model:         cfg.Model,
			provider:      cfg.Provider,
		}
		if err := runHook(cfg.CmdAfter, afterEnv, cfg.Stderr); err != nil {
			fmt.Fprintf(cfg.Stderr, "cmd-after failed (iteration %d): %v\n", iteration, err)
		}
	}

	// Run stop-when; exit 0 means stop gracefully
	if cfg.StopWhen != "" {
		stopEnv := hookEnv{
			iteration:     iteration,
			maxIterations: maxIter,
			exitCode:      result.ExitCode,
			inputTokens:   result.InputTokens,
			outputTokens:  result.OutputTokens,
			runID:         cfg.RunID,
			label:         cfg.Label,
			model:         cfg.Model,
			provider:      cfg.Provider,
		}
		if err := runHook(cfg.StopWhen, stopEnv, cfg.Stderr); err == nil {
			fmt.Fprintf(cfg.Stderr, "stop-when condition met after iteration %d, stopping\n", iteration)
			return nil
		}
	}

	// Check cost guard after each iteration
	if cfg.MaxCost > 0 && stats != nil {
		cost := estimateCost(stats.inputTokens, stats.outputTokens, stats.model)
		if cost > cfg.MaxCost {
			fmt.Fprintf(cfg.Stderr, "cost guard triggered: estimated $%.4f exceeds --max-cost $%.4f\n", cost, cfg.MaxCost)
			return errCostGuard
		}
	}

	return nil
}

// runWatchWorkers spawns cfg.Workers goroutines, each picking task files from
// the watch directory via a shared coordinator to prevent duplicate selection.
// Each worker prefixes its stderr output with [worker-N], or writes to a log
// file when the dashboard is active.
func runWatchWorkers(cfg Config) error {
	watchDir := cfg.Watch[0]

	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	var dash *workerDashboard
	if cfg.Dashboard {
		dash = setupWorkerDashboard(watchDir, cfg.Workers, cfg.Stderr)
		defer dash.stop()
	}

	coord := newWorkerCoordinator()
	var touchTrack *touchTracker
	if cfg.OnTouch {
		touchTrack = newTouchTracker()
	}
	errs := make(chan error, cfg.Workers)
	var wg sync.WaitGroup

	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerCfg := cfg
			workerCfg.WorkerID = workerID + 1
			if dash != nil {
				logFile, closeLog := dash.openWorkerLog(workerID)
				defer closeLog()
				if logFile != nil {
					workerCfg.Stderr = logFile
				}
			} else {
				workerCfg.Stderr = newPrefixWriter(cfg.Stderr, fmt.Sprintf("[worker-%d] ", workerID))
			}
			workerCfg.Runner = &workerIDRunner{inner: cfg.Runner, workerID: workerID}
			if err := runWorkerLoop(workerCfg, coord, touchTrack, pollDelay, dash, workerID); err != nil {
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

// runWorkerLoop is the per-worker iteration loop.
// dash and workerID are used to update the dashboard when non-nil.
func runWorkerLoop(cfg Config, coord *workerCoordinator, touchTrack *touchTracker, pollDelay time.Duration, dash *workerDashboard, workerID int) error {
	stats := runStats{runID: cfg.RunID, start: time.Now(), model: cfg.Model}

	logFile := ""
	if dash != nil && workerID >= 0 && workerID < len(dash.logFiles) {
		logFile = dash.logFiles[workerID]
	}

	// Run agent-pre once before the iteration loop
	if cfg.AgentPre != "" {
		formatter := NewLoopFormatter(cfg.Stderr)
		formatter.PhaseAgentHeader("pre")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentPre)
		}
		env := phaseEnv{phase: "pre", iteration: 0, maxIter: cfg.Iterations, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentPre, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-pre failed: %w", err)
		}
	}

	max := cfg.Iterations
	taskState := newRunTaskState()

	for i := 1; max == 0 || i <= max; i++ {
		// Check shutdown before starting each iteration
		select {
		case <-cfg.Shutdown:
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		var triggerMsg string
		if cfg.EffectiveID != "" {
			if msg, err := ReadTrigger(cfg.EffectiveID); err == nil && msg != "" {
				triggerMsg = msg
			}
		}

		var taskPath string
		var err error
		if triggerMsg == "" {
			if touchTrack != nil {
				taskPath, err = touchTrack.claimTouchDir(cfg.Watch[0], coord)
			} else {
				taskPath, err = coord.claim(cfg.Watch[0])
			}
			if err != nil {
				return err
			}
		}

		if taskPath == "" && triggerMsg == "" {
			if dash != nil {
				dash.dash.Update(workerID, WorkerState{Status: WorkerIdle, LogFile: logFile})
			}
			pollWaitWithWake(cfg.Stderr, "Waiting for tasks", pollDelay, cfg.Shutdown, wakeCh(&cfg))
			continue
		}

		var filename string
		taskCfg := cfg
		if triggerMsg != "" {
			filename = "trigger"
			taskCfg.Content = taskCfg.Content + "\n\n" + FormatTrigger(triggerMsg)
		} else {
			filename = filepath.Base(taskPath)
		}
		if dash != nil {
			dash.dash.Update(workerID, WorkerState{
				Status:    WorkerActive,
				TaskName:  filename,
				Iteration: i,
				MaxIter:   max,
				LogFile:   logFile,
			})
			taskCfg := cfg
			taskCfg.OnIterDone = func(iter, maxIter int) {
				dash.dash.Update(workerID, WorkerState{
					Status:    WorkerActive,
					TaskName:  filename,
					Iteration: iter,
					MaxIter:   maxIter,
					LogFile:   logFile,
				})
			}
			cfg = taskCfg
		}
		fmt.Fprintf(cfg.Stderr, "Processing task: %s\n", filename)

		if triggerMsg != "" {
			err = runTriggerTask(taskCfg, i, max, taskState, &stats)
		} else {
			err = runWatchTask(cfg, taskPath, i, max, taskState, &stats)
		}
		if err != nil {
			if triggerMsg == "" {
				coord.release(taskPath)
			}
			if dash != nil {
				dash.dash.Update(workerID, WorkerState{Status: WorkerIdle, LogFile: logFile})
			}
			if errors.Is(err, ErrInterrupted) {
				writeSummary(cfg, stats)
				return ErrInterrupted
			}
			if errors.Is(err, errCostGuard) {
				writeSummary(cfg, stats)
				return nil
			}
			if errors.Is(err, errFileGone) {
				// File completed by agent, continue to next iteration
				continue
			}
			fmt.Fprintf(cfg.Stderr, "Error processing %s: %v\n", filename, err)
			continue
		}
		if triggerMsg == "" {
			coord.release(taskPath)
		}
		if dash != nil {
			dash.dash.Update(workerID, WorkerState{Status: WorkerIdle, LogFile: logFile})
		}

		// Wait between iterations (skip after last), interruptible by shutdown or wake
		if (max == 0 || i < max) && (cfg.Delay > 0 || cfg.Fuzz > 0) {
			d := computeDelay(cfg.Delay, cfg.Fuzz)
			if d > 0 {
				fmt.Fprintf(cfg.Stderr, "waiting %v before next iteration\n", d)
				select {
				case <-time.After(d):
				case <-cfg.Shutdown:
					writeSummary(cfg, stats)
					return ErrInterrupted
				case <-wakeCh(&cfg):
					fmt.Fprintf(cfg.Stderr, "wake signal received, starting next iteration\n")
				}
			}
		}
	}

	// Run agent-post once after the iteration loop
	if cfg.AgentPost != "" {
		formatter := NewLoopFormatter(cfg.Stderr)
		formatter.PhaseAgentHeader("post")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentPost)
		}
		env := phaseEnv{phase: "post", iteration: 0, maxIter: max, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentPost, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-post failed: %w", err)
		}
	}

	writeSummary(cfg, stats)
	return nil
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
		SystemPrompt:        cfg.SystemPrompt,
		WorkingDir:        cfg.WorkDir,
		CommandOverride:  cfg.Command,
	}
}

// runTouchWatch watches a single file for mtime changes (touch events).
// When the file's mtime changes, an agent iteration is triggered.
// Touches during an active iteration are coalesced: N touches → 1 re-run.
// The file is a signal only; the prompt comes from cfg.Content.
func runTouchWatch(cfg Config) error {
	triggerFile := cfg.Watch[0]

	info, err := os.Stat(triggerFile)
	if err != nil {
		return fmt.Errorf("touch watch file: %w", err)
	}

	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	lastMod := info.ModTime()
	var dirty bool

	stats := runStats{runID: cfg.RunID, start: time.Now(), model: cfg.Model}

	if cfg.AgentPre != "" {
		formatter := NewLoopFormatter(cfg.Stderr)
		formatter.PhaseAgentHeader("pre")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentPre)
		}
		env := phaseEnv{phase: "pre", iteration: 0, maxIter: cfg.Iterations, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentPre, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-pre failed: %w", err)
		}
	}

	max := cfg.Iterations
	taskState := newRunTaskState()

	for i := 1; max == 0 || i <= max; i++ {
		select {
		case <-cfg.Shutdown:
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		info, err := os.Stat(triggerFile)
		if err != nil {
			if os.IsNotExist(err) {
				pollWait(cfg.Stderr, fmt.Sprintf("Waiting for %s (deleted)", triggerFile), pollDelay, cfg.Shutdown)
				continue
			}
			return fmt.Errorf("stat touch file: %w", err)
		}

		if info.ModTime().Equal(lastMod) && !dirty {
			pollWait(cfg.Stderr, fmt.Sprintf("Watching %s", triggerFile), pollDelay, cfg.Shutdown)
			continue
		}

		dirty = false
		lastMod = info.ModTime()

		triggerRelPath := triggerFile
		if wd, wdErr := os.Getwd(); wdErr == nil {
			if rel, relErr := filepath.Rel(wd, triggerFile); relErr == nil {
				triggerRelPath = rel
			}
		}

		if cfg.RunID == "" {
			cfg.RunID = generateRunID()
		}

		formatter := NewLoopFormatter(cfg.Stderr)
		formatter.IterationHeader(i, max, triggerRelPath, cfg.Label)

		onFailure := cfg.OnFailure
		if onFailure == "" {
			onFailure = OnFailureStop
		}

		if cfg.AgentBefore != "" {
			formatter.PhaseAgentHeader("before")
			if cfg.Verbose {
				fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentBefore)
			}
			env := phaseEnv{phase: "before", iteration: i, maxIter: max, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
			if err := runPhaseAgent(cfg, cfg.AgentBefore, env, cfg.Stderr); err != nil {
				if errors.Is(err, ErrInterrupted) {
					return ErrInterrupted
				}
				return fmt.Errorf("agent-before failed (iteration %d): %w", i, err)
			}
		}

		if cfg.CmdBefore != "" {
			formatter.CmdHookMarker("cmd-before", cfg.CmdBefore)
			if err := runHook(cfg.CmdBefore, hookEnv{iteration: i, maxIterations: max, runID: cfg.RunID, label: cfg.Label, model: cfg.Model, provider: cfg.Provider}, cfg.Stderr); err != nil {
				fmt.Fprintf(cfg.Stderr, "cmd-before failed (iteration %d): %v\n", i, err)
				continue
			}
		}

		start := time.Now()

		content := cfg.Content
		if taskState.retryCount > 0 && cfg.RetryPrompt != "" {
			content = cfg.RetryPrompt + "\n\n" + content
		}
		prompt := buildTouchPrompt(content, triggerRelPath, i, max)
		printVerboseProviderCommand(cfg, prompt)
		opts := buildRunOptions(cfg, prompt)
		opts.Env = append(opts.Env, buildJuggleEnv(cfg.RunID, i, max, cfg.Label, cfg.Model, cfg.Provider, triggerFile, -1)...)

		result, err := cfg.Runner.Run(opts)
		if err != nil {
			return fmt.Errorf("runner error on iteration %d of %s: %w", i, triggerRelPath, err)
		}

		if result.OverloadExhausted {
			return fmt.Errorf("agent exhausted overload retries on iteration %d of %s", i, triggerRelPath)
		}

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
				wait = taskState.backoff
				waitMsg = fmt.Sprintf("usage quota hit (reset time unknown), waiting %s", formatWaitDuration(wait))
				taskState.incrementBackoff()
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
			continue
		}

		if result.RateLimited {
			wait := taskState.backoff
			if result.RetryAfter > 0 {
				wait = result.RetryAfter
			}

			if cfg.MaxWait > 0 && wait > cfg.MaxWait {
				return fmt.Errorf("rate limited: wait %v exceeds max-wait %v", wait, cfg.MaxWait)
			}

			fmt.Fprintf(cfg.Stderr, "rate limited, waiting %v before retry\n", wait)
			select {
			case <-time.After(wait):
			case <-cfg.Shutdown:
				return ErrInterrupted
			}

			taskState.incrementBackoff()
			continue
		}

		if result.ExitCode != 0 {
			switch onFailure {
			case OnFailureStop:
				return fmt.Errorf("iteration %d failed (exit code %d)", i, result.ExitCode)
			case OnFailureRetry:
				maxRetries := cfg.Retries
				if maxRetries == 0 {
					maxRetries = 2
				}
				if taskState.retryCount < maxRetries {
					bd := retryBackoffFor(taskState.retryCount, cfg.RetryBackoffs)
					fmt.Fprintf(cfg.Stderr, "iteration %d failed (exit code %d), next attempt in %v (attempt %d/%d)\n",
						i, result.ExitCode, bd, taskState.retryCount+1, maxRetries)
					select {
					case <-time.After(bd):
					case <-cfg.Shutdown:
						return ErrInterrupted
					}
					taskState.retryCount++
				} else {
					taskState.retryCount = 0
					fmt.Fprintf(cfg.Stderr, "iteration %d failed after %d retries, continuing\n", i, maxRetries)
				}
			case OnFailureContinue:
				fmt.Fprintf(cfg.Stderr, "iteration %d failed (exit code %d), continuing\n", i, result.ExitCode)
			}

			taskState.consecutiveFailures++
			if cfg.MaxFailures > 0 && taskState.consecutiveFailures >= cfg.MaxFailures {
				return fmt.Errorf("stopping: %d consecutive failures", taskState.consecutiveFailures)
			}
		} else {
			taskState.consecutiveFailures = 0
			taskState.retryCount = 0
		}

		taskState.resetBackoff()
		stats.iterations++
		stats.inputTokens += result.InputTokens
		stats.outputTokens += result.OutputTokens
		stats.cacheTokens += result.CacheTokens
		formatter.IterationStatus(time.Since(start), result.InputTokens, result.OutputTokens, result.CacheTokens)

		if cfg.AgentAfter != "" {
			formatter.PhaseAgentHeader("after")
			if cfg.Verbose {
				fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentAfter)
			}
			env := phaseEnv{phase: "after", iteration: i, maxIter: max, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
			if err := runPhaseAgent(cfg, cfg.AgentAfter, env, cfg.Stderr); err != nil {
				if errors.Is(err, ErrInterrupted) {
					return ErrInterrupted
				}
				fmt.Fprintf(cfg.Stderr, "agent-after failed (iteration %d): %v\n", i, err)
			}
		}

		if cfg.CmdAfter != "" {
			formatter.CmdHookMarker("cmd-after", cfg.CmdAfter)
			afterEnv := hookEnv{
				iteration:     i,
				maxIterations: max,
				exitCode:      result.ExitCode,
				inputTokens:   result.InputTokens,
				outputTokens:  result.OutputTokens,
				runID:         cfg.RunID,
				label:         cfg.Label,
				model:         cfg.Model,
				provider:      cfg.Provider,
			}
			if err := runHook(cfg.CmdAfter, afterEnv, cfg.Stderr); err != nil {
				fmt.Fprintf(cfg.Stderr, "cmd-after failed (iteration %d): %v\n", i, err)
			}
		}

		if cfg.StopWhen != "" {
			stopEnv := hookEnv{
				iteration:     i,
				maxIterations: max,
				exitCode:      result.ExitCode,
				inputTokens:   result.InputTokens,
				outputTokens:  result.OutputTokens,
				runID:         cfg.RunID,
				label:         cfg.Label,
				model:         cfg.Model,
				provider:      cfg.Provider,
			}
			if err := runHook(cfg.StopWhen, stopEnv, cfg.Stderr); err == nil {
				fmt.Fprintf(cfg.Stderr, "stop-when condition met after iteration %d, stopping\n", i)
				return nil
			}
		}

		if cfg.MaxCost > 0 {
			cost := estimateCost(stats.inputTokens, stats.outputTokens, stats.model)
			if cost > cfg.MaxCost {
				fmt.Fprintf(cfg.Stderr, "cost guard triggered: estimated $%.4f exceeds --max-cost $%.4f\n", cost, cfg.MaxCost)
				return nil
			}
		}

		// After iteration: poll for new touches with dirty coalescing.
		// Re-stat the file; if mtime changed during the iteration, set dirty
		// so the next loop iteration fires immediately.
		if postInfo, postErr := os.Stat(triggerFile); postErr == nil {
			if !postInfo.ModTime().Equal(lastMod) {
				dirty = true
				lastMod = postInfo.ModTime()
			}
		}
	}

	if cfg.AgentPost != "" {
		formatter := NewLoopFormatter(cfg.Stderr)
		formatter.PhaseAgentHeader("post")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentPost)
		}
		env := phaseEnv{phase: "post", iteration: 0, maxIter: max, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentPost, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-post failed: %w", err)
		}
	}

	writeSummary(cfg, stats)
	return nil
}

// buildTouchPrompt builds the prompt for a touch-triggered iteration.
// It prepends a line indicating which trigger file started the loop.
func buildTouchPrompt(content, triggerRelPath string, iteration, maxIterations int) string {
	return fmt.Sprintf("Triggered by touch on: %s\n\n%s\n\n---\nThis is iteration %d of %s.",
		triggerRelPath, content, iteration, maxStr(maxIterations))
}
