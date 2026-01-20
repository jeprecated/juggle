package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ohare93/juggle/internal/agent/daemon"
	"github.com/ohare93/juggle/internal/session"
	"github.com/spf13/cobra"
)

var loopUpdateJSONFlag bool

var loopCmd = &cobra.Command{
	Use:   "loop",
	Short: "Manage agent loop status and progress",
	Long:  `Commands for managing agent loop status updates (agent-update.txt files).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var loopUpdateCmd = &cobra.Command{
	Use:   "update [session-id] <ball-id> <state> <message>",
	Short: "Update agent loop progress for a ball",
	Long: `Update the agent loop's progress tracking for a specific ball.

The session-id can be provided as the first argument, or via the
JUGGLE_SESSION_ID environment variable.

State should be one of: starting, working, blocked, testing, complete

Creates or overwrites agent-update.txt with the current status.

Examples:
  juggle loop update my-session juggle-123 starting "Beginning work on authentication"
  JUGGLE_SESSION_ID=my-session juggle loop update juggle-123 working "Implementing OAuth flow"
  juggle loop update my-session juggle-123 blocked "Waiting for API credentials"
  juggle loop update my-session juggle-123 testing "Running test suite"
  juggle loop update my-session juggle-123 complete "All ACs satisfied"`,
	Args: cobra.RangeArgs(3, 4),
	RunE: runLoopUpdate,
}

var loopHookEventCmd = &cobra.Command{
	Use:   "hook-event <event-type>",
	Short: "Receive Claude Code hook events and update metrics",
	Long: `Process Claude Code hook events and update session metrics.

This command is designed to be called by Claude Code hooks. It reads
JSON data from stdin and updates the session's agent-metrics.json file.

The session ID must be set via the JUGGLE_SESSION_ID environment variable.
If not set, the command exits silently (not a juggler-managed session).

Event types:
  post-tool     - After a tool executes successfully (tracks file changes, tool counts)
  tool-failure  - After a tool fails (tracks failure count)
  stop          - When Claude finishes a response (tracks turns, token usage)
  session-end   - When the Claude session ends (marks session as ended)

The hook reads JSON from stdin with structure depending on the event type:
  post-tool:    {"tool_name": "Write", "tool_input": {"file_path": "...", "command": "..."}}
  stop:         {"usage": {"input_tokens": N, "output_tokens": N, "cache_read_input_tokens": N}}
  session-end:  (any JSON, just signals end)

Examples:
  # Called by Claude Code hook (receives JSON on stdin)
  echo '{"tool_name":"Write","tool_input":{"file_path":"foo.go"}}' | juggle loop hook-event post-tool`,
	Args: cobra.ExactArgs(1),
	RunE: runLoopHookEvent,
}

func init() {
	loopUpdateCmd.Flags().BoolVar(&loopUpdateJSONFlag, "json", false, "Output as JSON")
	loopCmd.AddCommand(loopUpdateCmd)
	loopCmd.AddCommand(loopHookEventCmd)
	rootCmd.AddCommand(loopCmd)
}

func runLoopUpdate(cmd *cobra.Command, args []string) error {
	var sessionID, ballID, state, message string

	// Parse args: either (session-id, ball-id, state, message) or (ball-id, state, message) with env var
	if len(args) == 4 {
		sessionID = args[0]
		ballID = args[1]
		state = args[2]
		message = args[3]
	} else {
		// Three args - use env var for session ID
		sessionID = os.Getenv("JUGGLE_SESSION_ID")
		if sessionID == "" {
			err := fmt.Errorf("session ID required: provide as first argument or set JUGGLE_SESSION_ID")
			if loopUpdateJSONFlag {
				return printLoopUpdateJSONError(err)
			}
			return err
		}
		ballID = args[0]
		state = args[1]
		message = args[2]
	}

	// Validate state
	validStates := map[string]bool{
		"starting": true,
		"working":  true,
		"blocked":  true,
		"testing":  true,
		"complete": true,
	}
	if !validStates[state] {
		err := fmt.Errorf("invalid state '%s': must be one of starting, working, blocked, testing, complete", state)
		if loopUpdateJSONFlag {
			return printLoopUpdateJSONError(err)
		}
		return err
	}

	cwd, err := GetWorkingDir()
	if err != nil {
		err = fmt.Errorf("failed to get current directory: %w", err)
		if loopUpdateJSONFlag {
			return printLoopUpdateJSONError(err)
		}
		return err
	}

	store, err := session.NewSessionStoreWithConfig(cwd, GetStoreConfig())
	if err != nil {
		err = fmt.Errorf("failed to initialize session store: %w", err)
		if loopUpdateJSONFlag {
			return printLoopUpdateJSONError(err)
		}
		return err
	}

	// Format timestamped entry
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("[%s] ball=%s state=%s message=%s\n", timestamp, ballID, state, message)

	// Map "all" meta-session to "_all" for storage
	storageID := sessionID
	if sessionID == "all" {
		storageID = "_all"
	}

	// Write to agent update file (overwrites existing content)
	if err := store.WriteAgentUpdate(storageID, entry); err != nil {
		err = fmt.Errorf("failed to write agent update: %w", err)
		if loopUpdateJSONFlag {
			return printLoopUpdateJSONError(err)
		}
		return err
	}

	// Append to updates history for the main view display
	// Get iteration from daemon state if available
	iteration := 0
	if daemonState, err := daemon.ReadStateFile(cwd, storageID); err == nil && daemonState != nil {
		iteration = daemonState.Iteration
	}
	update := session.NewPhaseUpdate(ballID, state, message, iteration)
	_ = store.AppendUpdate(storageID, update) // Ignore errors to not disrupt agent

	if loopUpdateJSONFlag {
		return printLoopUpdateJSONSuccess(sessionID, ballID, state, message, timestamp)
	}

	// Success message for agent confirmation
	fmt.Printf("Updated agent status for session %s\n", sessionID)
	return nil
}

// LoopUpdateResponse is the JSON response for loop update command
type LoopUpdateResponse struct {
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	BallID    string `json:"ball_id"`
	State     string `json:"state"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func printLoopUpdateJSONSuccess(sessionID, ballID, state, message, timestamp string) error {
	resp := LoopUpdateResponse{
		Success:   true,
		SessionID: sessionID,
		BallID:    ballID,
		State:     state,
		Message:   message,
		Timestamp: timestamp,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
	return nil
}

func printLoopUpdateJSONError(err error) error {
	errResp := map[string]string{"error": err.Error()}
	data, _ := json.Marshal(errResp)
	fmt.Println(string(data))
	return nil // Return nil so the error is in JSON, not stderr
}

// runLoopHookEvent processes Claude Code hook events and updates session metrics
func runLoopHookEvent(cmd *cobra.Command, args []string) error {
	eventType := args[0]

	// Get session ID from environment - exit silently if not set
	sessionID := os.Getenv("JUGGLE_SESSION_ID")
	if sessionID == "" {
		// Not a juggler-managed session, exit silently
		return nil
	}

	// Read JSON from stdin
	inputData, err := io.ReadAll(os.Stdin)
	if err != nil {
		// Hooks should not block Claude, fail silently
		return nil
	}

	cwd, err := GetWorkingDir()
	if err != nil {
		return nil // Fail silently
	}

	store, err := session.NewSessionStoreWithConfig(cwd, GetStoreConfig())
	if err != nil {
		return nil // Fail silently
	}

	// Map "all" meta-session to "_all" for storage
	storageID := sessionID
	if sessionID == "all" {
		storageID = "_all"
	}

	// Process based on event type
	switch eventType {
	case "post-tool":
		return handlePostToolEvent(store, storageID, inputData)
	case "tool-failure":
		return handleToolFailureEvent(store, storageID, inputData)
	case "stop":
		return handleStopEvent(store, storageID, inputData)
	case "session-end":
		return handleSessionEndEvent(store, storageID)
	default:
		// Unknown event type, ignore silently
		return nil
	}
}

// PostToolPayload represents the JSON structure from PostToolUse hooks
type PostToolPayload struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
		Command  string `json:"command"`
	} `json:"tool_input"`
}

