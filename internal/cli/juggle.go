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

// errCostGuard is returned by runWatchTask when --max-cost threshold is exceeded.
var errCostGuard = errors.New("cost guard triggered")

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
	MaxFailures  int           // Stop after N consecutive non-zero exits (0 = disabled)
	CmdBefore    string        // Shell command to run before each iteration
	CmdAfter     string        // Shell command to run after each iteration
	StopWhen     string        // Shell command; exit 0 stops the loop gracefully
	AgentPre     string        // Agent session prompt to run once before the loop
	AgentBefore  string        // Agent session prompt to run before each iteration
	AgentAfter   string        // Agent session prompt to run after each iteration
	AgentPost         string        // Agent session prompt to run once after the loop
	HooksSettingsFile string        // path to temp hooks settings JSON file (Claude-specific)
	Log               string        // Path to log file (summary appended on completion)
	MaxCost           float64       // Stop loop when cumulative cost exceeds this (0 = disabled)
	Label             string        // Optional run label (exposed as JUGGLE_LABEL)
	RunID             string        // Stable UUID for the entire run (generated in Run() if empty)
	AllowedTools      []string      // Restrict agent to these tools only (mutually exclusive with DisallowedTools)
	DisallowedTools   []string      // Block specific tools (mutually exclusive with AllowedTools)

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
	model        string
}

// modelPricing holds per-token cost in USD per million tokens.
type modelPricing struct {
	inputPerMTok  float64
	outputPerMTok float64
}

// defaultPricing maps canonical model names to pricing (USD per million tokens).
var defaultPricing = map[string]modelPricing{
	"opus":   {inputPerMTok: 15.0, outputPerMTok: 75.0},
	"sonnet": {inputPerMTok: 3.0, outputPerMTok: 15.0},
	"haiku":  {inputPerMTok: 0.80, outputPerMTok: 4.0},
}

// estimateCost returns the estimated USD cost for the given token counts and model.
func estimateCost(inputTokens, outputTokens int, model string) float64 {
	p, ok := defaultPricing[model]
	if !ok {
		p = defaultPricing["sonnet"] // sensible default
	}
	return (float64(inputTokens)*p.inputPerMTok + float64(outputTokens)*p.outputPerMTok) / 1_000_000
}

// printRunSummary writes a summary line to w.
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

	cost := estimateCost(stats.inputTokens, stats.outputTokens, stats.model)
	fmt.Fprintf(w, "Run summary: %d iteration(s), %s, ~$%.4f, %s\n", stats.iterations, tok, cost, timing)
}

