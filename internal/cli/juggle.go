package cli

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/agent/provider"
	"github.com/spf13/cobra"
)

var version = "dev"

// SetVersion sets the version string (injected at build time).
func SetVersion(v string) { version = v }

// Config holds all CLI configuration for a juggle run.
type Config struct {
	Content      string        // Resolved prompt content (joined)
	Watch        string        // Watch directory path
	Iterations   int           // Max iterations (0 = unlimited)
	Model        string        // Model name
	Provider     string        // Provider name
	Delay        int           // Minutes between iterations
	Fuzz         int           // +/- random variance in minutes
	Trust        bool          // Bypass permission checks
	Interactive  bool          // Interactive TUI mode
	Timeout      time.Duration // Per-iteration timeout
	MaxWait      time.Duration // Max rate limit wait
	DryRun       bool          // Show prompt, don't run
	ShowThinking bool          // Show thinking blocks

	Runner agent.Runner // Injected runner (nil = build from Provider flag)
	Stdout io.Writer
	Stderr io.Writer
}

// flags is used for cobra flag binding.
var flags struct {
	watch        string
	iterations   int
	model        string
	provider     string
	delay        int
	fuzz         int
	trust        bool
	interactive  bool
	timeout      time.Duration
	maxWait      time.Duration
	dryRun       bool
	showThinking bool
}

func init() {
	f := rootCmd.Flags()
	f.StringVar(&flags.watch, "watch", "", "watch directory for task files")
	f.IntVarP(&flags.iterations, "iterations", "n", 10, "max iterations (0 = unlimited)")
	f.StringVar(&flags.model, "model", "sonnet", "model name")
	f.StringVar(&flags.provider, "provider", "claude", "provider name")
	f.IntVar(&flags.delay, "delay", 0, "minutes between iterations")
	f.IntVar(&flags.fuzz, "fuzz", 0, "+/- random variance in minutes")
	f.BoolVar(&flags.trust, "trust", false, "bypass permission checks")
	f.BoolVar(&flags.interactive, "interactive", false, "interactive TUI mode")
	f.DurationVar(&flags.timeout, "timeout", 0, "per-iteration timeout")
	f.DurationVar(&flags.maxWait, "max-wait", 0, "max rate limit wait")
	f.BoolVar(&flags.dryRun, "dry-run", false, "show prompt, don't run")
	f.BoolVar(&flags.showThinking, "show-thinking", false, "show thinking blocks")
}

var rootCmd = &cobra.Command{
	Use:   "juggle [prompt-content...]",
	Short: "Minimal agent loop runner",
	Long:  "Run an AI agent in a loop. All positional args are prompt content (strings or @file references).",
	Args: func(cmd *cobra.Command, args []string) error {
		watchFlag, _ := cmd.Flags().GetString("watch")
		if watchFlag == "" && len(args) == 0 {
			return fmt.Errorf("requires at least 1 arg when --watch is not set")
		}
		return nil
	},
	RunE:         run,
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// run is the cobra RunE handler.
func run(cmd *cobra.Command, args []string) error {
	resolved, err := ResolveArgs(args)
	if err != nil {
		return err
	}

	cfg := Config{
		Content:      strings.Join(resolved, "\n\n"),
		Watch:        flags.watch,
		Iterations:   flags.iterations,
		Model:        flags.model,
		Provider:     flags.provider,
		Delay:        flags.delay,
		Fuzz:         flags.fuzz,
		Trust:        flags.trust,
		Interactive:  flags.interactive,
		Timeout:      flags.timeout,
		MaxWait:      flags.maxWait,
		DryRun:       flags.dryRun,
		ShowThinking: flags.showThinking,
	}

	// Build runner from provider flag
	p := provider.Get(provider.Type(cfg.Provider))
	cfg.Runner = &agent.ProviderRunner{Provider: p}

	return Run(cfg)
}

// Run is the main entry point for juggle execution.
func Run(cfg Config) error {
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}

	if cfg.DryRun {
		prompt := BuildPrompt(cfg.Content, 1, cfg.Iterations)
		fmt.Fprint(cfg.Stdout, prompt)
		return nil
	}

	if cfg.Watch != "" {
		return RunWatch(cfg)
	}

	return RunLoop(cfg)
}

// RunLoop runs the agent in a loop for the configured number of iterations.
func RunLoop(cfg Config) error {
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}

	max := cfg.Iterations

	// Rate limit backoff state
	const initialBackoff = 30 * time.Second
	const maxBackoff = 10 * time.Minute
	backoff := initialBackoff

	for i := 1; max == 0 || i <= max; i++ {
		prompt := BuildPrompt(cfg.Content, i, max)
		opts := buildRunOptions(cfg, prompt)

		result, err := cfg.Runner.Run(opts)
		if err != nil {
			return fmt.Errorf("runner error on iteration %d: %w", i, err)
		}

		// Handle overload exhausted
		if result.OverloadExhausted {
			return fmt.Errorf("agent exhausted overload retries on iteration %d", i)
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

// computeDelay returns the duration to wait between iterations.
// It adds random fuzz in the range [-fuzz, +fuzz] minutes, clamped to >= 0.
func computeDelay(delayMinutes, fuzzMinutes int) time.Duration {
	if delayMinutes == 0 && fuzzMinutes == 0 {
		return 0
	}

	total := delayMinutes
	if fuzzMinutes > 0 {
		// Random value in [-fuzz, +fuzz]
		total += rand.Intn(2*fuzzMinutes+1) - fuzzMinutes
	}

	if total < 0 {
		total = 0
	}

	return time.Duration(total) * time.Minute
}

// BuildPrompt joins content with an iteration footer.
func BuildPrompt(content string, iteration, maxIterations int) string {
	return fmt.Sprintf("%s\n\n---\nThis is iteration %d of %s.\n", content, iteration, maxStr(maxIterations))
}

// BuildWatchPrompt wraps task file contents with content and footer.
func BuildWatchPrompt(taskContents, content, filename string, iteration, maxIterations int) string {
	return fmt.Sprintf("<task>\n%s\n</task>\n\n%s\n\n---\nThis is iteration %d of %s, processing %s.\n",
		taskContents, content, iteration, maxStr(maxIterations), filename)
}

func maxStr(max int) string {
	if max == 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", max)
}
