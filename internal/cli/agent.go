package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/agent/daemon"
	"github.com/ohare93/juggle/internal/agent/provider"
	"github.com/ohare93/juggle/internal/session"
	"github.com/ohare93/juggle/internal/tui"
	"github.com/ohare93/juggle/internal/vcs"
	"github.com/ohare93/juggle/internal/watcher"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// isTerminal checks if the given file descriptor is a terminal
func isTerminal(fd uintptr) bool {
	return term.IsTerminal(int(fd))
}

var (
	agentIterations    int
	agentTrust         bool
	agentTimeout       time.Duration
	agentDebug         bool
	agentDryRun        bool
	agentMaxWait       time.Duration
	agentBallID        string
	agentInteractive   bool
	agentModel         string
	agentDelay         int    // Delay between iterations in minutes (overrides config)
	agentFuzz          int    // +/- variance in delay minutes (overrides config)
	agentProvider      string // Agent provider (claude, opencode)
	agentIgnoreLock    bool   // Skip lock acquisition
	agentClearProgress bool   // Clear session progress before running
	agentPickBall      bool   // Interactive ball selection
	agentMessage       string // Message to append to agent prompt
	agentMessageFlag   bool   // Track if -m flag was provided (for interactive mode)
	agentDaemon         bool   // Run in daemon mode (persists after TUI exits)
	agentMonitor        bool   // Open monitor TUI (connects to running daemon)
	agentSkipHooksCheck bool   // Skip Claude hooks check

	// Refine command flags
	refineProvider string // Agent provider for refine command
	refineModel    string // Model for refine command
	refineMessage  string // Message to append to refine prompt
)

// agentCmd is the parent command for agent operations
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run AI agents on sessions",
	Long:  `Run AI agents to work on session balls autonomously.`,
}

// agentRunCmd runs the agent loop
var agentRunCmd = &cobra.Command{
	Use:   "run [session-id]",
	Short: "Run agent loop for a session",
	Long: `Run an AI agent loop for a session.

If no session-id is provided, a selector will be shown to choose from
available sessions. Use --all to show sessions from all discovered projects.

Special session "all":
Use "all" as the session-id to run the agent against ALL balls in the current
repo, without requiring a session file. This is useful for working on balls
that aren't tagged to any specific session.

The agent will:
1. Generate a prompt using 'juggle export --format agent'
2. Spawn claude with the prepared prompt
3. Capture output to .juggle/sessions/<id>/last_output.txt
4. Check for COMPLETE/BLOCKED signals after each iteration
5. Repeat until done or max iterations reached

Rate Limit Handling:
When Claude returns a rate limit error (429 or overloaded), the agent
automatically waits with exponential backoff before retrying. If Claude
specifies a retry-after time, that time is used instead.

Examples:
  # Show session selector (interactive)
  juggle agent run

  # Run agent for 10 iterations (default)
  juggle agent run my-feature

  # Run agent against ALL balls in repo (no session filter)
  juggle agent run all

  # Run for specific number of iterations
  juggle agent run my-feature --iterations 5

  # Work on a specific ball only (1 iteration, interactive mode)
  juggle agent run my-feature --ball juggle-5

  # Work on specific ball without specifying session (uses "all" meta-session)
  juggle agent run --ball juggle-5

  # Work on specific ball with multiple iterations (non-interactive)
  juggle agent run my-feature --ball juggle-5 -n 3

  # Run in interactive mode (full Claude TUI)
  juggle agent run my-feature --interactive

  # Run with specific model (small for quick tasks, large for complex work)
  juggle agent run my-feature --model sonnet

  # Run with full permissions (dangerous)
  juggle agent run my-feature --trust

  # Run with 5 minute timeout per iteration
  juggle agent run my-feature --timeout 5m

  # Set maximum wait time for rate limits (give up if exceeded)
  juggle agent run my-feature --max-wait 30m

  # Show prompt info without running (dry run)
  juggle agent run my-feature --dry-run

  # Show prompt info before running (debug mode)
  juggle agent run my-feature --debug

  # Select from sessions across all discovered projects
  juggle agent run --all

  # Interactively select a specific ball to work on
  juggle agent run --pick

  # Select a ball from a specific session
  juggle agent run my-feature --pick

  # Select a ball from all discovered projects
  juggle agent run --pick --all

  # Override iteration delay (5 minutes, overrides config)
  juggle agent run my-feature --delay 5

  # Override delay with variance (5 minutes ± 2 minutes)
  juggle agent run my-feature --delay 5 --fuzz 2

  # Disable delay entirely (overrides config even if set)
  juggle agent run my-feature --delay 0

  # Append a message to the agent prompt
  juggle agent run my-feature -M "Focus on the authentication flow first"

  # Open interactive prompt to enter message
  juggle agent run my-feature -M`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAgentRun,
}