// writeSummary prints the run summary to cfg.Stderr and, if cfg.Log is set, appends it to the log file.
func writeSummary(cfg Config, stats runStats) {
	printRunSummary(cfg.Stderr, stats)
	if cfg.Log != "" {
		if f, err := os.OpenFile(cfg.Log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			printRunSummary(f, stats)
			f.Close()
		}
	}
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
	maxFailures  int
	cmdBefore    string
	cmdAfter     string
	stopWhen     string
	agentPre     []string
	agentBefore  []string
	agentAfter   []string
	agentPost    []string
	hooks           []string
	hooksFile       string
	log             string
	maxCost         float64
	label           string
	allowedTools    []string
	disallowedTools []string
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
	f.IntVar(&flags.maxFailures, "max-failures", 3, "stop after N consecutive non-zero exits (0 = disabled)")
	f.StringVar(&flags.cmdBefore, "cmd-before", "", "shell command to run before each iteration")
	f.StringVar(&flags.cmdAfter, "cmd-after", "", "shell command to run after each iteration")
	f.StringVar(&flags.stopWhen, "stop-when", "", "shell command; exit 0 stops the loop gracefully")
	f.StringSliceVar(&flags.agentPre, "agent-pre", nil, "agent session prompt(s) to run once before the loop (comma-separated)")
	f.StringSliceVar(&flags.agentBefore, "agent-before", nil, "agent session prompt(s) to run before each iteration (comma-separated)")
	f.StringSliceVar(&flags.agentAfter, "agent-after", nil, "agent session prompt(s) to run after each iteration (comma-separated)")
	f.StringSliceVar(&flags.agentPost, "agent-post", nil, "agent session prompt(s) to run once after the loop (comma-separated)")
	f.StringArrayVar(&flags.hooks, "hook", nil, "agent-internal hook: EVENT:CMD (repeatable; @file resolves via JUGGLE_PROMPTS)")
	f.StringVar(&flags.hooksFile, "hooks-file", "", "path to Claude Code hooks settings JSON file")
	f.StringVar(&flags.log, "log", "", "append run summary to this file on completion")
	f.Float64Var(&flags.maxCost, "max-cost", 0, "stop loop when cumulative cost estimate exceeds this amount in USD (0 = disabled)")
	f.StringVar(&flags.label, "label", "", "optional label for the run (exposed as JUGGLE_LABEL)")
	f.StringSliceVar(&flags.allowedTools, "allowed-tools", nil, "restrict agent to these tools only (comma-separated; mutually exclusive with --disallowed-tools)")
	f.StringSliceVar(&flags.disallowedTools, "disallowed-tools", nil, "block specific tools (comma-separated; mutually exclusive with --allowed-tools)")

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

	agentPre, err := BuildPhaseContent(flags.agentPre)
	if err != nil {
		return fmt.Errorf("--agent-pre: %w", err)
	}
	agentBefore, err := BuildPhaseContent(flags.agentBefore)
	if err != nil {
		return fmt.Errorf("--agent-before: %w", err)
	}
	agentAfter, err := BuildPhaseContent(flags.agentAfter)
	if err != nil {
		return fmt.Errorf("--agent-after: %w", err)
	}
	agentPost, err := BuildPhaseContent(flags.agentPost)
	if err != nil {
		return fmt.Errorf("--agent-post: %w", err)
	}

	hooksSettingsFile, hooksCleanup, err := buildHooksSettingsFile(flags.hooks, flags.hooksFile)
	if err != nil {
		return fmt.Errorf("hooks: %w", err)
	}
	defer hooksCleanup()

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
		MaxFailures:  flags.maxFailures,
		CmdBefore:    flags.cmdBefore,
		CmdAfter:     flags.cmdAfter,
		StopWhen:     flags.stopWhen,
		AgentPre:     agentPre,
		AgentBefore:  agentBefore,
		AgentAfter:   agentAfter,
		AgentPost:         agentPost,
		HooksSettingsFile: hooksSettingsFile,
		Log:               flags.log,
		MaxCost:           flags.maxCost,
		Label:             flags.label,
		AllowedTools:      flags.allowedTools,
		DisallowedTools:   flags.disallowedTools,
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
	if cfg.RunID == "" {
		cfg.RunID = generateRunID()
	}

	if len(cfg.AllowedTools) > 0 && len(cfg.DisallowedTools) > 0 {
		return fmt.Errorf("--allowed-tools and --disallowed-tools are mutually exclusive")
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
	if cfg.RunID == "" {
		cfg.RunID = generateRunID()
	}

	max := cfg.Iterations
	formatter := NewLoopFormatter(cfg.Stderr)
	stats := runStats{start: time.Now(), model: cfg.Model}
	consecutiveFailures := 0

	// Rate limit backoff state
	const initialBackoff = 30 * time.Second
	const maxBackoff = 10 * time.Minute
	backoff := initialBackoff

	// Run agent-pre once before the loop (failure stops the run)
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
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		formatter.IterationHeader(i, max, "")

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

		prompt := BuildPrompt(cfg.Content, i, max)
		opts := buildRunOptions(cfg, prompt)
		opts.Env = append(opts.Env, buildJuggleEnv(cfg.RunID, i, max, cfg.Label, cfg.Model, cfg.Provider, "", -1)...)

		result, err := cfg.Runner.Run(opts)
		if err != nil {
			return fmt.Errorf("runner error on iteration %d: %w", i, err)
		}

		// Handle overload exhausted
		if result.OverloadExhausted {
			return fmt.Errorf("agent exhausted overload retries on iteration %d", i)
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
				writeSummary(cfg, stats)
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
				writeSummary(cfg, stats)
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
		stats.iterations++
		stats.inputTokens += result.InputTokens
		stats.outputTokens += result.OutputTokens
		stats.cacheTokens += result.CacheTokens
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
				writeSummary(cfg, stats)
				return nil
			}
		}

		// Check cost guard after each iteration, before delay sleep
		if cfg.MaxCost > 0 {
			cost := estimateCost(stats.inputTokens, stats.outputTokens, stats.model)
			if cost > cfg.MaxCost {
				fmt.Fprintf(cfg.Stderr, "cost guard triggered: estimated $%.4f exceeds --max-cost $%.4f\n", cost, cfg.MaxCost)
				writeSummary(cfg, stats)
				return nil
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
					writeSummary(cfg, stats)
					return ErrInterrupted
				}
			}
		}
	}

	// Run agent-post once after the loop (failure stops the run)
	if cfg.AgentPost != "" {
		env := phaseEnv{phase: "post", iteration: 0, maxIter: max}
		if err := runPhaseAgent(cfg, cfg.AgentPost, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-post failed: %w", err)
		}
	}

	writeSummary(cfg, stats)
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

// formatWaitDuration formats a duration as "Xh Ym Zs" for human-readable log messages.
func formatWaitDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func maxStr(max int) string {
	if max == 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", max)
}
