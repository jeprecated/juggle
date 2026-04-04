package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ohare93/juggle/internal/agent"
)

// ScanWatchDir returns the path to the first non-hidden regular file.
// Returns empty string if no eligible files found.
// Files are in alphabetical order (os.ReadDir sorts by name).
func ScanWatchDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading watch directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		return filepath.Join(dir, entry.Name()), nil
	}

	return "", nil
}

// RunWatch processes task files from a watched directory.
// For each file, reads contents, runs iterations, then picks next.
// Idles when empty, polling at delay interval (minimum 30 seconds).
func RunWatch(cfg Config) error {
	info, err := os.Stat(cfg.Watch)
	if err != nil {
		return fmt.Errorf("watch directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("watch path is not a directory: %s", cfg.Watch)
	}

	// Calculate poll delay: max(delay_minutes, 30s)
	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	stats := runStats{start: time.Now()}

	for {
		// Check shutdown before starting each scan/task cycle
		select {
		case <-cfg.Shutdown:
			printRunSummary(cfg.Stderr, stats)
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
				printRunSummary(cfg.Stderr, stats)
				return ErrInterrupted
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
	max := cfg.Iterations
	formatter := NewLoopFormatter(cfg.Stderr)
	consecutiveFailures := 0

	// Rate limit backoff state
	const initialBackoff = 30 * time.Second
	const maxBackoff = 10 * time.Minute
	backoff := initialBackoff

	for i := 1; max == 0 || i <= max; i++ {
		// Check shutdown flag before starting each new iteration
		select {
		case <-cfg.Shutdown:
			return ErrInterrupted
		default:
		}

		formatter.IterationHeader(i, max, filename)
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

		result, err := cfg.Runner.Run(opts)
		if err != nil {
			return fmt.Errorf("runner error on iteration %d of %s: %w", i, filename, err)
		}

		// Handle overload exhausted
		if result.OverloadExhausted {
			return fmt.Errorf("agent exhausted overload retries on iteration %d of %s", i, filename)
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

		// Track consecutive failures (non-zero exit code)
		if result.ExitCode != 0 {
			consecutiveFailures++
			if cfg.MaxFailures > 0 && consecutiveFailures >= cfg.MaxFailures {
				return fmt.Errorf("stopping: %d consecutive failures", consecutiveFailures)
			}
		} else {
			consecutiveFailures = 0
		}

		// Success: reset backoff, accumulate stats, and print status
		backoff = initialBackoff
		if stats != nil {
			stats.iterations++
			stats.inputTokens += result.InputTokens
			stats.outputTokens += result.OutputTokens
			stats.cacheTokens += result.CacheTokens
		}
		formatter.IterationStatus(time.Since(start), result.InputTokens, result.OutputTokens, result.CacheTokens)

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

	return nil
}

// buildRunOptions creates RunOptions from Config and a prompt.
func buildRunOptions(cfg Config, prompt string) agent.RunOptions {
	mode := agent.ModeHeadless
	if cfg.Interactive {
		mode = agent.ModeInteractive
	}

	perm := agent.PermissionAcceptEdits
	if cfg.Trust {
		perm = agent.PermissionBypass
	}

	return agent.RunOptions{
		Prompt:       prompt,
		Mode:         mode,
		Permission:   perm,
		Model:        cfg.Model,
		Timeout:      cfg.Timeout,
		ShowThinking: cfg.ShowThinking,
		Verbose:      cfg.Verbose,
		Context:      cfg.ForceCtx,
	}
}
