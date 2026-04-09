package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/agent/provider"
	"github.com/spf13/cobra"
)

// SetVersion sets the version string (injected at build time).
func SetVersion(v string) {
	rootCmd.Version = v
}

// ErrInterrupted is returned when the run is stopped by a signal.
var ErrInterrupted = errors.New("interrupted by signal")

// OnFailure controls what happens when an agent iteration exits non-zero.
type OnFailure string

const (
	// OnFailureStop halts the loop on the first non-zero exit (default).
	OnFailureStop OnFailure = "stop"
	// OnFailureContinue logs the failure and skips to the next iteration.
	OnFailureContinue OnFailure = "continue"
	// OnFailureRetry retries the same iteration up to --retries times with backoff.
	OnFailureRetry OnFailure = "retry"
)

// defaultRetryBackoffs are the backoff durations for --on-failure retry mode.
var defaultRetryBackoffs = []time.Duration{10 * time.Second, 30 * time.Second}

// retryBackoffFor returns the backoff duration for the given retry attempt (0-indexed).
// overrides replaces the default backoffs when non-nil.
func retryBackoffFor(attempt int, overrides []time.Duration) time.Duration {
	backoffs := defaultRetryBackoffs
	if len(overrides) > 0 {
		backoffs = overrides
	}
	if attempt < len(backoffs) {
		return backoffs[attempt]
	}
	return backoffs[len(backoffs)-1]
}

// errCostGuard is returned by runWatchTask when --max-cost threshold is exceeded.
var errCostGuard = errors.New("cost guard triggered")

