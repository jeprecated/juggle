package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/agent/provider"
	"github.com/spf13/cobra"
)

var version = "dev"

// SetVersion sets the version string (injected at build time).
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

// ErrInterrupted is returned when the run is stopped by a signal.
var ErrInterrupted = errors.New("interrupted by signal")

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
	Verbose      bool          // Show tool inputs in headless output

	// Shutdown is closed when the first signal arrives (graceful shutdown).
	// A nil channel means no shutdown signaling (normal operation).
	Shutdown <-chan struct{}
	// ForceCtx is cancelled on the second signal to kill the child process.
	// A nil context means no force-kill context.
	ForceCtx context.Context

	Runner agent.Runner // Injected runner (nil = build from Provider flag)
	Stdout io.Writer
	Stderr io.Writer
}

// runStats tracks cumulative metrics across iterations for the run summary.
type runStats struct {
	iterations   int
	inputTokens  int
	outputTokens int
	cacheTokens  int
	start        time.Time
}

// printRunSummary writes a summary line to w after a signal-interrupted run.
func printRunSummary(w io.Writer, stats runStats) {
	elapsed := time.Since(stats.start)
	var timing string
	if elapsed < time.Second {
		timing = fmt.Sprintf("%dms", elapsed.Milliseconds())
	} else {
		timing = fmt.Sprintf("%ds", int(elapsed.Seconds()))
	}

	tok := fmt.Sprintf("%d in / %d out", stats.inputTokens, stats.outputTokens)
	if stats.cacheTokens > 0 {
		tok += fmt.Sprintf(" (%d cached)", stats.cacheTokens)
	}

	fmt.Fprintf(w, "Run summary: %d iteration(s), %s, %s\n", stats.iterations, tok, timing)
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
	verbose      bool
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
	f.BoolVarP(&flags.verbose, "verbose", "v", false, "show tool inputs in output")

	// Hide less-common flags to reduce noise in default help output
	_ = f.MarkHidden("fuzz")
	_ = f.MarkHidden("interactive")
	_ = f.MarkHidden("show-thinking")
	_ = f.MarkHidden("provider")

	rootCmd.AddCommand(completionCmd)
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion script",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		default:
			return fmt.Errorf("unsupported shell: %s (use bash, zsh, or fish)", args[0])
		}
	},
}

var rootCmd = &cobra.Command{
	Use:     "juggle [prompt-content...]",
	Short:   "Minimal agent loop runner",
	Version: "dev",
	Long: `Run an AI agent in a loop. All positional args are prompt content (strings or @file references).

Shell completion:
  juggle completion bash > /etc/bash_completion.d/juggle
  juggle completion zsh > ~/.zshrc
  juggle completion fish > ~/.config/fish/completions/juggle.fish`,
	Example: `  # Basic: run 3 iterations
  juggle "fix the failing tests" -n 3

  # With a prompt file and trust mode
  juggle @task.md --trust -n 10

  # Watch mode: pick up tasks from a directory
  juggle --watch ./tasks/ @rules.md

  # With hooks: run tests after each iteration, stop when they pass
  juggle --cmd-after "npm test" --stop-when "npm test" @task.md

  # Multi-phase: run a tidy agent after each iteration
  juggle --agent-after @tidy @task.md -n 5`,
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: completeArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		watchFlag, _ := cmd.Flags().GetString("watch")
		if watchFlag == "" && len(args) == 0 {
			return cmd.Help()
		}
		return run(cmd, args)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
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
		Verbose:      flags.verbose,
	}

	// Build runner from provider flag
	p := provider.Get(provider.Type(cfg.Provider))
	cfg.Runner = &agent.ProviderRunner{Provider: p}

	// Set up signal handling: first signal = graceful shutdown, second = force kill
	shutdown := make(chan struct{})
	var shutdownOnce sync.Once
	forceCtx, forceCancel := context.WithCancel(context.Background())
	defer forceCancel()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		<-sigCh
		shutdownOnce.Do(func() { close(shutdown) })
		<-sigCh
		// Second signal: cancel child context then exit
		forceCancel()
		time.Sleep(200 * time.Millisecond)
		os.Exit(130)
	}()

	cfg.Shutdown = shutdown
	cfg.ForceCtx = forceCtx

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
	formatter := NewLoopFormatter(cfg.Stderr)
	stats := runStats{start: time.Now()}

	// Rate limit backoff state
	const initialBackoff = 30 * time.Second
	const maxBackoff = 10 * time.Minute
	backoff := initialBackoff

	for i := 1; max == 0 || i <= max; i++ {
		// Check shutdown flag before starting each new iteration
		select {
		case <-cfg.Shutdown:
			printRunSummary(cfg.Stderr, stats)
			return ErrInterrupted
		default:
		}

		formatter.IterationHeader(i, max, "")
		start := time.Now()

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
			select {
			case <-time.After(wait):
			case <-cfg.Shutdown:
				printRunSummary(cfg.Stderr, stats)
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

		// Success: reset backoff, accumulate stats, and print status
		backoff = initialBackoff
		stats.iterations++
		stats.inputTokens += result.InputTokens
		stats.outputTokens += result.OutputTokens
		stats.cacheTokens += result.CacheTokens
		formatter.IterationStatus(time.Since(start), result.InputTokens, result.OutputTokens, result.CacheTokens)

		// Wait between iterations (skip after last), interruptible by shutdown
		if (max == 0 || i < max) && (cfg.Delay > 0 || cfg.Fuzz > 0) {
			d := computeDelay(cfg.Delay, cfg.Fuzz)
			if d > 0 {
				fmt.Fprintf(cfg.Stderr, "waiting %v before next iteration\n", d)
				select {
				case <-time.After(d):
				case <-cfg.Shutdown:
					printRunSummary(cfg.Stderr, stats)
					return ErrInterrupted
				}
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
