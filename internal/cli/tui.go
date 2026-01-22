package cli

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ohare93/juggle/internal/session"
	"github.com/ohare93/juggle/internal/tui"
	"github.com/ohare93/juggle/internal/watcher"
	"github.com/spf13/cobra"
)

var tuiSessionFilter string

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal UI",
	Long: `Launch an interactive terminal user interface for managing balls.

The TUI provides a full-screen split-view interface with three panels:
sessions, balls, and todos. Use keyboard navigation for quick actions.

Use --all flag to show balls from all discovered projects:
  juggle --all tui

Use --session to start with a session pre-selected:
  juggle tui --session my-feature

Navigation:
  Tab/h/l    Switch between panels (sessions → balls → todos)
  ↑/k        Move up within panel
  ↓/j        Move down within panel
  Enter      Select item / expand
  Esc        Go back / deselect

CRUD Operations:
  a          Add new item (session/ball/todo based on panel)
  e          Edit selected item
  d          Delete selected item (with confirmation)
  t          Edit tags for selected ball
  Space      Toggle todo completion

State Management:
  s          Start ball (→ in_progress)
  c          Complete ball (→ complete, archives)
  b          Block ball (prompts for reason)

Search/Filter:
  /          Open search/filter for current panel
  Ctrl+U     Clear current filter

Filters (toggleable):
  1          Show all states
  2          Toggle pending visibility
  3          Toggle in_progress visibility
  4          Toggle blocked visibility
  5          Toggle complete visibility

Other:
  R          Refresh/reload (shift+r)
  ?          Show help
  q          Quit`,
	RunE: runTUI,
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Get working directory
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Load config
	config, err := session.LoadConfig()
	if err != nil {
		return err
	}

	// Create store
	store, err := session.NewStore(workingDir)
	if err != nil {
		return err
	}

	// Initialize split view with file watcher
	sessionStore, err := session.NewSessionStore(workingDir)
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
	if err := w.WatchProject(workingDir); err != nil {
		// Non-fatal: continue without watching if it fails
		w = nil
	} else {
		// Start the watcher
		w.Start()
	}

	model := tui.InitialSplitModelWithWatcher(store, sessionStore, config, !GlobalOpts.AllProjects, w, tuiSessionFilter)

	// Create program with alternate screen
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Check if user requested to run agent after TUI exit
	if tuiModel, ok := finalModel.(tui.Model); ok {
		if ballID := tuiModel.RunAgentForBall(); ballID != "" {
			fmt.Printf("\nStarting agent for ball %s...\n", ballID)

			// Run the agent loop directly
			agentConfig := AgentLoopConfig{
				SessionID:     "all", // Use "all" meta-session since we're targeting a specific ball
				ProjectDir:    workingDir,
				MaxIterations: 1,     // Single iteration for "run now"
				BallID:        ballID,
				Interactive:   true,  // Interactive mode for user involvement
			}

			result, err := RunAgentLoop(agentConfig)
			if err != nil {
				return fmt.Errorf("agent error: %w", err)
			}

			// Offer to commit after agent finishes
			if err := offerCommitAfterRun(workingDir, store, ballID, result); err != nil {
				// Non-fatal - just log the error
				fmt.Fprintf(os.Stderr, "Warning: commit prompt failed: %v\n", err)
			}
		}
	}

	return nil
}

// offerCommitAfterRun prompts the user to commit changes after an agent run.
// It uses the agent's commit message if available, otherwise falls back to "feat: <ballID>: <title>".
// If the agent reports completion, it marks the ball complete and informs the user.
func offerCommitAfterRun(projectDir string, store *session.Store, ballID string, result *AgentResult) error {
	const maxTitleLength = 50

	// Get the ball for title/context and potential completion
	ball, err := store.GetBallByID(ballID)
	if err != nil {
		// Can't get ball - still try to offer commit but skip completion
		fmt.Fprintf(os.Stderr, "Warning: could not load ball %s: %v\n", ballID, err)
	}

	// Mark ball as complete if the agent reports completion
	if result != nil && result.Complete && ball != nil {
		ball.MarkComplete("Completed via Run Now")
		if err := store.UpdateBall(ball); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update ball: %v\n", err)
		} else {
			fmt.Printf("✅ Ball %s marked complete\n", ballID)
		}
	}

	// Construct commit message
	var commitMsg string

	if result != nil && result.CommitMessage != "" {
		// Use agent-provided commit message (sanitized)
		commitMsg = strings.TrimSpace(result.CommitMessage)
	} else {
		// Fall back to "feat: <ballID>: <title>"
		if ball == nil {
			// Can't get ball title, use just the ball ID
			commitMsg = fmt.Sprintf("feat: %s", ballID)
		} else {
			title := ball.Title
			if title == "" {
				title = ball.Context
				// Truncate context if too long (use rune counting for Unicode safety)
				runes := []rune(title)
				if len(runes) > maxTitleLength {
					title = string(runes[:maxTitleLength-3]) + "..."
				}
			}
			if title != "" {
				commitMsg = fmt.Sprintf("feat: %s: %s", ballID, title)
			} else {
				commitMsg = fmt.Sprintf("feat: %s", ballID)
			}
		}
	}

	// Show commit message and prompt
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Println("Proposed commit message:")
	fmt.Printf("  %s\n", commitMsg)
	fmt.Println("─────────────────────────────────────────────────────")

	confirmed, err := ConfirmSingleKey("Commit changes?")
	if err != nil {
		// User cancelled or terminal error - skip commit
		fmt.Printf("Skipping commit: %v\n", err)
		return nil
	}

	if !confirmed {
		fmt.Println("Skipping commit.")
		return nil
	}

	// Perform the commit
	commitResult, err := performVCSCommit(projectDir, commitMsg)
	if err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	if commitResult.Success {
		if commitResult.CommitHash != "" {
			fmt.Printf("📝 Committed: %s\n", commitResult.CommitHash)
		}
		if commitResult.StatusOutput != "" && commitResult.StatusOutput != "No changes to commit" {
			fmt.Printf("📊 Status: %s\n", commitResult.StatusOutput)
		}
	} else if commitResult.ErrorMessage != "" {
		if commitResult.ErrorMessage == "no changes to commit" {
			fmt.Println("No changes to commit.")
		} else {
			fmt.Printf("⚠️  Commit failed: %s\n", commitResult.ErrorMessage)
		}
	}

	return nil
}

func init() {
	tuiCmd.Flags().StringVar(&tuiSessionFilter, "session", "", "Start with session pre-selected")
	rootCmd.AddCommand(tuiCmd)
}