// Config holds all CLI configuration for a juggle run.
type Config struct {
	Content           string                  // Resolved prompt content (joined)
	Watch             []string                // Watch directory paths (repeatable --watch)
	Iterations        int                     // Max iterations (0 = unlimited)
	Model             string                  // Model name
	Provider          string                  // Provider name
	Delay             int                     // Minutes between iterations
	Fuzz              int                     // +/- random variance in minutes
	Trust             bool                    // Bypass permission checks
	Plan              bool                    // Read-only plan mode
	Interactive       bool                    // Interactive TUI mode
	Timeout           time.Duration           // Per-iteration timeout
	MaxWait           time.Duration           // Max rate limit wait
	DryRun            bool                    // Show prompt, don't run
	ShowThinking      bool                    // Show thinking blocks
	Verbose           bool                    // Show tool inputs in headless output
	MaxFailures       int                     // Stop after N consecutive non-zero exits (0 = disabled)
	CmdBefore         string                  // Shell command to run before each iteration
	CmdAfter          string                  // Shell command to run after each iteration
	StopWhen          string                  // Shell command; exit 0 stops the loop gracefully
	AgentPre          string                  // Agent session prompt to run once before the loop
	AgentBefore       string                  // Agent session prompt to run before each iteration
	AgentAfter        string                  // Agent session prompt to run after each iteration
	AgentPost         string                  // Agent session prompt to run once after the loop
	Hooks             []string                // raw --hook flag values (for dry-run display)
	HooksSettingsFile string                  // path to temp hooks settings JSON file (Claude-specific)
	Log               string                  // Path to log file (summary appended on completion)
	MaxCost           float64                 // Stop loop when cumulative cost exceeds this (0 = disabled)
	Label             string                  // Optional run label (exposed as JUGGLE_LABEL)
	RunID             string                  // Stable UUID for the entire run (generated in Run() if empty)
	AllowedTools      []string                // Restrict agent to these tools only (mutually exclusive with DisallowedTools)
	DisallowedTools   []string                // Block specific tools (mutually exclusive with AllowedTools)
	MaxTurns          int                     // Cap tool-use turns per iteration (0 = provider default)
	MCPConfig         string                  // Path to MCP server config file
	OnFailure         OnFailure               // What to do on non-zero exit: stop, continue, retry (default: stop)
	Retries           int                     // Max retries for OnFailureRetry mode (0 = default 2)
	RetryBackoffs     []time.Duration         // Override retry backoffs for testing (nil = use defaults)
	PassthroughArgs   []string                // Extra flags passed verbatim to the agent CLI after juggle's own flags
	AgentCmd          string                  // Command template for --provider custom (e.g. "my-agent --prompt {prompt}")
	SystemPrompt      string                  // Optional system prompt appended to the agent's system prompt
	Workers           int                     // Number of concurrent watch workers (0 or 1 = serial, >=2 = parallel)
	WorkDir           string                  // Working directory for agent spawning (empty = juggle's cwd)
	Resume            bool                    // Resume from last completed iteration recorded in --log file
	Dashboard         bool                    // Show TUI dashboard for watch workers (auto-enabled for glob watch)
	OnIterDone        func(iter, maxIter int) // called after each successful iteration (dashboard hook; nil = disabled)

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
	runID        string
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

// writeSummary prints the run summary to cfg.Stderr and, if cfg.Log is set, appends a JSON summary entry.
func writeSummary(cfg Config, stats runStats) {
	printRunSummary(cfg.Stderr, stats)
	writeSummaryLog(cfg.Log, stats)
}

// flags is used for cobra flag binding for the root command and its persistent flags.
var flags struct {
	iterations      int
	model           string
	provider        string
	delay           int
	fuzz            int
	trust           bool
	plan            bool
	interactive     bool
	timeout         time.Duration
	maxWait         time.Duration
	dryRun          bool
	showThinking    bool
	verbose         bool
	maxFailures     int
	cmdBefore       string
	cmdAfter        string
	stopWhen        string
	agentPre        []string
	agentBefore     []string
	agentAfter      []string
	agentPost       []string
	hooks           []string
	hooksFile       string
	log             string
	maxCost         float64
	label           string
	allowedTools    []string
	disallowedTools []string
	maxTurns        int
	mcpConfig       string
	onFailure       string
	retries         int
	agentCmd        string
	systemPrompt    string
	workdir         string
	noConfig        bool
	resume          bool
}

// watchFlags holds flags specific to the watch subcommand.
var watchFlags struct {
	dirs      []string
	workers   int
	dashboard bool
}

func init() {
	// pf holds shared flags inherited by all subcommands.
	pf := rootCmd.PersistentFlags()
	pf.IntVarP(&flags.iterations, "iterations", "n", 10, "max iterations (0 = unlimited)")
	pf.StringVar(&flags.model, "model", "sonnet", "model name")
	pf.StringVar(&flags.provider, "provider", "claude", "provider name")
	pf.IntVar(&flags.delay, "delay", 0, "minutes between iterations")
	pf.IntVar(&flags.fuzz, "fuzz", 0, "+/- random variance in minutes")
	pf.BoolVar(&flags.trust, "trust", false, "bypass permission checks")
	pf.BoolVar(&flags.plan, "plan", false, "read-only plan mode (shortcut for --permission-mode plan)")
	pf.BoolVar(&flags.interactive, "interactive", false, "interactive TUI mode")
	pf.DurationVar(&flags.timeout, "timeout", 0, "per-iteration timeout")
	pf.DurationVar(&flags.maxWait, "max-wait", 0, "max rate limit wait")
	pf.BoolVar(&flags.dryRun, "dry-run", false, "show prompt, don't run")
	pf.BoolVar(&flags.showThinking, "show-thinking", false, "show thinking blocks")
	pf.BoolVarP(&flags.verbose, "verbose", "v", false, "show tool inputs in output")
	pf.IntVar(&flags.maxFailures, "max-failures", 3, "stop after N consecutive non-zero exits (0 = disabled)")
	pf.StringVar(&flags.cmdBefore, "cmd-before", "", "shell command to run before each iteration")
	pf.StringVar(&flags.cmdAfter, "cmd-after", "", "shell command to run after each iteration")
	pf.StringVar(&flags.stopWhen, "stop-when", "", "shell command; exit 0 stops the loop gracefully")
	pf.StringSliceVar(&flags.agentPre, "agent-pre", nil, "agent session prompt(s) to run once before the loop (comma-separated)")
	pf.StringSliceVar(&flags.agentBefore, "agent-before", nil, "agent session prompt(s) to run before each iteration (comma-separated)")
	pf.StringSliceVar(&flags.agentAfter, "agent-after", nil, "agent session prompt(s) to run after each iteration (comma-separated)")
	pf.StringSliceVar(&flags.agentPost, "agent-post", nil, "agent session prompt(s) to run once after the loop (comma-separated)")
	pf.StringArrayVar(&flags.hooks, "hook", nil, "agent-internal hook: EVENT:CMD (repeatable; @file resolves via JUGGLE_PROMPTS)")
	pf.StringVar(&flags.hooksFile, "hooks-file", "", "path to Claude Code hooks settings JSON file")
	pf.StringVar(&flags.log, "log", "", "append one JSON line per iteration to this file (JSONL); summary appended on completion")
	pf.Float64Var(&flags.maxCost, "max-cost", 0, "stop loop when cumulative cost estimate exceeds this amount in USD (0 = disabled)")
	pf.StringVar(&flags.label, "label", "", "optional label for the run (exposed as JUGGLE_LABEL)")
	pf.StringSliceVar(&flags.allowedTools, "allowed-tools", nil, "restrict agent to these tools only (comma-separated; mutually exclusive with --disallowed-tools)")
	pf.StringSliceVar(&flags.disallowedTools, "disallowed-tools", nil, "block specific tools (comma-separated; mutually exclusive with --allowed-tools)")
	pf.IntVar(&flags.maxTurns, "max-turns", 0, "cap tool-use turns per iteration (0 = provider default)")
	pf.StringVar(&flags.mcpConfig, "mcp-config", "", "path to MCP server config file")
	pf.StringVar(&flags.onFailure, "on-failure", "stop", "behavior on non-zero exit: stop, continue, or retry")
	pf.IntVar(&flags.retries, "retries", 2, "max retries per iteration for --on-failure retry (default 2)")
	pf.StringVar(&flags.agentCmd, "agent-cmd", "", "command template for custom provider (e.g. \"my-agent --prompt {prompt}\"); sets --provider custom automatically")
	pf.StringVar(&flags.systemPrompt, "system-prompt", "", "append text to the agent's system prompt (@file resolves via JUGGLE_PROMPTS)")
	pf.StringVar(&flags.workdir, "workdir", "", "working directory for agent execution (default: juggle's cwd)")
	pf.BoolVar(&flags.noConfig, "no-config", false, "skip config file loading")
	pf.BoolVar(&flags.resume, "resume", false, "resume from last completed iteration in the --log file (requires --log)")

	// Hide less-common flags to reduce noise in default help output
	_ = pf.MarkHidden("fuzz")
	_ = pf.MarkHidden("interactive")
	_ = pf.MarkHidden("show-thinking")
	_ = pf.MarkHidden("provider")

	// Assign visible flags to help groups. Hidden flags (fuzz, interactive,
	// show-thinking, provider) are intentionally omitted here.
	for _, name := range []string{"iterations", "delay", "timeout", "max-wait", "max-failures", "stop-when", "max-cost", "on-failure", "retries", "resume"} {
		setFlagGroup(pf, name, "Loop Control")
	}
	for _, name := range []string{"model", "trust", "plan", "system-prompt", "allowed-tools", "disallowed-tools", "max-turns", "mcp-config", "agent-cmd", "workdir"} {
		setFlagGroup(pf, name, "Agent Configuration")
	}
	for _, name := range []string{"cmd-before", "cmd-after", "agent-pre", "agent-before", "agent-after", "agent-post", "hook", "hooks-file"} {
		setFlagGroup(pf, name, "Lifecycle Hooks")
	}
	for _, name := range []string{"dry-run", "verbose", "log", "label"} {
		setFlagGroup(pf, name, "Output")
	}

	// Watch subcommand flags
	wf := watchCmd.Flags()
	wf.StringArrayVar(&watchFlags.dirs, "dir", nil, "additional watch directory (repeatable)")
	wf.IntVar(&watchFlags.workers, "workers", 1, "number of concurrent watch workers")
	wf.BoolVar(&watchFlags.dashboard, "dashboard", false, "show TUI dashboard for watch workers (default on for glob watch)")
	for _, name := range []string{"dir", "workers", "dashboard"} {
		setFlagGroup(wf, name, "Watch Mode")
	}

	rootCmd.SetHelpFunc(groupedHelp)
	watchCmd.SetHelpFunc(groupedHelp)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(watchCmd)
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|nushell|powershell]",
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
		case "nushell":
			genNushellCompletion(rootCmd, os.Stdout)
			return nil
		case "powershell":
			genPowerShellCompletion(rootCmd, os.Stdout)
			return nil
		default:
			return fmt.Errorf("unsupported shell: %s (use bash, zsh, fish, nushell, or powershell)", args[0])
		}
	},
}