// StopPayload represents the JSON structure from Stop hooks
type StopPayload struct {
	Usage struct {
		InputTokens          int `json:"input_tokens"`
		OutputTokens         int `json:"output_tokens"`
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func handlePostToolEvent(store *session.SessionStore, sessionID string, inputData []byte) error {
	var payload PostToolPayload
	if err := json.Unmarshal(inputData, &payload); err != nil {
		return nil // Invalid JSON, fail silently
	}

	// Determine the file path from tool input
	filePath := payload.ToolInput.FilePath

	// Append to updates history (ignore errors to not block agent)
	update := session.NewToolUseUpdate(payload.ToolName, filePath)
	_ = store.AppendUpdate(sessionID, update)

	return store.UpdateMetricsFromPostTool(sessionID, payload.ToolName, filePath)
}

func handleToolFailureEvent(store *session.SessionStore, sessionID string, inputData []byte) error {
	var payload PostToolPayload
	if err := json.Unmarshal(inputData, &payload); err != nil {
		return nil // Invalid JSON, fail silently
	}

	// Append to updates history (ignore errors to not block agent)
	update := session.NewToolFailureUpdate(payload.ToolName)
	_ = store.AppendUpdate(sessionID, update)

	return store.UpdateMetricsFromToolFailure(sessionID, payload.ToolName)
}

func handleStopEvent(store *session.SessionStore, sessionID string, inputData []byte) error {
	var payload StopPayload
	if err := json.Unmarshal(inputData, &payload); err != nil {
		return nil // Invalid JSON, fail silently
	}

	// Append to updates history (ignore errors to not block agent)
	update := session.NewStopUpdate(
		payload.Usage.InputTokens,
		payload.Usage.OutputTokens,
		payload.Usage.CacheReadInputTokens,
	)
	_ = store.AppendUpdate(sessionID, update)

	return store.UpdateMetricsFromStop(
		sessionID,
		payload.Usage.InputTokens,
		payload.Usage.OutputTokens,
		payload.Usage.CacheReadInputTokens,
	)
}

func handleSessionEndEvent(store *session.SessionStore, sessionID string) error {
	// Append to updates history (ignore errors to not block agent)
	update := session.NewSessionEndUpdate()
	_ = store.AppendUpdate(sessionID, update)

	return store.UpdateMetricsFromSessionEnd(sessionID)
}
