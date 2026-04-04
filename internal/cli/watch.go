package cli

import (
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

	for {
		taskPath, err := ScanWatchDir(cfg.Watch)
		if err != nil {
			return err
		}

		if taskPath == "" {
			fmt.Fprintf(cfg.Stderr, "Watch directory empty, polling in %v...\n", pollDelay)
			time.Sleep(pollDelay)
			continue
		}

		filename := filepath.Base(taskPath)
		fmt.Fprintf(cfg.Stderr, "Processing task: %s\n", filename)

		if err := runWatchTask(cfg, taskPath, filename); err != nil {
			fmt.Fprintf(cfg.Stderr, "Error processing %s: %v\n", filename, err)
			continue
		}
	}
}

// runWatchTask runs the iteration loop for a single watch task file.
// Re-reads the task file each iteration to pick up agent-appended progress.
func runWatchTask(cfg Config, taskFile, filename string) error {
	max := cfg.Iterations

	// Rate limit backoff state
	const initialBackoff = 30 * time.Second
	const maxBackoff = 10 * time.Minute
	backoff := initialBackoff

	for i := 1; max == 0 || i <= max; i++ {
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
			time.Sleep(wait)

			// Double backoff for next time, cap at maxBackoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			// Retry same iteration
			i--
			continue
		}

		// Success: reset backoff
		backoff = initialBackoff

		// Wait between iterations (skip after last)
		if (max == 0 || i < max) && (cfg.Delay > 0 || cfg.Fuzz > 0) {
			d := computeDelay(cfg.Delay, cfg.Fuzz)
			if d > 0 {
				fmt.Fprintf(cfg.Stderr, "waiting %v before next iteration\n", d)
				time.Sleep(d)
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
	}
}