var watchCmd = &cobra.Command{
	Use:   "watch <dir> [prompt-content...]",
	Short: "Watch a directory for task files and run the agent on each",
	Long: `Watch a directory for task files. The first positional argument is the watch
directory; remaining positional arguments are prompt content (@file references supported).

Additional watch directories can be added with --dir (repeatable).`,
	Example: `  # Watch a directory, run 5 iterations per task
  juggle watch ./tasks/ @rules.md -n 5

  # Multiple watch directories
  juggle watch ./tasks/ @rules.md --dir ./more-tasks/

  # Watch with multiple workers and dashboard
  juggle watch ./tasks/ --workers 4 --dashboard`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWatchSubcmd(cmd, args)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

var rootCmd = &cobra.Command{
	Use:     "juggle [prompt-content...]",
	Short:   "Minimal agent loop runner",
	Version: "dev",
	Long:    `Run an AI agent in a loop. All positional args are prompt content (strings or @file references).`,
	Example: `  # Basic: run 3 iterations
  juggle "fix the failing tests" -n 3

  # With a prompt file and trust mode
  juggle @task.md --trust -n 10

  # Watch mode: pick up tasks from a directory
  juggle watch ./tasks/ @rules.md

  # With hooks: run tests after each iteration, stop when they pass
  juggle --cmd-after "npm test" --stop-when "npm test" @task.md

  # Multi-phase: run a tidy agent after each iteration
  juggle --agent-after @tidy @task.md -n 5

  # Pass provider-specific flags after -- (appended to agent CLI as-is)
  juggle @task.md -- --max-turns 50 --allowedTools Bash,Read`,
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: completeArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, args)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// splitPassthroughArgs splits positional args at the '--' separator.
// dashLen is the value returned by cmd.Flags().ArgsLenAtDash() (-1 if no '--').
// Returns (normalArgs, passthroughArgs); passthroughArgs is nil when dashLen < 0 or nothing follows.
func splitPassthroughArgs(args []string, dashLen int) ([]string, []string) {
	if dashLen < 0 {
		return args, nil
	}
	normal := args[:dashLen]
	if len(normal) == 0 {
		normal = nil
	}
	passthru := args[dashLen:]
	if len(passthru) == 0 {
		passthru = nil
	}
	return normal, passthru
}

// run is the cobra RunE handler.
func run(cmd *cobra.Command, args []string) error {
	normalArgs, passthroughArgs := splitPassthroughArgs(args, cmd.Flags().ArgsLenAtDash())

	// Load and apply config file defaults before using any flag values.
	fileCfg, _, cfgErr := LoadConfig(flags.noConfig, ".", os.Stderr)
	if cfgErr != nil {
		return cfgErr
	}
	if fileCfg != nil {
		// Apply verbose first: config may set verbose=true, which affects the output below.
		if fileCfg.Verbose != nil && !cmd.Flags().Changed("verbose") {
			flags.verbose = *fileCfg.Verbose
		}
		ApplyFileConfig(fileCfg, cmd.Flags().Changed, flags.verbose, os.Stderr)
	}

	resolved, err := ResolveArgs(normalArgs)
	if err != nil {
		return err
	}

	// Read piped stdin when not in interactive mode and stdin is not a TTY.
	if !flags.interactive {
		if info, statErr := os.Stdin.Stat(); statErr == nil {
			isTTY := info.Mode()&os.ModeCharDevice != 0
			stdinContent, readErr := ReadStdin(os.Stdin, isTTY)
			if readErr != nil {
				return fmt.Errorf("reading stdin: %w", readErr)
			}
			if stdinContent != "" {
				resolved = append(resolved, stdinContent)
			}
		}
	}

	// Show help when there is no content.
	if len(resolved) == 0 {
		return cmd.Help()
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

	var systemPrompt string
	if flags.systemPrompt != "" {
		resolved1, err1 := ResolveArgs([]string{flags.systemPrompt})
		if err1 != nil {
			return fmt.Errorf("--system-prompt: %w", err1)
		}
		systemPrompt = resolved1[0]
	}

	if flags.mcpConfig != "" {
		if _, err := os.Stat(flags.mcpConfig); err != nil {
			return fmt.Errorf("--mcp-config: file not found: %s", flags.mcpConfig)
		}
	}

	hooksSettingsFile, hooksCleanup, err := buildHooksSettingsFile(flags.hooks, flags.hooksFile)
	if err != nil {
		return fmt.Errorf("hooks: %w", err)
	}
	defer hooksCleanup()

	cfg := Config{
		Content:           strings.Join(resolved, "\n\n"),
		Iterations:        flags.iterations,
		Model:             flags.model,
		Provider:          flags.provider,
		Delay:             flags.delay,
		Fuzz:              flags.fuzz,
		Trust:             flags.trust,
		Plan:              flags.plan,
		Interactive:       flags.interactive,
		Timeout:           flags.timeout,
		MaxWait:           flags.maxWait,
		DryRun:            flags.dryRun,
		ShowThinking:      flags.showThinking,
		Verbose:           flags.verbose,
		MaxFailures:       flags.maxFailures,
		CmdBefore:         flags.cmdBefore,
		CmdAfter:          flags.cmdAfter,
		StopWhen:          flags.stopWhen,
		AgentPre:          agentPre,
		AgentBefore:       agentBefore,
		AgentAfter:        agentAfter,
		AgentPost:         agentPost,
		Hooks:             flags.hooks,
		HooksSettingsFile: hooksSettingsFile,
		Log:               flags.log,
		MaxCost:           flags.maxCost,
		Label:             flags.label,
		AllowedTools:      flags.allowedTools,
		DisallowedTools:   flags.disallowedTools,
		MaxTurns:          flags.maxTurns,
		MCPConfig:         flags.mcpConfig,
		OnFailure:         OnFailure(flags.onFailure),
		Retries:           flags.retries,
		PassthroughArgs:   passthroughArgs,
		AgentCmd:          flags.agentCmd,
		SystemPrompt:      systemPrompt,
		WorkDir:           flags.workdir,
		Resume:            flags.resume,
	}

	// --agent-cmd auto-sets --provider custom
	if cfg.AgentCmd != "" && cfg.Provider == "claude" {
		cfg.Provider = "custom"
	}

	// Build runner from provider flag
	var p provider.Provider
	if provider.Type(cfg.Provider) == provider.TypeCustom {
		if cfg.AgentCmd == "" {
			return fmt.Errorf("--provider custom requires --agent-cmd")
		}
		p = provider.GetCustom(cfg.AgentCmd)
	} else {
		p = provider.Get(provider.Type(cfg.Provider))
	}
	cfg.Runner = &agent.ProviderRunner{Provider: p}

	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	if cfg.DryRun {
		return Run(cfg)
	}

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
		writeStopRequestedMessage(stderr, isColorEnabled(stderr))
		shutdownOnce.Do(func() { close(shutdown) })
		<-sigCh
		// Second signal: cancel child context then exit
		forceCancel()
		time.Sleep(200 * time.Millisecond)
		os.Exit(130)
	}()

	cfg.Shutdown = shutdown
	cfg.ForceCtx = forceCtx

	// Set up keypress listener: pressing 'q' acts like the first Ctrl+C.
	// Only active when stdin is a TTY; silently skipped otherwise.
	if tty, ttyCleanup, err := openTTYKeypress(); err == nil {
		color := isColorEnabled(stderr)
		trigger := func() { shutdownOnce.Do(func() { close(shutdown) }) }
		waitKeypress := StartKeypressListener(tty, trigger, color, stderr)
		defer func() {
			ttyCleanup()   // restore terminal + close tty (unblocks goroutine Read)
			waitKeypress() // wait for goroutine to exit
		}()
	}

	return Run(cfg)
}

// runWatchSubcmd is the cobra RunE handler for the watch subcommand.
// args[0] is the watch directory; args[1:] are prompt content.
func runWatchSubcmd(cmd *cobra.Command, args []string) error {
	normalArgs, passthroughArgs := splitPassthroughArgs(args, cmd.Flags().ArgsLenAtDash())

	watchDir := normalArgs[0]
	promptArgs := normalArgs[1:]

	watchDirs := append([]string{watchDir}, watchFlags.dirs...)

	// Load and apply config file defaults before using any flag values.
	fileCfg, _, cfgErr := LoadConfig(flags.noConfig, ".", os.Stderr)
	if cfgErr != nil {
		return cfgErr
	}
	if fileCfg != nil {
		if fileCfg.Verbose != nil && !cmd.Flags().Changed("verbose") {
			flags.verbose = *fileCfg.Verbose
		}
		ApplyFileConfig(fileCfg, cmd.Flags().Changed, flags.verbose, os.Stderr)
	}

	resolved, err := ResolveArgs(promptArgs)
	if err != nil {
		return err
	}

	// Detect shell glob expansion on the watch directory argument.
	if err := detectShellGlobExpansion(watchDirs, promptArgs); err != nil {
		return err
	}

	// Read piped stdin as additional prompt content when not in interactive mode.
	if !flags.interactive {
		if info, statErr := os.Stdin.Stat(); statErr == nil {
			isTTY := info.Mode()&os.ModeCharDevice != 0
			stdinContent, readErr := ReadStdin(os.Stdin, isTTY)
			if readErr != nil {
				return fmt.Errorf("reading stdin: %w", readErr)
			}
			if stdinContent != "" {
				resolved = append(resolved, stdinContent)
			}
		}
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

	var systemPrompt string
	if flags.systemPrompt != "" {
		resolved1, err1 := ResolveArgs([]string{flags.systemPrompt})
		if err1 != nil {
			return fmt.Errorf("--system-prompt: %w", err1)
		}
		systemPrompt = resolved1[0]
	}

	if flags.mcpConfig != "" {
		if _, err := os.Stat(flags.mcpConfig); err != nil {
			return fmt.Errorf("--mcp-config: file not found: %s", flags.mcpConfig)
		}
	}

	hooksSettingsFile, hooksCleanup, err := buildHooksSettingsFile(flags.hooks, flags.hooksFile)
	if err != nil {
		return fmt.Errorf("hooks: %w", err)
	}
	defer hooksCleanup()

	cfg := Config{
		Content:           strings.Join(resolved, "\n\n"),
		Watch:             watchDirs,
		Workers:           watchFlags.workers,
		Dashboard:         watchFlags.dashboard,
		Iterations:        flags.iterations,
		Model:             flags.model,
		Provider:          flags.provider,
		Delay:             flags.delay,
		Fuzz:              flags.fuzz,
		Trust:             flags.trust,
		Plan:              flags.plan,
		Interactive:       flags.interactive,
		Timeout:           flags.timeout,
		MaxWait:           flags.maxWait,
		DryRun:            flags.dryRun,
		ShowThinking:      flags.showThinking,
		Verbose:           flags.verbose,
		MaxFailures:       flags.maxFailures,
		CmdBefore:         flags.cmdBefore,
		CmdAfter:          flags.cmdAfter,
		StopWhen:          flags.stopWhen,
		AgentPre:          agentPre,
		AgentBefore:       agentBefore,
		AgentAfter:        agentAfter,
		AgentPost:         agentPost,
		Hooks:             flags.hooks,
		HooksSettingsFile: hooksSettingsFile,
		Log:               flags.log,
		MaxCost:           flags.maxCost,
		Label:             flags.label,
		AllowedTools:      flags.allowedTools,
		DisallowedTools:   flags.disallowedTools,
		MaxTurns:          flags.maxTurns,
		MCPConfig:         flags.mcpConfig,
		OnFailure:         OnFailure(flags.onFailure),
		Retries:           flags.retries,
		PassthroughArgs:   passthroughArgs,
		AgentCmd:          flags.agentCmd,
		SystemPrompt:      systemPrompt,
		WorkDir:           flags.workdir,
		Resume:            flags.resume,
	}

	// --agent-cmd auto-sets --provider custom
	if cfg.AgentCmd != "" && cfg.Provider == "claude" {
		cfg.Provider = "custom"
	}

	// Build runner from provider flag
	var p provider.Provider
	if provider.Type(cfg.Provider) == provider.TypeCustom {
		if cfg.AgentCmd == "" {
			return fmt.Errorf("--provider custom requires --agent-cmd")
		}
		p = provider.GetCustom(cfg.AgentCmd)
	} else {
		p = provider.Get(provider.Type(cfg.Provider))
	}
	cfg.Runner = &agent.ProviderRunner{Provider: p}

	stderr := cfg.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	if cfg.DryRun {
		return Run(cfg)
	}

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
		writeStopRequestedMessage(stderr, isColorEnabled(stderr))
		shutdownOnce.Do(func() { close(shutdown) })
		<-sigCh
		forceCancel()
		time.Sleep(200 * time.Millisecond)
		os.Exit(130)
	}()

	cfg.Shutdown = shutdown
	cfg.ForceCtx = forceCtx

	if tty, ttyCleanup, err := openTTYKeypress(); err == nil {
		color := isColorEnabled(stderr)
		trigger := func() { shutdownOnce.Do(func() { close(shutdown) }) }
		waitKeypress := StartKeypressListener(tty, trigger, color, stderr)
		defer func() {
			ttyCleanup()
			waitKeypress()
		}()
	}

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

	if cfg.Plan && cfg.Trust {
		return fmt.Errorf("--plan and --trust are mutually exclusive")
	}

	if len(cfg.AllowedTools) > 0 && len(cfg.DisallowedTools) > 0 {
		return fmt.Errorf("--allowed-tools and --disallowed-tools are mutually exclusive")
	}

	if cfg.Retries < 0 {
		return fmt.Errorf("--retries must be non-negative")
	}

	if cfg.Workers > 1 && len(cfg.Watch) == 0 {
		return fmt.Errorf("--workers requires --watch")
	}

	if cfg.WorkDir != "" {
		if _, err := os.Stat(cfg.WorkDir); err != nil {
			return fmt.Errorf("--workdir: %w", err)
		}
	}

	// Resolve relative watch paths against workdir
	if cfg.WorkDir != "" {
		for i, w := range cfg.Watch {
			if !filepath.IsAbs(w) {
				cfg.Watch[i] = filepath.Join(cfg.WorkDir, w)
			}
		}
	}

	switch cfg.OnFailure {
	case OnFailureStop, OnFailureContinue, OnFailureRetry, "":
		// valid (empty treated as stop)
	default:
		return fmt.Errorf("invalid --on-failure value: %q (use stop, continue, or retry)", cfg.OnFailure)
	}

	if cfg.DryRun {
		printDryRun(cfg, cfg.Stdout)
		return nil
	}

	if len(cfg.Watch) > 0 {
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

	if cfg.Resume && cfg.Log == "" {
		return fmt.Errorf("--resume requires --log to be set")
	}

	if cfg.Label == "" {
		cfg.Label = autoLabel(cfg.Content)
	}

	max := cfg.Iterations
	formatter := NewLoopFormatter(cfg.Stderr)
	stats := runStats{runID: cfg.RunID, start: time.Now(), model: cfg.Model}
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

	// Determine start iteration (1, or N+1 when resuming)
	startFrom := 1
	if cfg.Resume {
		last, err := parseLastIteration(cfg.Log)
		if err != nil {
			return fmt.Errorf("--resume: reading log: %w", err)
		}
		startFrom = last + 1
		if last > 0 {
			fmt.Fprintf(cfg.Stderr, "resuming from iteration %d\n", startFrom)
		}
	}

	// Run agent-pre once before the loop (failure stops the run)
	if cfg.AgentPre != "" {
		formatter.PhaseAgentHeader("pre")
		if cfg.Verbose {
			fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentPre)
		}
		env := phaseEnv{phase: "pre", iteration: 0, maxIter: max, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
		if err := runPhaseAgent(cfg, cfg.AgentPre, env, cfg.Stderr); err != nil {
			return fmt.Errorf("agent-pre failed: %w", err)
		}
	}

	for i := startFrom; max == 0 || i <= max; i++ {
		// Check shutdown flag before starting each new iteration
		select {
		case <-cfg.Shutdown:
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		formatter.IterationHeader(i, max, "", cfg.Label)

		// Run agent-before; skip iteration on failure
		if cfg.AgentBefore != "" {
			formatter.PhaseAgentHeader("before")
			if cfg.Verbose {
				fmt.Fprintf(cfg.Stderr, "  prompt: %s\n", cfg.AgentBefore)
			}
			env := phaseEnv{phase: "before", iteration: i, maxIter: max, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
			if err := runPhaseAgent(cfg, cfg.AgentBefore, env, cfg.Stderr); err != nil {
				if errors.Is(err, ErrInterrupted) {
					writeSummary(cfg, stats)
					return ErrInterrupted
				}
				fmt.Fprintf(cfg.Stderr, "agent-before failed (skipping iteration %d): %v\n", i, err)
				continue
			}
		}

		// Run cmd-before; skip iteration on failure
		if cfg.CmdBefore != "" {
			formatter.CmdHookMarker("cmd-before", cfg.CmdBefore)
			if err := runHook(cfg.CmdBefore, hookEnv{iteration: i, maxIterations: max, runID: cfg.RunID, label: cfg.Label, model: cfg.Model, provider: cfg.Provider}, cfg.Stderr); err != nil {
				fmt.Fprintf(cfg.Stderr, "cmd-before failed (skipping iteration %d): %v\n", i, err)
				continue
			}
		}

		start := time.Now()

		prompt := BuildPrompt(cfg.Content, i, max)
		printVerboseProviderCommand(cfg, prompt)
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
						writeSummary(cfg, stats)
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
		stats.iterations++
		stats.inputTokens += result.InputTokens
		stats.outputTokens += result.OutputTokens
		stats.cacheTokens += result.CacheTokens
		formatter.IterationStatus(time.Since(start), result.InputTokens, result.OutputTokens, result.CacheTokens)

		// Record completed iteration to log (enables --resume after crash)
		var errStr *string
		if result.Error != nil {
			s := result.Error.Error()
			errStr = &s
		}
		writeIterationLog(cfg.Log, iterationLogEntry{
			RunID:        cfg.RunID,
			Timestamp:    time.Now().UTC(),
			Iteration:    i,
			Label:        cfg.Label,
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
			env := phaseEnv{phase: "after", iteration: i, maxIter: max, runID: cfg.RunID, model: cfg.Model, provider: cfg.Provider, label: cfg.Label}
			if err := runPhaseAgent(cfg, cfg.AgentAfter, env, cfg.Stderr); err != nil {
				if errors.Is(err, ErrInterrupted) {
					writeSummary(cfg, stats)
					return ErrInterrupted
				}
				fmt.Fprintf(cfg.Stderr, "agent-after failed (iteration %d): %v\n", i, err)
			}
		}

		// Run cmd-after; log warning on failure but continue
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

		// Run stop-when; exit 0 means stop gracefully
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
func BuildWatchPrompt(taskContents, content, taskRelPath string, iteration, maxIterations int) string {
	return fmt.Sprintf("<task>\nfile: %s\n%s\n</task>\n\n%s\n\n---\nThis is iteration %d of %s, processing %s.\n",
		taskRelPath, taskContents, content, iteration, maxStr(maxIterations), taskRelPath)
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

// autoLabel returns the first ~50 characters of content as a run label.
func autoLabel(content string) string {
	content = strings.TrimSpace(content)
	const maxLen = 50
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen]
}