// agentRefineCmd launches an interactive session to review and improve ball definitions
var agentRefineCmd = &cobra.Command{
	Use:     "refine [session]",
	Aliases: []string{"refinement"},
	Short:   "Review and improve ball definitions interactively",
	Long: `Launch an interactive Claude session in plan mode to review and improve balls.

This command helps you:
- Improve acceptance criteria to be specific and testable
- Identify overlapping or duplicate work items
- Adjust priorities based on impact and dependencies
- Clarify vague intents for better autonomous execution

Ball Selection:
- No argument: Review all balls in current repo
- Session arg: Review balls with that session tag
- --all flag: Review balls from all discovered projects

Examples:
  # Review balls in current repo
  juggle agent refine

  # Review balls for a specific session
  juggle agent refine my-feature

  # Review all balls across all projects
  juggle agent refine --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAgentRefine,
}

// agentSetupRepoCmd configures Claude Code settings for optimal headless execution
var agentSetupRepoCmd = &cobra.Command{
	Use:   "setup-repo",
	Short: "Configure repository for autonomous agent operations",
	Long: `Interactively configure Claude Code settings for this repository.

Analyzes your project's technology stack and creates tailored
sandbox permissions for build tools, package managers, and dev servers.

Uses AI-assisted configuration to detect:
  - Package managers (npm, pip, cargo, go mod, etc.)
  - Build tools and compilers
  - Dev servers and their ports
  - Testing frameworks

Example:
  juggle agent setup-repo`,
	Args: cobra.NoArgs,
	RunE: runAgentSetupRepo,
}

func init() {
	agentRunCmd.Flags().IntVarP(&agentIterations, "iterations", "n", 10, "Maximum number of iterations")
	agentRunCmd.Flags().BoolVar(&agentTrust, "trust", false, "Run with --dangerously-skip-permissions (dangerous!)")
	agentRunCmd.Flags().DurationVarP(&agentTimeout, "timeout", "T", 0, "Timeout per iteration (e.g., 5m, 1h). 0 = no timeout")
	agentRunCmd.Flags().BoolVarP(&agentDebug, "debug", "d", false, "Show prompt info before running the agent")
	agentRunCmd.Flags().BoolVar(&agentDryRun, "dry-run", false, "Show prompt info without running the agent")
	agentRunCmd.Flags().DurationVar(&agentMaxWait, "max-wait", 0, "Maximum wait time for rate limits before giving up (e.g., 30m). 0 = wait indefinitely")
	agentRunCmd.Flags().StringVarP(&agentBallID, "ball", "b", "", "Work on a specific ball only (defaults to 1 iteration, interactive)")
	agentRunCmd.Flags().BoolVarP(&agentInteractive, "interactive", "i", false, "Run in interactive mode (full Claude TUI, defaults to 1 iteration)")
	agentRunCmd.Flags().StringVarP(&agentModel, "model", "m", "", "Model to use (opus, sonnet, haiku). Default: opus for large balls, sonnet for others")
	agentRunCmd.Flags().IntVar(&agentDelay, "delay", 0, "Delay between iterations in minutes (overrides config, 0 = no delay)")
	agentRunCmd.Flags().IntVar(&agentFuzz, "fuzz", 0, "Random +/- variance in delay minutes (overrides config)")
	agentRunCmd.Flags().StringVar(&agentProvider, "provider", "", "Agent provider to use (claude, opencode). Default: from config or claude")
	agentRunCmd.Flags().BoolVar(&agentIgnoreLock, "ignore-lock", false, "Skip lock acquisition (use with caution)")
	agentRunCmd.Flags().BoolVar(&agentClearProgress, "clear-progress", false, "Clear session progress before running")
	agentRunCmd.Flags().BoolVar(&agentPickBall, "pick", false, "Interactively select a ball to work on")
	agentRunCmd.Flags().StringVarP(&agentMessage, "message", "M", "", "Message to append to the agent prompt. If flag is provided without value, opens interactive input")
	agentRunCmd.Flags().BoolVar(&agentDaemon, "daemon", false, "Run agent as background daemon (persists when TUI exits)")
	agentRunCmd.Flags().BoolVar(&agentMonitor, "monitor", false, "Open monitor TUI (connects to running daemon if exists)")
	agentRunCmd.Flags().BoolVar(&agentSkipHooksCheck, "skip-hooks-check", false, "Skip Claude hooks installation check")

	// Refine command flags
	agentRefineCmd.Flags().StringVar(&refineProvider, "provider", "", "Agent provider to use (claude, opencode). Default: from config or claude")
	agentRefineCmd.Flags().StringVarP(&refineModel, "model", "m", "", "Model to use (opus, sonnet, haiku). Default: sonnet")
	agentRefineCmd.Flags().StringVarP(&refineMessage, "message", "M", "", "Message to append to the refine prompt. If flag is provided without value, opens interactive input")

	agentCmd.AddCommand(agentRunCmd)
	agentCmd.AddCommand(agentRefineCmd)
	agentCmd.AddCommand(agentSetupRepoCmd)
	rootCmd.AddCommand(agentCmd)
}

// getMessageInteractive prompts the user to enter a message interactively.
// The message can be multiple lines; an empty line followed by Enter (or Ctrl+D) completes input.
// Returns the trimmed message or empty string if cancelled.
func getMessageInteractive() (string, error) {
	// Check if stdin is a terminal
	if !isTerminal(os.Stdin.Fd()) {
		return "", fmt.Errorf("interactive message input requires a terminal (stdin is not a tty)")
	}

	fmt.Println("Enter your message (press Enter twice to finish, or Ctrl+D):")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	var lines []string
	emptyLineCount := 0

	for {
		fmt.Print("  > ")
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF (Ctrl+D) - finish input
			break
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r") // Handle Windows line endings

		if line == "" {
			emptyLineCount++
			if emptyLineCount >= 1 {
				// One empty line finishes input
				break
			}
		} else {
			// Reset empty line count if we get content
			emptyLineCount = 0
			lines = append(lines, line)
		}
	}

	message := strings.Join(lines, "\n")
	return strings.TrimSpace(message), nil
}

// AgentResult holds the result of an agent run
type AgentResult struct {
	Iterations         int           `json:"iterations"`
	Complete           bool          `json:"complete"`
	Blocked            bool          `json:"blocked"`
	BlockedReason      string        `json:"blocked_reason,omitempty"`
	TimedOut           bool          `json:"timed_out"`
	TimeoutMessage     string        `json:"timeout_message,omitempty"`
	RateLimitExceded   bool          `json:"rate_limit_exceeded"`
	TotalWaitTime      time.Duration `json:"total_wait_time,omitempty"`
	OverloadRetries    int           `json:"overload_retries,omitempty"`    // Number of 529 overload retry waits
	OverloadWaitTime   time.Duration `json:"overload_wait_time,omitempty"` // Total time spent waiting for overload recovery
	BallsComplete      int           `json:"balls_complete"`
	BallsBlocked       int           `json:"balls_blocked"`
	BallsTotal         int           `json:"balls_total"`
	StartedAt          time.Time     `json:"started_at"`
	EndedAt            time.Time     `json:"ended_at"`
	CommitMessage      string        `json:"commit_message,omitempty"` // Commit message from agent (if provided)
}

// AgentLoopConfig configures the agent loop behavior
type AgentLoopConfig struct {
	SessionID            string
	ProjectDir           string
	MaxIterations        int
	Trust                bool
	Debug                bool          // Add debug reasoning instructions to prompt
	IterDelay            time.Duration // Delay between iterations (set to 0 for tests)
	Timeout              time.Duration // Timeout per iteration (0 = no timeout)
	MaxWait              time.Duration // Maximum time to wait for rate limits (0 = wait indefinitely)
	BallID               string        // Specific ball to work on (empty = all session balls)
	Interactive          bool          // Run in interactive mode (full Claude TUI)
	Model                string        // Model to use (opus, sonnet, haiku). Empty = auto-select based on ball model_size
	OverloadRetryMinutes int           // Minutes to wait before retrying after 529 overload exhaustion (-1 = use config default, 0 = no wait)
	Provider             string        // Agent provider to use (claude, opencode). Empty = from config or claude
	IgnoreLock           bool          // Skip lock acquisition (use with caution)
	Message              string        // User message to append to the agent prompt
	DaemonMode           bool          // Run in daemon mode with file-based state and control
}

// sessionStorageID returns the session ID used for storage (progress, output, lock)
// For the "all" meta-session, this returns "_all" since "all" is reserved as a meta-session name
func sessionStorageID(sessionID string) string {
	if sessionID == "all" {
		return "_all"
	}
	return sessionID
}

// RunAgentLoop executes the agent loop with the given configuration.
// This is the testable core of the agent run command.
func RunAgentLoop(config AgentLoopConfig) (*AgentResult, error) {
	startTime := time.Now()

	sessionStore, err := session.NewSessionStore(config.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	// "all" is a special meta-session that targets all balls in repo without requiring a session file
	isAllSession := config.SessionID == "all"

	// Load session to get default model (for non-"all" sessions)
	var juggleSession *session.JuggleSession
	if !isAllSession {
		var err error
		juggleSession, err = sessionStore.LoadSession(config.SessionID)
		if err != nil {
			return nil, fmt.Errorf("session not found: %s", config.SessionID)
		}
	}

	// storageID is used for output paths and progress tracking
	// For "all" meta-session, this returns "_all"
	storageID := sessionStorageID(config.SessionID)

	// Acquire exclusive lock to prevent concurrent agent runs
	// - If IgnoreLock is true, skip locking entirely
	// - If BallID is specified, use per-ball locking (allows different balls to run concurrently)
	// - Otherwise, use session-level locking
	var lockRelease func() error
	if config.IgnoreLock {
		lockRelease = func() error { return nil }
	} else if config.BallID != "" {
		// Per-ball locking for --ball runs
		ballLock, err := session.AcquireBallLock(config.ProjectDir, config.BallID)
		if err != nil {
			return nil, err
		}
		lockRelease = ballLock.Release
	} else {
		// Session-level locking for full session runs
		lock, err := sessionStore.AcquireSessionLock(storageID)
		if err != nil {
			return nil, err
		}
		lockRelease = lock.Release
	}
	defer lockRelease()

	// Create output file path using storage ID
	// For "all" meta-session, ensure the _all session directory exists
	if isAllSession {
		allDir := filepath.Join(config.ProjectDir, ".juggle", "sessions", "_all")
		if err := os.MkdirAll(allDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create _all session directory: %w", err)
		}
	}
	outputPath := filepath.Join(config.ProjectDir, ".juggle", "sessions", storageID, "last_output.txt")

	result := &AgentResult{
		StartedAt: startTime,
	}

	// Daemon mode setup: write PID file and initial state
	var daemonPaused bool // Track pause state for daemon mode
	if config.DaemonMode {
		// Write PID file so TUI can find us
		daemonInfo := &daemon.Info{
			PID:           os.Getpid(),
			SessionID:     config.SessionID,
			ProjectDir:    config.ProjectDir,
			StartedAt:     startTime,
			MaxIterations: config.MaxIterations,
			Model:         config.Model,
			Provider:      config.Provider,
		}
		if err := daemon.WritePIDFile(config.ProjectDir, storageID, daemonInfo); err != nil {
			return nil, fmt.Errorf("failed to write daemon PID file: %w", err)
		}
		// Write initial state with Iteration=0 so monitor shows 0/X before first iteration starts
		initialState := &daemon.State{
			Running:       true,
			Iteration:     0,
			MaxIterations: config.MaxIterations,
			Model:         config.Model,
			Provider:      config.Provider,
			StartedAt:     startTime,
		}
		_ = daemon.WriteStateFile(config.ProjectDir, storageID, initialState)
		// Ensure cleanup on exit - write final state first so TUI can detect exit
		defer func() {
			// Build status message from result
			var status string
			switch {
			case result.Complete:
				status = "Complete"
			case result.Blocked:
				if result.BlockedReason != "" {
					status = result.BlockedReason
				} else if result.BallsBlocked > 0 && result.Iterations == 0 {
					status = fmt.Sprintf("No workable balls (%d blocked)", result.BallsBlocked)
				} else {
					status = "Blocked"
				}
			case result.TimedOut:
				status = "Timed out"
			case result.RateLimitExceded:
				status = "Rate limited"
			case result.OverloadRetries > 0 && result.OverloadWaitTime > 0:
				status = "Overloaded"
			default:
				status = fmt.Sprintf("%d/%d iterations", result.Iterations, config.MaxIterations)
			}

			// Write final state with Running: false so TUI monitor can detect exit
			finalState := &daemon.State{
				Running:       false,
				Iteration:     result.Iterations,
				MaxIterations: config.MaxIterations,
				StartedAt:     startTime,
				Status:        status,
			}
			_ = daemon.WriteStateFile(config.ProjectDir, storageID, finalState)
			// Clean up PID and control files (but not state file)
			daemon.CleanupPIDAndControl(config.ProjectDir, storageID)
		}()
	}

	// Track rate limit state
	var totalWaitTime time.Duration
	rateLimitRetries := 0
	rateLimitRetrying := false // Skip header when retrying after rate limit

	// Track 529 overload exhaustion state
	var overloadWaitTime time.Duration
	overloadRetries := 0
	overloadRetrying := false // Skip header when retrying after overload

	// Track crash retry state
	crashRetries := 0
	crashRetrying := false // Skip header when retrying after crash
	const maxCrashRetries = 3

	// Load overload retry interval from config (or use provided override)
	// -1 means "use config default", 0 means "no wait" (for testing), >0 is explicit minutes
	overloadRetryMinutes := config.OverloadRetryMinutes
	if overloadRetryMinutes < 0 {
		overloadRetryMinutes, _ = session.GetGlobalOverloadRetryMinutesWithOptions(GetConfigOptions())
	}

	// Configure agent provider based on CLI flag, project config, and global config
	globalProvider, err := session.GetGlobalAgentProviderWithOptions(GetConfigOptions())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load global agent provider config: %v\n", err)
	}
	projectProvider, err := session.GetProjectAgentProvider(config.ProjectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load project agent provider config: %v\n", err)
	}
	providerType := provider.Detect(config.Provider, projectProvider, globalProvider)

	// Verify provider binary is available
	if !provider.IsAvailable(providerType) {
		return nil, fmt.Errorf("agent provider %q is not available (binary %q not found in PATH)",
			providerType, provider.BinaryName(providerType))
	}

	agentProv := provider.Get(providerType)
	agent.SetProvider(agentProv)

	// Configure model overrides
	globalOverrides, err := session.GetGlobalModelOverridesWithOptions(GetConfigOptions())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load global model overrides: %v\n", err)
	}
	projectOverrides, err := session.GetProjectModelOverrides(config.ProjectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load project model overrides: %v\n", err)
	}
	modelOverrides := session.MergeModelOverrides(globalOverrides, projectOverrides)
	agent.SetModelOverrides(modelOverrides)

	// Pre-loop check: is there any work the agent can do?
	// Exit early if all balls are blocked (need human intervention) or no actionable balls exist
	// Exception: --ball or --interactive means human IS intervening, so blocked balls are workable
	workable, blockedCount, totalCount, err := countWorkableBalls(config.ProjectDir, config.SessionID, config.BallID, config.Interactive)
	if err != nil {
		return nil, fmt.Errorf("checking workable balls: %w", err)
	}

	if workable == 0 {
		result.EndedAt = time.Now()
		result.Iterations = 0
		result.BallsTotal = totalCount
		result.BallsBlocked = blockedCount

		if blockedCount > 0 {
			fmt.Fprintf(os.Stderr, "⏸ No actionable work: %d ball(s) blocked, waiting for human intervention\n", blockedCount)
			result.Blocked = true
			return result, nil
		}
		// No balls at all (all complete/researched or truly empty)
		fmt.Fprintf(os.Stderr, "✓ No actionable balls in session\n")
		result.Complete = true
		return result, nil
	}

	for iteration := 1; iteration <= config.MaxIterations; iteration++ {
		result.Iterations = iteration

		// Print iteration separator and header (skip when retrying after rate limit, overload, or crash)
		if !rateLimitRetrying && !overloadRetrying && !crashRetrying {
			if iteration > 1 {
				fmt.Println()
				fmt.Println()
				fmt.Println("════════════════════════════════════════════════════════════════════════════════")
				fmt.Println()
				fmt.Println()
			}
			fmt.Printf("════════════════════════════════ Iteration %d/%d ════════════════════════════════\n\n", iteration, config.MaxIterations)
		}
		rateLimitRetrying = false  // Reset for next iteration
		overloadRetrying = false   // Reset for next iteration
		crashRetrying = false      // Reset for next iteration

		// Record progress state before iteration (for validation)
		// Use storageID (maps "all" to "_all") for progress tracking
		progressBefore := getProgressLineCount(sessionStore, storageID)

		// Daemon mode: check for control commands and update state
		if config.DaemonMode {
			// Check for pause - wait until resumed
			for daemonPaused {
				time.Sleep(500 * time.Millisecond)
				ctrl, _ := daemon.ReadControlCommand(config.ProjectDir, storageID)
				if ctrl != nil && ctrl.Command == daemon.CmdResume {
					daemonPaused = false
					fmt.Println("▶️  Resumed by user")
				}
			}

			// Check for control commands
			ctrl, _ := daemon.ReadControlCommand(config.ProjectDir, storageID)
			if ctrl != nil {
				switch ctrl.Command {
				case daemon.CmdCancel:
					fmt.Println("🛑 Cancelled by user")
					result.Blocked = true
					result.BlockedReason = "Cancelled by user via monitor TUI"
					result.EndedAt = time.Now()
					return result, nil
				case daemon.CmdPause:
					daemonPaused = true
					fmt.Println("⏸️  Pausing after this iteration...")
				case daemon.CmdChangeModel:
					if ctrl.Args != "" {
						config.Model = ctrl.Args
						fmt.Printf("🔧 Model changed to %s for next iteration\n", ctrl.Args)
					}
				case daemon.CmdSkipBall:
					// Mark current ball as blocked and continue
					if config.BallID != "" {
						fmt.Printf("⏭️  Skipping ball %s by user request\n", config.BallID)
						// Ball skip will be handled by marking it blocked - for now just log
					}
				}
			}
		}

		// Load balls for model selection
		balls, err := loadBallsForModelSelection(config.ProjectDir, config.SessionID, config.BallID)
		if err != nil {
			return nil, fmt.Errorf("failed to load balls for model selection: %w", err)
		}

		// Check for ball-level AgentProvider override when working on a single ball
		activeBalls := filterActiveBalls(balls)
		if len(activeBalls) == 1 && activeBalls[0].AgentProvider != "" && config.Provider == "" {
			// Ball has an AgentProvider override and CLI didn't explicitly set one
			ballProvider := activeBalls[0].AgentProvider
			if provider.IsAvailable(provider.Type(ballProvider)) {
				agentProv := provider.Get(provider.Type(ballProvider))
				agent.SetProvider(agentProv)
				fmt.Printf("🔧 Provider: %s (ball %s has agent_provider override)\n", ballProvider, activeBalls[0].ShortID())
			} else {
				fmt.Fprintf(os.Stderr, "⚠️  Ball %s has agent_provider=%q but it's not available, using default\n", activeBalls[0].ShortID(), ballProvider)
			}
		}

		// Get session default model
		var sessionDefaultModel session.ModelSize
		if juggleSession != nil {
			sessionDefaultModel = juggleSession.DefaultModel
		}

		// Select optimal model for this iteration
		modelSelection := selectModelForIteration(config, balls, sessionDefaultModel)

		// Log model selection (only if not explicitly set)
		if config.Model == "" {
			fmt.Printf("🤖 Model: %s (%s)\n\n", modelSelection.Model, modelSelection.Reason)
		}

		// Daemon mode: update state file for TUI to read
		if config.DaemonMode {
			var currentBallID, currentBallTitle string
			var acsTotal int
			if len(activeBalls) > 0 {
				currentBallID = activeBalls[0].ShortID()
				currentBallTitle = activeBalls[0].Title
				acsTotal = len(activeBalls[0].AcceptanceCriteria)
			}
			state := &daemon.State{
				Running:          true,
				Paused:           daemonPaused,
				CurrentBallID:    currentBallID,
				CurrentBallTitle: currentBallTitle,
				Iteration:        iteration,
				MaxIterations:    config.MaxIterations,
				ACsComplete:      0,      // AC completion not tracked per-item currently
				ACsTotal:         acsTotal,
				Model:            modelSelection.Model,
				Provider:         string(providerType),
				StartedAt:        startTime,
			}
			// Best effort - don't fail if state write fails
			_ = daemon.WriteStateFile(config.ProjectDir, storageID, state)
		}

		// Generate prompt using export command
		prompt, err := generateAgentPrompt(config.ProjectDir, config.SessionID, config.Debug, config.BallID, config.Message)
		if err != nil {
			return nil, fmt.Errorf("failed to generate prompt: %w", err)
		}

		// Build run options
		opts := agent.RunOptions{
			Prompt:     prompt,
			Mode:       agent.ModeHeadless,
			Permission: agent.PermissionAcceptEdits,
			Timeout:    config.Timeout,
			Model:      modelSelection.Model,
		}
		if config.Interactive {
			opts.Mode = agent.ModeInteractive
		}
		if config.Trust {
			opts.Permission = agent.PermissionBypass
		}
		// Add autonomous system prompt for headless mode
		if !config.Interactive {
			opts.SystemPrompt = agent.AutonomousSystemPrompt
		}

		// Run agent with options using the Runner interface
		runResult, err := agent.DefaultRunner.Run(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to run agent: %w", err)
		}

		// Check for subprocess crash (non-zero exit, not rate limit/overload)
		if runResult.Error != nil && runResult.ExitCode != 0 && !runResult.RateLimited && !runResult.OverloadExhausted {
			waitTime := time.Duration(math.Pow(2, float64(crashRetries))) * time.Second
			if waitTime > 60*time.Second {
				waitTime = 60 * time.Second
			}

			crashRetries++
			if crashRetries > maxCrashRetries {
				return nil, fmt.Errorf("agent crashed %d times, giving up (last error: %v)", crashRetries, runResult.Error)
			}

			logCrashToProgress(config.ProjectDir, storageID,
				fmt.Sprintf("Agent crashed (exit code %d), waiting %v before retry (attempt %d/%d)",
					runResult.ExitCode, waitTime, crashRetries, maxCrashRetries))

			fmt.Printf("💥 Agent crashed (exit code %d). Waiting %v before retry (attempt %d/%d)...\n",
				runResult.ExitCode, waitTime, crashRetries, maxCrashRetries)

			waitWithCountdown(waitTime)
			crashRetrying = true

			iteration--
			continue
		}

		// Check for rate limit
		if runResult.RateLimited {
			waitTime := calculateWaitTime(runResult.RetryAfter, rateLimitRetries)

			// Check if we've exceeded max wait
			if config.MaxWait > 0 && totalWaitTime+waitTime > config.MaxWait {
				result.RateLimitExceded = true
				result.TotalWaitTime = totalWaitTime
				logRateLimitToProgress(config.ProjectDir, storageID,
					fmt.Sprintf("Rate limit exceeded max-wait of %v (total waited: %v)", config.MaxWait, totalWaitTime))
				break
			}

			// Log waiting status
			logRateLimitToProgress(config.ProjectDir, storageID,
				fmt.Sprintf("Rate limited, waiting %v before retry (attempt %d)", waitTime, rateLimitRetries+1))

			fmt.Printf("⏳ Rate limited. Waiting %v before retry...\n", waitTime)

			// Wait with countdown display
			waitWithCountdown(waitTime)

			totalWaitTime += waitTime
			rateLimitRetries++
			rateLimitRetrying = true // Skip header on retry

			// Retry this iteration (don't increment)
			iteration--
			continue
		}

		// Reset retry counters on successful run
		rateLimitRetries = 0
		crashRetries = 0

		// Check for 529 overload exhaustion (Claude's built-in retries exhausted)
		if runResult.OverloadExhausted {
			waitTime := time.Duration(overloadRetryMinutes) * time.Minute

			// Check if we've exceeded max wait
			if config.MaxWait > 0 && totalWaitTime+overloadWaitTime+waitTime > config.MaxWait {
				result.RateLimitExceded = true
				result.TotalWaitTime = totalWaitTime + overloadWaitTime
				result.OverloadRetries = overloadRetries
				result.OverloadWaitTime = overloadWaitTime
				logOverloadToProgress(config.ProjectDir, storageID,
					fmt.Sprintf("Overload retry exceeded max-wait of %v (total waited: %v)", config.MaxWait, totalWaitTime+overloadWaitTime))
				break
			}

			// Log waiting status
			logOverloadToProgress(config.ProjectDir, storageID,
				fmt.Sprintf("Claude API overloaded (529), waiting %v before retry (attempt %d)", waitTime, overloadRetries+1))

			fmt.Printf("🔥 Claude API overloaded (529). Built-in retries exhausted.\n")
			fmt.Printf("⏳ Waiting %v before restarting agent...\n", waitTime)

			// Wait with countdown display
			waitWithCountdown(waitTime)

			overloadWaitTime += waitTime
			overloadRetries++
			overloadRetrying = true // Skip header on retry

			// Retry this iteration (don't increment)
			iteration--
			continue
		}

		// Check for timeout
		if runResult.TimedOut {
			result.TimedOut = true
			result.TimeoutMessage = fmt.Sprintf("Iteration %d timed out after %v", iteration, config.Timeout)
			// Log timeout to progress
			logTimeoutToProgress(config.ProjectDir, storageID, result.TimeoutMessage)
			break
		}

		// Save output to file (ignore errors for test compatibility)
		_ = os.WriteFile(outputPath, []byte(runResult.Output), 0644)

		// Check for completion signals (already parsed by Runner)
		if runResult.Complete {
			// VALIDATE: Check if progress was updated this iteration
			progressAfter := getProgressLineCount(sessionStore, storageID)
			if progressAfter <= progressBefore {
				fmt.Println()
				fmt.Printf("⚠️  Agent signaled COMPLETE but did not update progress. Continuing iteration...\n")
				// Don't accept the signal - continue to check terminal state
			} else {
				// VALIDATE: Check if all balls are actually in terminal state (complete or blocked)
				terminal, complete, blocked, total := checkBallsTerminal(config.ProjectDir, config.SessionID, config.BallID)
				if total > 0 && terminal == total {
					// Commit changes if agent provided a commit message
					if runResult.CommitMessage != "" {
						commitResult, err := performJJCommit(config.ProjectDir, runResult.CommitMessage)
						if err == nil && commitResult != nil {
							if commitResult.Success {
								if commitResult.CommitHash != "" {
									fmt.Printf("📝 Committed: %s\n", commitResult.CommitHash)
								}
								if commitResult.StatusOutput != "No changes to commit" {
									fmt.Printf("📊 Status: %s\n", commitResult.StatusOutput)
								}
							} else if commitResult.ErrorMessage != "" {
								fmt.Printf("⚠️  Commit failed: %s\n", commitResult.ErrorMessage)
							}
						}
					}
					result.Complete = true
					result.BallsComplete = complete
					result.BallsBlocked = blocked
					result.BallsTotal = total
					result.CommitMessage = runResult.CommitMessage
					break
				}
				// Signal was premature - log warning and continue
				fmt.Println()
				fmt.Printf("⚠️  Agent signaled COMPLETE but only %d/%d balls are in terminal state (%d complete, %d blocked). Continuing...\n",
					terminal, total, complete, blocked)
			}
		}

		if runResult.Continue {
			// VALIDATE: Check if progress was updated this iteration
			progressAfter := getProgressLineCount(sessionStore, storageID)
			if progressAfter <= progressBefore {
				fmt.Println()
				fmt.Printf("⚠️  Agent signaled CONTINUE but did not update progress. Continuing iteration...\n")
				// Don't accept the signal - fall through to terminal state check
			} else {
				// Agent completed one ball, more remain - continue to next iteration
				fmt.Println()
				fmt.Printf("✓ Agent completed a ball, continuing to next iteration...\n")

				// Commit changes if agent provided a commit message
				if runResult.CommitMessage != "" {
					commitResult, err := performJJCommit(config.ProjectDir, runResult.CommitMessage)
					if err == nil && commitResult != nil {
						if commitResult.Success {
							if commitResult.CommitHash != "" {
								fmt.Printf("📝 Committed: %s\n", commitResult.CommitHash)
							}
							if commitResult.StatusOutput != "No changes to commit" {
								fmt.Printf("📊 Status: %s\n", commitResult.StatusOutput)
							}
						} else if commitResult.ErrorMessage != "" {
							fmt.Printf("⚠️  Commit failed: %s\n", commitResult.ErrorMessage)
						}
					}
				}

				// Update ball counts for progress tracking
				_, complete, blocked, total := checkBallsTerminal(config.ProjectDir, config.SessionID, config.BallID)
				result.BallsComplete = complete
				result.BallsBlocked = blocked
				result.BallsTotal = total

				continue
			}
		}

		if runResult.Blocked {
			// VALIDATE: Check if progress was updated this iteration
			progressAfter := getProgressLineCount(sessionStore, storageID)
			if progressAfter <= progressBefore {
				// No progress file update - but check VCS for uncommitted work
				// This handles cases where agent hit a blocker before running `juggle blocked`
				globalVCS, gErr := session.GetGlobalVCSWithOptions(GetConfigOptions())
				if gErr != nil {
					globalVCS = "" // Fall back to auto-detection
				}
				projectVCS, pErr := session.GetProjectVCS(config.ProjectDir)
				if pErr != nil {
					projectVCS = "" // Fall back to auto-detection
				}
				backend := vcs.GetBackendForProject(config.ProjectDir, vcs.VCSType(projectVCS), vcs.VCSType(globalVCS))

				hasChanges, vcsErr := backend.HasChanges(config.ProjectDir)
				if vcsErr == nil && hasChanges {
					// VCS shows uncommitted changes - agent was working when it hit blocker
					fmt.Println()
					fmt.Printf("🔍 Detected uncommitted changes despite no progress update\n")
					fmt.Printf("📊 Backing out work and accepting BLOCKED signal...\n")

					// Describe the working copy with BLOCKED reason
					descMsg := fmt.Sprintf("BLOCKED: %s", runResult.BlockedReason)
					if err := backend.DescribeWorkingCopy(config.ProjectDir, descMsg); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to describe working copy: %v\n", err)
					}

					// Isolate work and reset to clean state
					isolatedRev, err := backend.IsolateAndReset(config.ProjectDir, "")
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to isolate work: %v\n", err)
					} else if isolatedRev != "" {
						fmt.Printf("✓ Isolated work in revision: %s\n", isolatedRev)

						// Verify working copy is clean after reset
						if stillDirty, checkErr := backend.HasChanges(config.ProjectDir); checkErr == nil && stillDirty {
							fmt.Fprintf(os.Stderr, "Warning: working copy still has changes after reset\n")
						}
					}

					result.Blocked = true
					result.BlockedReason = runResult.BlockedReason
					break
				}

				// No VCS changes either - truly no progress
				fmt.Println()
				fmt.Printf("⚠️  Agent signaled BLOCKED but did not update progress. Continuing iteration...\n")
				// Don't accept the signal - fall through to terminal state check
			} else {
				result.Blocked = true
				result.BlockedReason = runResult.BlockedReason
				break
			}
		}

		// Check if all balls are in terminal state (complete or blocked)
		terminal, complete, blocked, total := checkBallsTerminal(config.ProjectDir, config.SessionID, config.BallID)
		result.BallsComplete = complete
		result.BallsBlocked = blocked
		result.BallsTotal = total

		if total > 0 && terminal == total {
			result.Complete = true
			break
		}

		// Delay before next iteration (unless this was the last one)
		if iteration < config.MaxIterations && config.IterDelay > 0 {
			time.Sleep(config.IterDelay)
		}
	}

	result.TotalWaitTime = totalWaitTime + overloadWaitTime
	result.OverloadRetries = overloadRetries
	result.OverloadWaitTime = overloadWaitTime
	result.EndedAt = time.Now()

	// Save run history (best-effort, don't fail the run if this errors)
	saveAgentHistory(config, result, outputPath)

	return result, nil
}

// calculateWaitTime determines how long to wait before retrying after rate limit
// Uses the explicit retry-after time if provided, otherwise exponential backoff
func calculateWaitTime(retryAfter time.Duration, retryCount int) time.Duration {
	if retryAfter > 0 {
		// Use the time specified by Claude, with a small buffer
		return retryAfter + 5*time.Second
	}

	// Exponential backoff: 30s, 1m, 2m, 4m, 8m, 16m (capped at 16m)
	baseWait := 30 * time.Second
	maxWait := 16 * time.Minute

	wait := baseWait * time.Duration(1<<retryCount) // 2^retryCount
	if wait > maxWait {
		wait = maxWait
	}

	return wait
}

// calculateFuzzyDelay calculates the actual delay to use with random variance.
// baseMinutes is the base delay in minutes, fuzz is the +/- variance in minutes.
// The actual delay will be: base + random(-fuzz, fuzz) minutes.
// The result is guaranteed to be >= 0.
func calculateFuzzyDelay(baseMinutes, fuzz int) time.Duration {
	delay := baseMinutes
	if fuzz > 0 {
		// Add random variance: -fuzz to +fuzz
		variance := rand.Intn(2*fuzz+1) - fuzz
		delay += variance
	}
	// Ensure non-negative
	if delay < 0 {
		delay = 0
	}
	return time.Duration(delay) * time.Minute
}

// CalculateFuzzyDelayForTest is an exported wrapper for testing
func CalculateFuzzyDelayForTest(baseMinutes, fuzz int) time.Duration {
	return calculateFuzzyDelay(baseMinutes, fuzz)
}

// waitWithCountdown waits for the specified duration, showing periodic countdown updates
func waitWithCountdown(duration time.Duration) {
	remaining := duration
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for remaining > 0 {
		select {
		case <-ticker.C:
			remaining -= 10 * time.Second
			if remaining > 0 {
				fmt.Printf("  ... %v remaining\n", remaining.Round(time.Second))
			}
		case <-time.After(remaining):
			return
		}
	}
}

// logRateLimitToProgress logs a rate limit event to the session's progress file
func logRateLimitToProgress(projectDir, sessionID, message string) {
	sessionStore, err := session.NewSessionStore(projectDir)
	if err != nil {
		return // Ignore errors - logging is best-effort
	}

	entry := fmt.Sprintf("[RATE_LIMIT] %s", message)
	_ = sessionStore.AppendProgress(sessionID, entry)
}

// logOverloadToProgress logs a 529 overload event to the session's progress file
func logOverloadToProgress(projectDir, sessionID, message string) {
	sessionStore, err := session.NewSessionStore(projectDir)
	if err != nil {
		return // Ignore errors - logging is best-effort
	}

	entry := fmt.Sprintf("[OVERLOAD_529] %s", message)
	_ = sessionStore.AppendProgress(sessionID, entry)
}

// logCrashToProgress logs a crash event to the session's progress file
func logCrashToProgress(projectDir, sessionID, message string) {
	sessionStore, err := session.NewSessionStore(projectDir)
	if err != nil {
		return // Ignore errors - logging is best-effort
	}

	entry := fmt.Sprintf("[CRASH] %s", message)
	_ = sessionStore.AppendProgress(sessionID, entry)
}

// SessionSelection holds the result of selecting a session for agent run
type SessionSelection struct {
	SessionID  string
	ProjectDir string
}

// selectSessionForAgent shows an interactive session selector for agent run.
// Returns the selected session info or nil if cancelled.
func selectSessionForAgent(cwd string) (*SessionSelection, error) {
	// Load config to discover projects
	config, err := LoadConfigForCommand()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create store for current directory
	store, err := NewStoreForCommand(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Get session store for local sessions
	sessionStore, err := session.NewSessionStoreWithConfig(cwd, GetStoreConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session store: %w", err)
	}

	// Collect sessions based on scope (--all flag)
	type sessionInfo struct {
		ID          string
		Description string
		ProjectDir  string
		BallCount   int
	}
	var sessions []sessionInfo

	if GlobalOpts.AllProjects {
		// Discover all projects and their sessions
		projects, err := DiscoverProjectsForCommand(config, store)
		if err != nil {
			return nil, fmt.Errorf("failed to discover projects: %w", err)
		}

		for _, projectPath := range projects {
			projSessionStore, err := session.NewSessionStore(projectPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create session store for %s: %v\n", projectPath, err)
				continue
			}

			projSessions, err := projSessionStore.ListSessions()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to list sessions for %s: %v\n", projectPath, err)
				continue
			}

			for _, s := range projSessions {
				// Count balls for this session
				balls, _ := session.LoadBallsBySession([]string{projectPath}, s.ID)
				sessions = append(sessions, sessionInfo{
					ID:          s.ID,
					Description: s.Description,
					ProjectDir:  projectPath,
					BallCount:   len(balls),
				})
			}
		}
	} else {
		// Local sessions only
		localSessions, err := sessionStore.ListSessions()
		if err != nil {
			return nil, fmt.Errorf("failed to list sessions: %w", err)
		}

		for _, s := range localSessions {
			// Count balls for this session
			balls, _ := session.LoadBallsBySession([]string{cwd}, s.ID)
			sessions = append(sessions, sessionInfo{
				ID:          s.ID,
				Description: s.Description,
				ProjectDir:  cwd,
				BallCount:   len(balls),
			})
		}
	}

	if len(sessions) == 0 {
		scopeMsg := "this project"
		if GlobalOpts.AllProjects {
			scopeMsg = "any discovered project"
		}
		return nil, fmt.Errorf("no sessions found in %s. Create one with: juggle sessions create <id>", scopeMsg)
	}

	// Display session selector
	fmt.Println("Select a session to run the agent on:")
	fmt.Println()

	for i, s := range sessions {
		prefix := fmt.Sprintf("  %d. %s", i+1, s.ID)
		ballInfo := fmt.Sprintf("(%d balls)", s.BallCount)
		if s.Description != "" {
			fmt.Printf("%s - %s %s\n", prefix, s.Description, ballInfo)
		} else {
			fmt.Printf("%s %s\n", prefix, ballInfo)
		}
		// Show project directory if viewing all projects
		if GlobalOpts.AllProjects {
			fmt.Printf("     📁 %s\n", s.ProjectDir)
		}
	}
	fmt.Println()
	fmt.Print("Enter number (or 'q' to cancel): ")

	// Read selection
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	input = strings.TrimSpace(input)

	// Handle cancel
	if input == "q" || input == "Q" || input == "" {
		return nil, nil
	}

	// Parse selection
	var idx int
	_, err = fmt.Sscanf(input, "%d", &idx)
	if err != nil || idx < 1 || idx > len(sessions) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	selected := sessions[idx-1]

	// If the selected session is from a different project, notify the user
	if selected.ProjectDir != cwd {
		fmt.Printf("\n📁 Session is in project: %s\n", selected.ProjectDir)
		fmt.Printf("   Running agent in that directory...\n\n")
	}

	return &SessionSelection{
		SessionID:  selected.ID,
		ProjectDir: selected.ProjectDir,
	}, nil
}

// SelectSessionForAgentForTest is an exported wrapper for testing
func SelectSessionForAgentForTest(cwd string) (*SessionSelection, error) {
	return selectSessionForAgent(cwd)
}

// SessionInfo holds information about a session for testing/display
type SessionInfo struct {
	ID          string
	Description string
	ProjectDir  string
	BallCount   int
}

// GetSessionsForSelectorForTest returns the list of sessions that would be shown in the selector.
// This is for testing purposes to verify cross-project session discovery.
func GetSessionsForSelectorForTest(cwd string) ([]SessionInfo, error) {
	// Load config to discover projects
	config, err := LoadConfigForCommand()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create store for current directory
	store, err := NewStoreForCommand(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Get session store for local sessions
	sessionStore, err := session.NewSessionStoreWithConfig(cwd, GetStoreConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session store: %w", err)
	}

	var sessions []SessionInfo

	if GlobalOpts.AllProjects {
		// Discover all projects and their sessions
		projects, err := DiscoverProjectsForCommand(config, store)
		if err != nil {
			return nil, fmt.Errorf("failed to discover projects: %w", err)
		}

		for _, projectPath := range projects {
			projSessionStore, err := session.NewSessionStore(projectPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create session store for %s: %v\n", projectPath, err)
				continue
			}

			projSessions, err := projSessionStore.ListSessions()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to list sessions for %s: %v\n", projectPath, err)
				continue
			}

			for _, s := range projSessions {
				// Count balls for this session
				balls, _ := session.LoadBallsBySession([]string{projectPath}, s.ID)
				sessions = append(sessions, SessionInfo{
					ID:          s.ID,
					Description: s.Description,
					ProjectDir:  projectPath,
					BallCount:   len(balls),
				})
			}
		}
	} else {
		// Local sessions only
		localSessions, err := sessionStore.ListSessions()
		if err != nil {
			return nil, fmt.Errorf("failed to list sessions: %w", err)
		}

		for _, s := range localSessions {
			// Count balls for this session
			balls, _ := session.LoadBallsBySession([]string{cwd}, s.ID)
			sessions = append(sessions, SessionInfo{
				ID:          s.ID,
				Description: s.Description,
				ProjectDir:  cwd,
				BallCount:   len(balls),
			})
		}
	}

	return sessions, nil
}

// BallSelection holds the result of selecting a ball for agent run
type BallSelection struct {
	BallID     string
	ProjectDir string
	SessionID  string // Determined from ball tags or "all"
}

// selectBallForAgent shows an interactive ball selector for agent run.
// If sessionFilter is provided, only shows balls from that session.
// Shows non-terminal balls: pending, in_progress, blocked.
// Returns the selected ball info or nil if cancelled.
func selectBallForAgent(cwd string, sessionFilter string) (*BallSelection, error) {
	// Load config to discover projects
	config, err := LoadConfigForCommand()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create store for current directory
	store, err := NewStoreForCommand(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Collect balls based on scope
	type ballInfo struct {
		Ball       *session.Ball
		ProjectDir string
	}
	var allBalls []ballInfo

	if GlobalOpts.AllProjects {
		// Discover all projects
		projects, err := DiscoverProjectsForCommand(config, store)
		if err != nil {
			return nil, fmt.Errorf("failed to discover projects: %w", err)
		}

		for _, projectPath := range projects {
			var balls []*session.Ball
			var loadErr error
			if sessionFilter != "" && sessionFilter != "all" {
				balls, loadErr = session.LoadBallsBySession([]string{projectPath}, sessionFilter)
			} else {
				balls, loadErr = session.LoadAllBalls([]string{projectPath})
			}
			if loadErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to load balls from %s: %v\n", projectPath, loadErr)
				continue
			}
			for _, b := range balls {
				allBalls = append(allBalls, ballInfo{Ball: b, ProjectDir: projectPath})
			}
		}
	} else {
		// Local project only
		var balls []*session.Ball
		var loadErr error
		if sessionFilter != "" && sessionFilter != "all" {
			balls, loadErr = session.LoadBallsBySession([]string{cwd}, sessionFilter)
		} else {
			balls, loadErr = session.LoadAllBalls([]string{cwd})
		}
		if loadErr != nil {
			return nil, fmt.Errorf("failed to load balls: %w", loadErr)
		}
		for _, b := range balls {
			allBalls = append(allBalls, ballInfo{Ball: b, ProjectDir: cwd})
		}
	}

	// Filter to non-terminal states (pending, in_progress, blocked)
	var actionable []ballInfo
	for _, bi := range allBalls {
		switch bi.Ball.State {
		case session.StatePending, session.StateInProgress, session.StateBlocked:
			actionable = append(actionable, bi)
		}
	}

	if len(actionable) == 0 {
		scopeMsg := "this project"
		if GlobalOpts.AllProjects {
			scopeMsg = "any discovered project"
		}
		filterMsg := ""
		if sessionFilter != "" && sessionFilter != "all" {
			filterMsg = fmt.Sprintf(" in session '%s'", sessionFilter)
		}
		return nil, fmt.Errorf("no actionable balls found%s in %s (all balls are complete or none exist)", filterMsg, scopeMsg)
	}

	// Sort: in_progress first, then pending, then blocked
	stateOrder := func(s session.BallState) int {
		switch s {
		case session.StateInProgress:
			return 0
		case session.StatePending:
			return 1
		case session.StateBlocked:
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(actionable, func(i, j int) bool {
		return stateOrder(actionable[i].Ball.State) < stateOrder(actionable[j].Ball.State)
	})

	// Compute minimal unique IDs for display
	balls := make([]*session.Ball, len(actionable))
	for i, bi := range actionable {
		balls[i] = bi.Ball
	}
	minIDs := session.ComputeMinimalUniqueIDs(balls)

	// Display ball selector
	fmt.Println("Select a ball to work on:")
	fmt.Println()

	for i, bi := range actionable {
		ball := bi.Ball
		stateLabel := fmt.Sprintf("[%s]", ball.State)
		minID := minIDs[ball.ID]
		priorityLabel := fmt.Sprintf("(%s)", ball.Priority)
		fmt.Printf("  %d. %s %s - %s %s\n", i+1, stateLabel, minID, ball.Title, priorityLabel)

		// Show blocked reason if blocked
		if ball.State == session.StateBlocked && ball.BlockedReason != "" {
			fmt.Printf("     Reason: %s\n", ball.BlockedReason)
		}

		// Show project directory if viewing all projects
		if GlobalOpts.AllProjects {
			fmt.Printf("     📁 %s\n", bi.ProjectDir)
		}
	}
	fmt.Println()
	fmt.Print("Enter number (or 'q' to cancel): ")

	// Read selection
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	input = strings.TrimSpace(input)

	// Handle cancel
	if input == "q" || input == "Q" || input == "" {
		return nil, nil
	}

	// Parse selection
	var idx int
	_, err = fmt.Sscanf(input, "%d", &idx)
	if err != nil || idx < 1 || idx > len(actionable) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	selected := actionable[idx-1]

	// Determine session ID: prefer the filter, then ball's first tag, then "all"
	sessionID := "all"
	if sessionFilter != "" && sessionFilter != "all" {
		sessionID = sessionFilter
	} else if len(selected.Ball.Tags) > 0 {
		sessionID = selected.Ball.Tags[0]
	}

	// If the selected ball is from a different project, notify the user
	if selected.ProjectDir != cwd {
		fmt.Printf("\n📁 Ball is in project: %s\n", selected.ProjectDir)
		fmt.Printf("   Running agent in that directory...\n\n")
	}

	return &BallSelection{
		BallID:     selected.Ball.ID,
		ProjectDir: selected.ProjectDir,
		SessionID:  sessionID,
	}, nil
}

// SelectBallForAgentForTest is an exported wrapper for testing
func SelectBallForAgentForTest(cwd string, sessionFilter string) (*BallSelection, error) {
	return selectBallForAgent(cwd, sessionFilter)
}

func runAgentRun(cmd *cobra.Command, args []string) error {
	// Get current directory
	cwd, err := GetWorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Track which project directory to use (may change if session is in different project)
	projectDir := cwd

	// Handle --monitor flag: start daemon if needed and open monitor TUI
	if agentMonitor {
		if len(args) == 0 {
			return fmt.Errorf("--monitor requires a session ID argument")
		}
		sessionID := args[0]
		storageID := sessionStorageID(sessionID)

		// Check if a daemon is running for this session
		running, _, err := daemon.IsRunning(projectDir, storageID)
		if err != nil {
			return fmt.Errorf("failed to check daemon status: %w", err)
		}

		if !running {
			// No daemon running - start one in the background
			fmt.Printf("Starting agent daemon for session %s...\n", sessionID)

			// Ensure session directory exists for log file
			logPath := filepath.Join(projectDir, ".juggle", "sessions", storageID, "agent.log")
			if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
				return fmt.Errorf("failed to create session directory: %w", err)
			}

			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return fmt.Errorf("failed to create log file: %w", err)
			}

			// Build daemon command
			daemonCmd := exec.Command(os.Args[0], "agent", "run", "--daemon", sessionID)
			daemonCmd.Env = append(os.Environ(), "JUGGLE_DAEMON_CHILD=1")
			daemonCmd.Stdout = logFile
			daemonCmd.Stderr = logFile
			daemonCmd.Dir = projectDir

			if err := daemonCmd.Start(); err != nil {
				logFile.Close()
				return fmt.Errorf("failed to start daemon: %w", err)
			}

			fmt.Printf("Agent daemon started (PID %d)\n", daemonCmd.Process.Pid)
			logFile.Close()

			// Give the daemon a moment to initialize and write PID file
			time.Sleep(500 * time.Millisecond)
			running = true
		}

		// Launch monitor TUI
		return launchMonitorTUI(projectDir, sessionID, storageID, running)
	}

	// Handle --pick flag (interactive ball selection)
	if agentPickBall {
		// --pick and --ball are mutually exclusive
		if agentBallID != "" {
			return fmt.Errorf("cannot use --pick with --ball (they are mutually exclusive)")
		}

		// Check terminal for interactive mode
		if !isTerminal(os.Stdin.Fd()) {
			return fmt.Errorf("--pick requires interactive terminal")
		}

		// Determine session filter from args
		var sessionFilter string
		if len(args) > 0 {
			sessionFilter = args[0]
		}

		selected, err := selectBallForAgent(cwd, sessionFilter)
		if err != nil {
			return err
		}
		if selected == nil {
			// User cancelled
			return nil
		}

		// Set ball ID and session from selection
		agentBallID = selected.BallID
		projectDir = selected.ProjectDir

		// Clear session progress if requested
		if agentClearProgress {
			sessionStore, err := session.NewSessionStoreWithConfig(projectDir, GetStoreConfig())
			if err != nil {
				return fmt.Errorf("failed to initialize session store: %w", err)
			}

			clearID := selected.SessionID
			if selected.SessionID == "all" {
				clearID = "_all"
			}

			if err := sessionStore.ClearProgress(clearID); err != nil {
				return fmt.Errorf("failed to clear progress: %w", err)
			}
			fmt.Printf("Cleared progress for session: %s\n", selected.SessionID)
			fmt.Println()
		}

		// Run agent loop for the selected ball
		_, err = RunAgentLoop(AgentLoopConfig{
			SessionID:     selected.SessionID,
			ProjectDir:    projectDir,
			MaxIterations: 1,
			BallID:        agentBallID,
			Interactive:   true,
			Model:         agentModel,
			IterDelay:     0,
			Timeout:       agentTimeout,
			Trust:         agentTrust,
			MaxWait:       agentMaxWait,
			Provider:      agentProvider,
			IgnoreLock:    agentIgnoreLock,
		})
		return err
	}

	// Determine session ID from args or selector
	var sessionID string
	if len(args) > 0 {
		sessionID = args[0]
	} else if agentBallID != "" {
		// --ball specified without session - default to "all" meta-session
		sessionID = "all"
	} else {
		// No session provided - check if stdin is a terminal before showing selector
		// In headless mode (no tty), error gracefully instead of hanging on selector
		if !isTerminal(os.Stdin.Fd()) {
			return fmt.Errorf("session-id is required in non-interactive mode (use 'all' to target all balls)")
		}
		// Show selector
		selected, err := selectSessionForAgent(cwd)
		if err != nil {
			return err
		}
		if selected == nil {
			// User cancelled
			return nil
		}
		sessionID = selected.SessionID
		projectDir = selected.ProjectDir
	}

	// Determine iterations and interactive mode
	// Default to 1 iteration when --ball or --interactive is specified (unless -n was explicitly set)
	iterations := agentIterations
	interactive := agentInteractive
	if (agentBallID != "" || agentInteractive) && !cmd.Flags().Changed("iterations") {
		iterations = 1
	}
	// --ball implies interactive mode (unless -n was explicitly set for multiple iterations)
	if agentBallID != "" && !cmd.Flags().Changed("iterations") {
		interactive = true
	}

	// Handle --message flag
	// If flag was provided but value is empty, prompt for interactive input
	message := agentMessage
	if cmd.Flags().Changed("message") && message == "" {
		var err error
		message, err = getMessageInteractive()
		if err != nil {
			return err
		}
		if message == "" {
			fmt.Println("No message provided, continuing without additional context.")
			fmt.Println()
		}
	}

	// Handle --dry-run and --debug: show prompt info
	if agentDryRun || agentDebug {
		prompt, err := generateAgentPrompt(projectDir, sessionID, true, agentBallID, message) // debug=true for reasoning instructions
		if err != nil {
			return fmt.Errorf("failed to generate prompt: %w", err)
		}

		fmt.Println("=== Agent Prompt Info ===")
		fmt.Println()
		fmt.Printf("Session: %s\n", sessionID)
		if agentBallID != "" {
			fmt.Printf("Ball: %s\n", agentBallID)
		}
		fmt.Printf("Max iterations: %d\n", iterations)
		fmt.Printf("Trust mode: %v\n", agentTrust)
		fmt.Printf("Interactive mode: %v\n", interactive)
		if agentModel != "" {
			fmt.Printf("Model: %s\n", agentModel)
		}
		if agentProvider != "" {
			fmt.Printf("Provider: %s\n", agentProvider)
		}
		if message != "" {
			fmt.Printf("Message: (appended to prompt)\n")
		}
		if agentTimeout > 0 {
			fmt.Printf("Timeout per iteration: %v\n", agentTimeout)
		}
		if agentMaxWait > 0 {
			fmt.Printf("Max rate limit wait: %v\n", agentMaxWait)
		}
		fmt.Println()
		fmt.Println("=== Generated Prompt ===")
		fmt.Println()
		fmt.Println(prompt)
		fmt.Println()
		fmt.Printf("=== Prompt Length: %d characters ===\n", len(prompt))

		// If dry-run, exit without running
		if agentDryRun {
			fmt.Println()
			fmt.Println("(Dry run - agent not started)")
			return nil
		}

		// If debug, continue to run the agent
		fmt.Println()
		fmt.Println("=== Starting Agent ===")
		fmt.Println()
	}

	// Print warning if --trust is used
	if agentTrust {
		fmt.Println("⚠️  WARNING: Running with --trust flag. Agent has full system permissions.")
		fmt.Println("    Only use this if you trust the agent and understand the risks.")
		fmt.Println()
	}

	if agentBallID != "" {
		fmt.Printf("Starting agent for ball: %s (session: %s)\n", agentBallID, sessionID)
	} else {
		fmt.Printf("Starting agent for session: %s\n", sessionID)
	}
	fmt.Printf("Max iterations: %d\n", iterations)
	fmt.Println()

	// Print timeout if specified
	if agentTimeout > 0 {
		fmt.Printf("Timeout per iteration: %v\n", agentTimeout)
	}

	// Load iteration delay settings (flags override config)
	var iterDelay time.Duration
	var delayMinutes, fuzz int

	// Check if --delay flag was explicitly provided
	if cmd.Flags().Changed("delay") {
		delayMinutes = agentDelay
		// Check if --fuzz was also provided, otherwise default to 0
		if cmd.Flags().Changed("fuzz") {
			fuzz = agentFuzz
		}
	} else {
		// Load from config
		var err error
		delayMinutes, fuzz, err = session.GetGlobalIterationDelayWithOptions(GetConfigOptions())
		if err != nil {
			delayMinutes = 0
			fuzz = 0
		}
		// Override fuzz from flag if set
		if cmd.Flags().Changed("fuzz") {
			fuzz = agentFuzz
		}
	}

	// If delay is 0, skip the delay feature entirely (regardless of fuzz)
	if delayMinutes > 0 {
		iterDelay = calculateFuzzyDelay(delayMinutes, fuzz)
		fmt.Printf("Iteration delay: %v", iterDelay.Round(time.Second))
		if fuzz > 0 {
			fmt.Printf(" (base: %dm ± %dm)", delayMinutes, fuzz)
		}
		fmt.Println()
	}

	// Clear session progress if requested
	if agentClearProgress {
		sessionStore, err := session.NewSessionStoreWithConfig(projectDir, GetStoreConfig())
		if err != nil {
			return fmt.Errorf("failed to initialize session store: %w", err)
		}

		// Normalize session ID for clearing
		clearID := sessionID
		if sessionID == "all" {
			clearID = "_all"
		}

		if err := sessionStore.ClearProgress(clearID); err != nil {
			return fmt.Errorf("failed to clear progress: %w", err)
		}
		fmt.Printf("Cleared progress for session: %s\n", sessionID)
		fmt.Println()
	}

	// Handle daemon mode: fork to background if not already the child process
	storageID := sessionStorageID(sessionID)
	if os.Getenv("JUGGLE_DAEMON_CHILD") == "1" {
		// We are the daemon child process - clear the env var and continue
		os.Unsetenv("JUGGLE_DAEMON_CHILD")

		// Set up signal handling for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
		go func() {
			<-sigChan
			// Send cancel command to self via control file
			daemon.SendControlCommand(projectDir, storageID, daemon.CmdCancel, "signal")
		}()
	} else if agentDaemon {
		// We are the parent - fork a child process and exit

		// Ensure session directory exists for log file
		logPath := filepath.Join(projectDir, ".juggle", "sessions", storageID, "agent.log")
		if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
			return fmt.Errorf("failed to create session directory: %w", err)
		}

		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}

		// Re-exec ourselves with the same arguments
		daemonCmd := exec.Command(os.Args[0], os.Args[1:]...)
		daemonCmd.Env = append(os.Environ(), "JUGGLE_DAEMON_CHILD=1")
		daemonCmd.Stdout = logFile
		daemonCmd.Stderr = logFile
		daemonCmd.Dir = projectDir

		if err := daemonCmd.Start(); err != nil {
			logFile.Close()
			return fmt.Errorf("failed to start daemon: %w", err)
		}

		fmt.Printf("Agent daemon started (PID %d)\n", daemonCmd.Process.Pid)
		fmt.Printf("Log file: %s\n", logPath)
		fmt.Printf("Monitor with: juggle agent run --monitor %s\n", sessionID)

		logFile.Close()
		return nil // Parent exits here
	}

	// Check Claude settings for optimal headless execution
	if !agentSkipHooksCheck && !agentDaemon && isTerminal(os.Stdin.Fd()) {
		if !shouldSkipClaudeSetupCheck() {
			issues := checkClaudeSettings()

			// Check if there are any actual problems (not just status messages)
			hasProblems := false
			for _, issue := range issues {
				if strings.HasPrefix(issue, "✗") {
					hasProblems = true
					break
				}
			}

			if hasProblems {
				fmt.Println("Claude Code is not configured for optimal headless execution:")
				for _, issue := range issues {
					fmt.Println("  " + issue)
				}
				fmt.Println("")

				response := promptSetupOrSkip()
				switch response {
				case "y":
					cwd, _ := GetWorkingDir()
					if err := InitProject(InitOptions{
						TargetDir:            cwd,
						JuggleDirName:        ".juggle",
						CreateClaudeSettings: true,
						Output:               os.Stdout,
					}); err != nil {
						fmt.Printf("Setup failed: %v\n", err)
					}
				case "never":
					if err := saveSkipClaudeSetupCheck(); err != nil {
						fmt.Printf("Warning: could not save preference: %v\n", err)
					} else {
						fmt.Println("Preference saved. Run 'juggle init' to set up later.")
					}
				}
			}
		}
	}

	// Check if Claude hooks are installed for enhanced progress tracking
	if !agentSkipHooksCheck && !agentDaemon && isTerminal(os.Stdin.Fd()) {
		if !AreHooksInstalled() {
			fmt.Println("Claude Code hooks are not installed.")
			fmt.Println("Hooks provide enhanced progress tracking: file changes, tool counts, token usage.")
			fmt.Print("\nInstall hooks now? [Y/n] ")

			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response == "" || response == "y" || response == "yes" {
				if err := runHooksInstall(nil, nil); err != nil {
					fmt.Printf("Warning: failed to install hooks: %v\n", err)
				}
				fmt.Println()
			}
		}
	}

	// Run the agent loop
	loopConfig := AgentLoopConfig{
		SessionID:            sessionID,
		ProjectDir:           projectDir,
		MaxIterations:        iterations,
		Trust:                agentTrust,
		Debug:                false, // Debug mode now just shows prompt info, doesn't affect prompt content
		IterDelay:            iterDelay,
		Timeout:              agentTimeout,
		MaxWait:              agentMaxWait,
		BallID:               agentBallID,
		Interactive:          interactive,
		Model:                agentModel,
		OverloadRetryMinutes: -1,              // Use config default
		Provider:             agentProvider,   // Use CLI flag (empty = auto-detect from config)
		IgnoreLock:           agentIgnoreLock, // Skip lock acquisition if set
		Message:              message,         // User message to append to prompt
		DaemonMode:           agentDaemon,     // Run as daemon with file-based state/control
	}

	result, err := RunAgentLoop(loopConfig)
	if err != nil {
		return err
	}

	elapsed := result.EndedAt.Sub(result.StartedAt)

	// Print summary
	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Iterations: %d\n", result.Iterations)
	fmt.Printf("Balls: %d complete, %d blocked, %d total\n", result.BallsComplete, result.BallsBlocked, result.BallsTotal)
	fmt.Printf("Time elapsed: %s\n", elapsed.Round(time.Second))

	if result.TotalWaitTime > 0 {
		fmt.Printf("Total wait time: %v\n", result.TotalWaitTime.Round(time.Second))
		if result.OverloadRetries > 0 {
			fmt.Printf("  - Overload (529) retries: %d (waited %v)\n", result.OverloadRetries, result.OverloadWaitTime.Round(time.Second))
		}
	}

	if result.Complete {
		fmt.Println("Status: COMPLETE")
	} else if result.Blocked {
		fmt.Printf("Status: BLOCKED (%s)\n", result.BlockedReason)
	} else if result.TimedOut {
		fmt.Printf("Status: TIMEOUT (%s)\n", result.TimeoutMessage)
	} else if result.RateLimitExceded {
		fmt.Printf("Status: RATE_LIMIT_EXCEEDED (max-wait: %v)\n", agentMaxWait)
	} else {
		fmt.Println("Status: Max iterations reached")
	}

	// Map "all" meta-session to "_all" for output path
	outputStorageID := sessionStorageID(sessionID)
	outputPath := filepath.Join(projectDir, ".juggle", "sessions", outputStorageID, "last_output.txt")
	fmt.Printf("\nOutput saved to: %s\n", outputPath)

	return nil
}

// launchMonitorTUI launches the TUI in agent monitor mode
func launchMonitorTUI(projectDir, sessionID, storageID string, daemonRunning bool) error {
	// Load config
	config, err := session.LoadConfig()
	if err != nil {
		return err
	}

	// Create store
	store, err := session.NewStore(projectDir)
	if err != nil {
		return err
	}

	// Create session store
	sessionStore, err := session.NewSessionStore(projectDir)
	if err != nil {
		return err
	}

	// Create file watcher
	w, err := watcher.New()
	if err != nil {
		return err
	}
	defer w.Close()

	// Watch the project directory
	if err := w.WatchProject(projectDir); err != nil {
		// Non-fatal: continue without watching if it fails
		w = nil
	} else {
		w.Start()
	}

	// Create model in monitor mode
	model := tui.InitialMonitorModel(store, sessionStore, config, true, w, storageID, daemonRunning)

	// Create program with alternate screen
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run
	_, err = p.Run()
	return err
}

// generateAgentPrompt generates the agent prompt using export command.
// The message parameter, if non-empty, is appended to the end of the generated prompt.
func generateAgentPrompt(projectDir, sessionID string, debug bool, ballID string, message string) (string, error) {
	// Use the export functionality directly instead of shelling out
	// This is more efficient and avoids subprocess overhead

	// Load config to discover projects
	config, err := LoadConfigForCommand()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	// Create store for current directory
	store, err := NewStoreForCommand(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to create store: %w", err)
	}

	// Discover projects
	projects, err := DiscoverProjectsForCommand(config, store)
	if err != nil {
		return "", fmt.Errorf("failed to discover projects: %w", err)
	}

	if len(projects) == 0 {
		return "", fmt.Errorf("no projects with .juggle directories found")
	}

	// Load all balls from discovered projects
	allBalls, err := session.LoadAllBalls(projects)
	if err != nil {
		return "", fmt.Errorf("failed to load balls: %w", err)
	}

	// Filter by session tag
	// "all" is a special meta-session that means "all balls in repo" (no session filtering)
	var balls []*session.Ball
	if sessionID == "all" {
		balls = allBalls
	} else {
		balls = make([]*session.Ball, 0)
		for _, ball := range allBalls {
			for _, tag := range ball.Tags {
				if tag == sessionID {
					balls = append(balls, ball)
					break
				}
			}
		}
	}

	// Filter out complete and blocked balls by default (they clutter the context for no gain)
	// Exception: when a specific ball is requested, allow it even if complete/blocked
	if ballID == "" {
		filteredBalls := make([]*session.Ball, 0, len(balls))
		for _, ball := range balls {
			if ball.State != session.StateComplete && ball.State != session.StateResearched && ball.State != session.StateBlocked {
				filteredBalls = append(filteredBalls, ball)
			}
		}
		balls = filteredBalls
	}

	// Filter to specific ball if ballID is specified
	singleBall := false
	if ballID != "" {
		matches := session.ResolveBallByPrefix(balls, ballID)
		if len(matches) == 0 {
			return "", fmt.Errorf("ball %s not found in session %s", ballID, sessionID)
		}
		if len(matches) > 1 {
			matchingIDs := make([]string, len(matches))
			for i, m := range matches {
				matchingIDs[i] = m.ID
			}
			return "", fmt.Errorf("ambiguous ID '%s' matches %d balls: %s", ballID, len(matches), strings.Join(matchingIDs, ", "))
		}
		balls = []*session.Ball{matches[0]}
		singleBall = true
	}

	// Call exportAgent directly
	output, err := exportAgent(projectDir, sessionID, balls, debug, singleBall)
	if err != nil {
		return "", err
	}

	prompt := string(output)

	// Append user message if provided
	if message != "" {
		prompt += "\n<user-message>\n" + message + "\n</user-message>\n"
	}

	return prompt, nil
}

// countWorkableBalls returns counts of balls the agent can work on (pending/in_progress) vs blocked
// This is used for pre-loop validation to exit early when there's no actionable work
// Balls in complete/researched states are excluded (same as agent export)
// If ballID is specified, only counts that specific ball
// If interactive is true, blocked balls are treated as workable (human is present to intervene)
// "all" is a special meta-session that includes all balls in the repo without filtering by tag
func countWorkableBalls(projectDir, sessionID, ballID string, interactive bool) (workable, blocked, total int, err error) {
	// Load config
	config, err := LoadConfigForCommand()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to load config: %w", err)
	}

	// Create store
	store, err := NewStoreForCommand(projectDir)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to create store: %w", err)
	}

	// Discover projects
	projects, err := DiscoverProjectsForCommand(config, store)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to discover projects: %w", err)
	}

	// Load all balls
	allBalls, err := session.LoadAllBalls(projects)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to load balls: %w", err)
	}

	// "all" is a meta-session that means "all balls in repo"
	isAllSession := sessionID == "all"

	// Count balls with session tag (or all balls if using "all" meta-session)
	for _, ball := range allBalls {
		var matchesSession bool
		if isAllSession {
			matchesSession = true // Include all balls
		} else {
			for _, tag := range ball.Tags {
				if tag == sessionID {
					matchesSession = true
					break
				}
			}
		}

		if matchesSession {
			// If filtering by specific ball, skip others
			if ballID != "" && ball.ID != ballID && ball.ShortID() != ballID {
				continue
			}

			// Skip states that are excluded from agent exports
			// (complete, researched are not shown to the agent)
			switch ball.State {
			case session.StateComplete, session.StateResearched:
				continue
			case session.StatePending, session.StateInProgress:
				workable++
				total++
			case session.StateBlocked:
				// If user is running interactively or explicitly targeted this ball,
				// treat it as workable (they ARE the human intervention)
				if interactive || (ballID != "" && (ball.ID == ballID || ball.ShortID() == ballID)) {
					workable++
				} else {
					blocked++
				}
				total++
			}
		}
	}

	return workable, blocked, total, nil
}

// checkBallsTerminal returns counts of balls in terminal states (complete or blocked) and total balls for session
// If ballID is specified, only counts that specific ball
// "all" is a special meta-session that includes all balls in the repo without filtering by tag
func checkBallsTerminal(projectDir, sessionID, ballID string) (terminal, complete, blocked, total int) {
	// Load config
	config, err := LoadConfigForCommand()
	if err != nil {
		return 0, 0, 0, 0
	}

	// Create store
	store, err := NewStoreForCommand(projectDir)
	if err != nil {
		return 0, 0, 0, 0
	}

	// Discover projects
	projects, err := DiscoverProjectsForCommand(config, store)
	if err != nil {
		return 0, 0, 0, 0
	}

	// Load all balls
	allBalls, err := session.LoadAllBalls(projects)
	if err != nil {
		return 0, 0, 0, 0
	}

	// "all" is a meta-session that means "all balls in repo"
	isAllSession := sessionID == "all"

	// Count balls with session tag (or all balls if using "all" meta-session)
	for _, ball := range allBalls {
		var matchesSession bool
		if isAllSession {
			matchesSession = true // Include all balls
		} else {
			for _, tag := range ball.Tags {
				if tag == sessionID {
					matchesSession = true
					break
				}
			}
		}

		if matchesSession {
			// If filtering by specific ball, skip others
			if ballID != "" && ball.ID != ballID && ball.ShortID() != ballID {
				continue
			}
			total++
			if ball.State == session.StateComplete {
				complete++
				terminal++
			} else if ball.State == session.StateBlocked {
				blocked++
				terminal++
			}
		}
	}

	return terminal, complete, blocked, total
}

// logTimeoutToProgress logs a timeout event to the session's progress file
func logTimeoutToProgress(projectDir, sessionID, message string) {
	sessionStore, err := session.NewSessionStore(projectDir)
	if err != nil {
		return // Ignore errors - logging is best-effort
	}

	entry := fmt.Sprintf("[TIMEOUT] %s", message)
	_ = sessionStore.AppendProgress(sessionID, entry)
}

// getProgressLineCount returns the number of lines in the session's progress file.
// Used to detect if progress was updated during an iteration.
func getProgressLineCount(store *session.SessionStore, sessionID string) int {
	progress, err := store.LoadProgress(sessionID)
	if err != nil {
		return 0
	}
	if progress == "" {
		return 0
	}
	// Count newlines + 1 for content (unless trailing newline only)
	lines := strings.Count(progress, "\n")
	// If there's content but no trailing newline, add 1
	if len(progress) > 0 && !strings.HasSuffix(progress, "\n") {
		lines++
	}
	return lines
}

// GetProgressLineCountForTest is an exported wrapper for testing
func GetProgressLineCountForTest(store *session.SessionStore, sessionID string) int {
	return getProgressLineCount(store, sessionID)
}

// saveAgentHistory saves the agent run history to the history file
func saveAgentHistory(config AgentLoopConfig, result *AgentResult, outputPath string) {
	historyStore, err := session.NewAgentHistoryStore(config.ProjectDir)
	if err != nil {
		return // Best-effort, ignore errors
	}

	record := session.NewAgentRunRecord(config.SessionID, config.ProjectDir, result.StartedAt)
	record.MaxIterations = config.MaxIterations
	record.OutputFile = outputPath

	// Set the appropriate result type
	if result.Complete {
		record.SetComplete(result.Iterations, result.BallsComplete, result.BallsBlocked, result.BallsTotal)
	} else if result.Blocked {
		record.SetBlocked(result.Iterations, result.BlockedReason, result.BallsComplete, result.BallsBlocked, result.BallsTotal)
	} else if result.TimedOut {
		record.SetTimeout(result.Iterations, result.TimeoutMessage, result.BallsComplete, result.BallsBlocked, result.BallsTotal)
	} else if result.RateLimitExceded {
		record.SetRateLimitExceeded(result.Iterations, result.TotalWaitTime, result.BallsComplete, result.BallsBlocked, result.BallsTotal)
	} else {
		// Max iterations reached
		record.SetMaxIterations(result.Iterations, result.BallsComplete, result.BallsBlocked, result.BallsTotal)
	}

	// Preserve total wait time and ended time from result
	record.TotalWaitTime = result.TotalWaitTime
	record.EndedAt = result.EndedAt

	_ = historyStore.AppendRecord(record)
}

// runAgentRefine implements the agent refine command
func runAgentRefine(cmd *cobra.Command, args []string) error {
	// Parse optional session argument
	var sessionID string
	if len(args) > 0 {
		sessionID = args[0]
	}

	// Get current directory
	cwd, err := GetWorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Handle --message flag
	// If flag was provided but value is empty, prompt for interactive input
	message := refineMessage
	if cmd.Flags().Changed("message") && message == "" {
		var err error
		message, err = getMessageInteractive()
		if err != nil {
			return err
		}
		if message == "" {
			fmt.Println("No message provided, continuing without additional context.")
			fmt.Println()
		}
	}

	// Load balls based on scope
	balls, err := loadBallsForRefine(cwd, sessionID)
	if err != nil {
		return fmt.Errorf("failed to load balls: %w", err)
	}

	if len(balls) == 0 {
		fmt.Println("No balls found to refine.")
		return nil
	}

	// Generate the refinement prompt
	prompt, err := generateRefinePrompt(cwd, sessionID, balls, message)
	if err != nil {
		return fmt.Errorf("failed to generate prompt: %w", err)
	}

	fmt.Printf("Starting refinement session for %d ball(s)...\n", len(balls))
	if sessionID != "" {
		fmt.Printf("Session filter: %s\n", sessionID)
	}
	fmt.Println()

	// Configure agent provider
	globalProvider, err := session.GetGlobalAgentProviderWithOptions(GetConfigOptions())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load global agent provider config: %v\n", err)
	}
	projectProvider, err := session.GetProjectAgentProvider(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load project agent provider config: %v\n", err)
	}
	providerType := provider.Detect(refineProvider, projectProvider, globalProvider)

	// Verify provider binary is available
	if !provider.IsAvailable(providerType) {
		return fmt.Errorf("agent provider %q is not available (binary %q not found in PATH)",
			providerType, provider.BinaryName(providerType))
	}

	agentProv := provider.Get(providerType)
	agent.SetProvider(agentProv)

	// Configure model overrides
	globalOverrides, err := session.GetGlobalModelOverridesWithOptions(GetConfigOptions())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load global model overrides: %v\n", err)
	}
	projectOverrides, err := session.GetProjectModelOverrides(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load project model overrides: %v\n", err)
	}
	modelOverrides := session.MergeModelOverrides(globalOverrides, projectOverrides)
	agent.SetModelOverrides(modelOverrides)

	// Run agent in interactive + plan mode
	opts := agent.RunOptions{
		Prompt:     prompt,
		Mode:       agent.ModeInteractive,
		Permission: agent.PermissionPlan,
		Model:      refineModel, // Use model flag if provided
		WorkingDir: cwd,
	}

	_, err = agent.DefaultRunner.Run(opts)
	if err != nil {
		return fmt.Errorf("refinement session failed: %w", err)
	}

	return nil
}

// loadBallsForRefine loads balls based on scope:
// - If sessionID provided, filter by session tag
// - If GlobalOpts.AllProjects, load from all discovered projects
// - Otherwise, load from current repo only
func loadBallsForRefine(projectDir, sessionID string) ([]*session.Ball, error) {
	// Load config to discover projects
	config, err := LoadConfigForCommand()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create store for current directory
	store, err := NewStoreForCommand(projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Discover projects (respects --all flag)
	projects, err := DiscoverProjectsForCommand(config, store)
	if err != nil {
		return nil, fmt.Errorf("failed to discover projects: %w", err)
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects with .juggle directories found")
	}

	// Load all balls from discovered projects
	allBalls, err := session.LoadAllBalls(projects)
	if err != nil {
		return nil, fmt.Errorf("failed to load balls: %w", err)
	}

	// If no session filter or "all" is specified, return all non-complete balls
	// "all" is a special meta-session meaning "all balls in repo"
	if sessionID == "" || sessionID == "all" {
		balls := make([]*session.Ball, 0)
		for _, ball := range allBalls {
			if ball.State != session.StateComplete {
				balls = append(balls, ball)
			}
		}
		return balls, nil
	}

	// Filter by session tag
	balls := make([]*session.Ball, 0)
	for _, ball := range allBalls {
		for _, tag := range ball.Tags {
			if tag == sessionID {
				balls = append(balls, ball)
				break
			}
		}
	}

	return balls, nil
}

// generateRefinePrompt creates the prompt for the refinement session.
// The message parameter, if non-empty, is appended to the end of the generated prompt.
func generateRefinePrompt(projectDir, sessionID string, balls []*session.Ball, message string) (string, error) {
	var buf strings.Builder

	// Write context section with session info if available
	buf.WriteString("<context>\n")
	if sessionID != "" {
		// Try to load session context
		sessionStore, err := session.NewSessionStore(projectDir)
		if err == nil {
			juggleSession, err := sessionStore.LoadSession(sessionID)
			if err == nil && juggleSession.Description != "" {
				buf.WriteString(fmt.Sprintf("Session: %s\n", sessionID))
				buf.WriteString(fmt.Sprintf("Description: %s\n", juggleSession.Description))
				if juggleSession.Context != "" {
					buf.WriteString("\n")
					buf.WriteString(juggleSession.Context)
					if !strings.HasSuffix(juggleSession.Context, "\n") {
						buf.WriteString("\n")
					}
				}
			}
		}
	}
	buf.WriteString("</context>\n\n")

	// Write balls section with all details
	buf.WriteString("<balls>\n")
	for i, ball := range balls {
		if i > 0 {
			buf.WriteString("\n")
		}
		writeBallForRefine(&buf, ball)
	}
	buf.WriteString("</balls>\n\n")

	// Write instructions section with refinement template
	buf.WriteString("<instructions>\n")
	buf.WriteString(agent.GetRefinePromptTemplate())
	if !strings.HasSuffix(agent.GetRefinePromptTemplate(), "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString("</instructions>\n")

	// Append user message if provided
	if message != "" {
		buf.WriteString("\n<user-message>\n")
		buf.WriteString(message)
		buf.WriteString("\n</user-message>\n")
	}

	return buf.String(), nil
}

// LoadBallsForRefineForTest is an exported wrapper for testing
func LoadBallsForRefineForTest(projectDir, sessionID string) ([]*session.Ball, error) {
	return loadBallsForRefine(projectDir, sessionID)
}

// GenerateRefinePromptForTest is an exported wrapper for testing
func GenerateRefinePromptForTest(projectDir, sessionID string, balls []*session.Ball) (string, error) {
	return generateRefinePrompt(projectDir, sessionID, balls, "")
}

// GenerateRefinePromptWithMessageForTest is an exported wrapper for testing with a message
func GenerateRefinePromptWithMessageForTest(projectDir, sessionID string, balls []*session.Ball, message string) (string, error) {
	return generateRefinePrompt(projectDir, sessionID, balls, message)
}

// RunAgentRefineForTest runs the agent refine logic for testing
func RunAgentRefineForTest(projectDir, sessionID string) error {
	// Override GlobalOpts.ProjectDir for test
	oldProjectDir := GlobalOpts.ProjectDir
	GlobalOpts.ProjectDir = projectDir
	defer func() { GlobalOpts.ProjectDir = oldProjectDir }()

	// Load balls based on scope
	balls, err := loadBallsForRefine(projectDir, sessionID)
	if err != nil {
		return fmt.Errorf("failed to load balls: %w", err)
	}

	if len(balls) == 0 {
		return nil // No balls to refine
	}

	// Generate the refinement prompt
	prompt, err := generateRefinePrompt(projectDir, sessionID, balls, "")
	if err != nil {
		return fmt.Errorf("failed to generate prompt: %w", err)
	}

	// Run Claude in interactive + plan mode
	opts := agent.RunOptions{
		Prompt:     prompt,
		Mode:       agent.ModeInteractive,
		Permission: agent.PermissionPlan,
	}

	_, err = agent.DefaultRunner.Run(opts)
	return err
}

// GenerateAgentPromptForTest is an exported wrapper for testing prompt generation
func GenerateAgentPromptForTest(projectDir, sessionID string, debug bool, ballID string) (string, error) {
	return generateAgentPrompt(projectDir, sessionID, debug, ballID, "")
}

// GenerateAgentPromptWithMessageForTest is an exported wrapper for testing prompt generation with a message
func GenerateAgentPromptWithMessageForTest(projectDir, sessionID string, debug bool, ballID string, message string) (string, error) {
	return generateAgentPrompt(projectDir, sessionID, debug, ballID, message)
}

// writeBallForRefine writes a single ball with all details for refinement
func writeBallForRefine(buf *strings.Builder, ball *session.Ball) {
	// Header with ID, state, and priority
	buf.WriteString(fmt.Sprintf("## %s [%s] (priority: %s)\n", ball.ID, ball.State, ball.Priority))

	// Title
	buf.WriteString(fmt.Sprintf("Title: %s\n", ball.Title))

	// Context (full task description if different from title)
	if ball.Context != "" && ball.Context != ball.Title {
		buf.WriteString(fmt.Sprintf("Context: %s\n", ball.Context))
	}

	// Project directory
	if ball.WorkingDir != "" {
		buf.WriteString(fmt.Sprintf("Project: %s\n", ball.WorkingDir))
	}

	// Acceptance criteria
	if len(ball.AcceptanceCriteria) > 0 {
		buf.WriteString("Acceptance Criteria:\n")
		for i, ac := range ball.AcceptanceCriteria {
			buf.WriteString(fmt.Sprintf("  %d. %s\n", i+1, ac))
		}
	} else {
		buf.WriteString("Acceptance Criteria: (none - needs definition)\n")
	}

	// Blocked reason if blocked
	if ball.State == session.StateBlocked && ball.BlockedReason != "" {
		buf.WriteString(fmt.Sprintf("Blocked: %s\n", ball.BlockedReason))
	}

	// Tags
	if len(ball.Tags) > 0 {
		buf.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(ball.Tags, ", ")))
	}

	// Output if researched
	if ball.Output != "" {
		buf.WriteString(fmt.Sprintf("Research Output: %s\n", ball.Output))
	}
}

// ModelSelection contains model selection results
type ModelSelection struct {
	Model      string   // Model to use for this iteration (opus, sonnet, haiku)
	Reason     string   // Why this model was selected
	BallsCount int      // Number of balls that prefer this model
}

// selectModelForIteration analyzes remaining balls and chooses the optimal model.
// Priority order:
// 1. If config.Model is explicitly set (via --model flag), use it
// 2. If working on a single ball with ModelOverride set, use that override
// 3. Use session.DefaultModel if available
// 4. Choose based on ball model preferences (prioritize matching balls)
// 5. Default to "opus" (largest/most capable model)
//
// The function returns the model to use and reason for selection.
func selectModelForIteration(config AgentLoopConfig, balls []*session.Ball, defaultSessionModel session.ModelSize) *ModelSelection {
	// If model explicitly provided via --model flag, use it
	if config.Model != "" {
		return &ModelSelection{
			Model:  config.Model,
			Reason: "explicitly set via --model flag",
		}
	}

	// Filter to non-terminal balls only
	activeBalls := filterActiveBalls(balls)
	if len(activeBalls) == 0 {
		return &ModelSelection{
			Model:  "opus",
			Reason: "no active balls",
		}
	}

	// If working on a single ball with ModelOverride, use that
	if len(activeBalls) == 1 && activeBalls[0].ModelOverride != "" {
		return &ModelSelection{
			Model:      activeBalls[0].ModelOverride,
			Reason:     fmt.Sprintf("ball %s has model_override set", activeBalls[0].ShortID()),
			BallsCount: 1,
		}
	}

	// Count balls by model preference
	modelCounts := countBallsByModel(activeBalls)

	// If session has a default model and there are balls without explicit preference,
	// count those as preferring the session default
	if defaultSessionModel != "" && defaultSessionModel != session.ModelSizeBlank {
		blankCount := modelCounts[""]
		sessionModel := mapModelSizeToString(defaultSessionModel)
		modelCounts[sessionModel] += blankCount
		delete(modelCounts, "")
	}

	// Find the model with most balls (prefer larger models on tie)
	selectedModel := "opus"
	maxCount := 0
	selectedReason := "default (no model preferences specified)"

	// Check in order of preference (larger models first for ties)
	modelPriority := []string{"opus", "sonnet", "haiku"}
	for _, model := range modelPriority {
		count := modelCounts[model]
		if count > maxCount {
			maxCount = count
			selectedModel = model
			selectedReason = fmt.Sprintf("%d ball(s) prefer %s model", count, model)
		}
	}

	// If only blank preferences and no session default, use opus
	if maxCount == 0 {
		if defaultSessionModel != "" && defaultSessionModel != session.ModelSizeBlank {
			selectedModel = mapModelSizeToString(defaultSessionModel)
			selectedReason = "session default model"
		} else {
			selectedModel = "opus"
			selectedReason = "default (no preferences)"
		}
	}

	return &ModelSelection{
		Model:      selectedModel,
		Reason:     selectedReason,
		BallsCount: maxCount,
	}
}

// filterActiveBalls returns only balls that are not in terminal state (complete/researched)
func filterActiveBalls(balls []*session.Ball) []*session.Ball {
	active := make([]*session.Ball, 0)
	for _, ball := range balls {
		if ball.State != session.StateComplete && ball.State != session.StateResearched {
			active = append(active, ball)
		}
	}
	return active
}

// countBallsByModel counts how many balls prefer each model size
func countBallsByModel(balls []*session.Ball) map[string]int {
	counts := make(map[string]int)
	for _, ball := range balls {
		model := mapModelSizeToString(ball.ModelSize)
		counts[model]++
	}
	return counts
}

// mapModelSizeToString converts ModelSize to the string used by Claude CLI
func mapModelSizeToString(size session.ModelSize) string {
	switch size {
	case session.ModelSizeSmall:
		return "haiku"
	case session.ModelSizeMedium:
		return "sonnet"
	case session.ModelSizeLarge:
		return "opus"
	default:
		return "" // blank/unset
	}
}

// prioritizeBallsByModel sorts balls so those matching the current model come first.
// This is called after sortBallsForAgent to further prioritize by model match.
// Within model-matched balls, the existing sort order (state, deps, priority) is preserved.
func prioritizeBallsByModel(balls []*session.Ball, currentModel string, sessionDefaultModel session.ModelSize) {
	if currentModel == "" {
		return // No prioritization needed if no model set
	}

	// Determine which ModelSize values match the current model
	matchesModel := func(ball *session.Ball) bool {
		ballModel := ball.ModelSize
		// If ball has no preference, use session default
		if ballModel == "" || ballModel == session.ModelSizeBlank {
			ballModel = sessionDefaultModel
		}
		// Convert to string and compare
		return mapModelSizeToString(ballModel) == currentModel || ballModel == session.ModelSizeBlank
	}

	// Stable sort: matching balls first, preserving relative order within each group
	sort.SliceStable(balls, func(i, j int) bool {
		matchI := matchesModel(balls[i])
		matchJ := matchesModel(balls[j])
		// Only reorder if one matches and other doesn't
		if matchI && !matchJ {
			return true
		}
		return false
	})
}

// checkClaudeSettings checks if Claude Code is configured for optimal headless execution.
// Returns a list of status messages (prefixed with checkmark or X).
func checkClaudeSettings() []string {
	var issues []string

	cwd, err := GetWorkingDir()
	if err != nil {
		return issues // Don't block on errors
	}

	settingsPath := filepath.Join(cwd, ".claude", "settings.json")

	// Check if settings file exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		issues = append(issues, "✗ Settings file missing (.claude/settings.json)")
		return issues
	}

	// Load and check settings
	settings, err := LoadClaudeSettings(settingsPath)
	if err != nil {
		issues = append(issues, "✗ Settings file invalid")
		return issues
	}

	// Check sandbox enabled
	sandbox := settings.GetSandboxConfig()
	if sandbox == nil || !sandbox.Enabled {
		issues = append(issues, "✗ Sandbox mode not enabled")
	} else {
		issues = append(issues, "✓ Sandbox mode enabled")
	}

	// Check hooks
	if AreHooksInstalled() {
		issues = append(issues, "✓ Hooks installed")
	} else {
		issues = append(issues, "✗ Hooks not installed")
	}

	return issues
}

// shouldSkipClaudeSetupCheck checks if the user has chosen to skip the setup check.
func shouldSkipClaudeSetupCheck() bool {
	cwd, err := GetWorkingDir()
	if err != nil {
		return false
	}

	configPath := filepath.Join(cwd, ".juggle", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}

	skip, ok := config["skip_claude_setup_check"].(bool)
	return ok && skip
}

// saveSkipClaudeSetupCheck saves the preference to skip the setup check.
func saveSkipClaudeSetupCheck() error {
	cwd, err := GetWorkingDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(cwd, ".juggle", "config.json")

	// Load existing config or create new
	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &config)
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	config["skip_claude_setup_check"] = true

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, newData, 0644)
}

// promptSetupOrSkip prompts the user with Y/n/never options.
// Returns "y", "n", or "never".
func promptSetupOrSkip() string {
	fmt.Print("Run setup now? [Y/n/never]: ")

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "n"
	}
	defer term.Restore(fd, oldState)

	b := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(b)
		if err != nil {
			return "n"
		}

		key := b[0]

		// Ctrl+C
		if key == 3 {
			fmt.Println("\n^C")
			return "n"
		}

		// Enter or Y/y = yes
		if key == '\r' || key == '\n' || key == 'y' || key == 'Y' {
			fmt.Println("y")
			return "y"
		}

		// N/n = no
		if key == 'n' || key == 'N' {
			fmt.Println("n")
			return "n"
		}

		// Never (accept 'e' for "never")
		if key == 'e' || key == 'E' {
			fmt.Println("never")
			return "never"
		}
	}
}

func runAgentSetupRepo(cmd *cobra.Command, args []string) error {
	cwd, err := GetWorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Determine the provider
	globalProvider, err := session.GetGlobalAgentProviderWithOptions(GetConfigOptions())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load global agent provider config: %v\n", err)
	}
	projectProvider, err := session.GetProjectAgentProvider(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load project agent provider config: %v\n", err)
	}
	providerType := provider.Detect("", projectProvider, globalProvider)

	// Verify provider binary is available
	if !provider.IsAvailable(providerType) {
		return fmt.Errorf("agent provider %q is not available (binary %q not found in PATH)",
			providerType, provider.BinaryName(providerType))
	}

	agentProv := provider.Get(providerType)
	agent.SetProvider(agentProv)

	prompt := "Use the /sandbox-setup skill to configure this repository for autonomous agent operations. Analyze the codebase, ask me questions about my workflow, and update .claude/settings.json with appropriate permissions."

	fmt.Println("Launching interactive sandbox configuration...")
	fmt.Println("")

	opts := agent.RunOptions{
		Prompt:     prompt,
		Mode:       agent.ModeInteractive,
		Permission: agent.PermissionAcceptEdits,
		WorkingDir: cwd,
	}

	_, err = agent.DefaultRunner.Run(opts)
	if err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	return nil
}

// SelectModelForIterationForTest is an exported wrapper for testing
func SelectModelForIterationForTest(config AgentLoopConfig, balls []*session.Ball, defaultSessionModel session.ModelSize) *ModelSelection {
	return selectModelForIteration(config, balls, defaultSessionModel)
}

// PrioritizeBallsByModelForTest is an exported wrapper for testing
func PrioritizeBallsByModelForTest(balls []*session.Ball, currentModel string, sessionDefaultModel session.ModelSize) {
	prioritizeBallsByModel(balls, currentModel, sessionDefaultModel)
}

// FilterActiveBallsForTest is an exported wrapper for testing
func FilterActiveBallsForTest(balls []*session.Ball) []*session.Ball {
	return filterActiveBalls(balls)
}

// CountBallsByModelForTest is an exported wrapper for testing
func CountBallsByModelForTest(balls []*session.Ball) map[string]int {
	return countBallsByModel(balls)
}

// loadBallsForModelSelection loads balls for model selection purposes.
// This is similar to generateAgentPrompt but returns the balls instead of generating a prompt.
func loadBallsForModelSelection(projectDir, sessionID, ballID string) ([]*session.Ball, error) {
	// Load config to discover projects
	config, err := LoadConfigForCommand()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create store for current directory
	store, err := NewStoreForCommand(projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Discover projects
	projects, err := DiscoverProjectsForCommand(config, store)
	if err != nil {
		return nil, fmt.Errorf("failed to discover projects: %w", err)
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects with .juggle directories found")
	}

	// Load all balls from discovered projects
	allBalls, err := session.LoadAllBalls(projects)
	if err != nil {
		return nil, fmt.Errorf("failed to load balls: %w", err)
	}

	// Filter by session tag
	// "all" is a special meta-session that means "all balls in repo" (no session filtering)
	var balls []*session.Ball
	if sessionID == "all" {
		balls = allBalls
	} else {
		balls = make([]*session.Ball, 0)
		for _, ball := range allBalls {
			for _, tag := range ball.Tags {
				if tag == sessionID {
					balls = append(balls, ball)
					break
				}
			}
		}
	}

	// Filter out complete and blocked balls by default (they clutter the context for no gain)
	// Exception: when a specific ball is requested, allow it even if complete/blocked
	if ballID == "" {
		filteredBalls := make([]*session.Ball, 0, len(balls))
		for _, ball := range balls {
			if ball.State != session.StateComplete && ball.State != session.StateResearched && ball.State != session.StateBlocked {
				filteredBalls = append(filteredBalls, ball)
			}
		}
		balls = filteredBalls
	}

	// Filter to specific ball if ballID is specified
	if ballID != "" {
		matches := session.ResolveBallByPrefix(balls, ballID)
		if len(matches) == 0 {
			return nil, fmt.Errorf("ball %s not found in session %s", ballID, sessionID)
		}
		if len(matches) > 1 {
			matchingIDs := make([]string, len(matches))
			for i, m := range matches {
				matchingIDs[i] = m.ID
			}
			return nil, fmt.Errorf("ambiguous ID '%s' matches %d balls: %s", ballID, len(matches), strings.Join(matchingIDs, ", "))
		}
		return []*session.Ball{matches[0]}, nil
	}

	return balls, nil
}

// LoadBallsForModelSelectionForTest is an exported wrapper for testing
func LoadBallsForModelSelectionForTest(projectDir, sessionID, ballID string) ([]*session.Ball, error) {
	return loadBallsForModelSelection(projectDir, sessionID, ballID)
}

// CommitResult represents the outcome of a VCS commit operation
type CommitResult struct {
	Success       bool   // Whether the commit succeeded
	CommitHash    string // Short hash of the new commit (if successful)
	StatusOutput  string // Output from status after commit
	ErrorMessage  string // Error message if commit failed
}

// performVCSCommit executes a commit using the configured VCS backend.
// This is called by juggle after the agent signals completion.
// Returns nil if there are no changes to commit.
func performVCSCommit(projectDir, commitMessage string) (*CommitResult, error) {
	// Load VCS settings
	globalVCS, _ := session.GetGlobalVCSWithOptions(GetConfigOptions())
	projectVCS, _ := session.GetProjectVCS(projectDir)

	// Get the appropriate backend
	backend := vcs.GetBackendForProject(projectDir, vcs.VCSType(projectVCS), vcs.VCSType(globalVCS))

	// Perform commit
	vcsResult, err := backend.Commit(projectDir, commitMessage)
	if err != nil {
		return nil, err
	}

	// Convert to our CommitResult type
	return &CommitResult{
		Success:      vcsResult.Success,
		CommitHash:   vcsResult.CommitHash,
		StatusOutput: vcsResult.StatusOutput,
		ErrorMessage: vcsResult.ErrorMessage,
	}, nil
}

// performJJCommit is kept for backward compatibility - delegates to performVCSCommit
func performJJCommit(projectDir, commitMessage string) (*CommitResult, error) {
	return performVCSCommit(projectDir, commitMessage)
}

// PerformJJCommitForTest is an exported wrapper for testing
func PerformJJCommitForTest(projectDir, commitMessage string) (*CommitResult, error) {
	return performVCSCommit(projectDir, commitMessage)
}
