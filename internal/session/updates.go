package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

const (
	updatesFile = "updates.jsonl"
)

// UpdateType represents the category of an update entry
type UpdateType string

const (
	// Agent phase updates (from juggle loop update)
	UpdateTypePhase UpdateType = "phase"
	// Hook events
	UpdateTypeToolUse     UpdateType = "tool"
	UpdateTypeToolFailure UpdateType = "tool_failure"
	UpdateTypeStop        UpdateType = "stop"
	UpdateTypeSessionEnd  UpdateType = "session_end"
)

// UpdateEntry represents a single update in the session's history.
// These are stored in updates.jsonl and displayed in the TUI.
type UpdateEntry struct {
	Timestamp time.Time  `json:"timestamp"`
	Type      UpdateType `json:"type"`
	BallID    string     `json:"ball_id,omitempty"`    // Associated ball (if known)
	Iteration int        `json:"iteration,omitempty"`  // Agent iteration (if known)
	State     string     `json:"state,omitempty"`      // Phase state (starting/working/blocked/testing/complete)
	Message   string     `json:"message,omitempty"`    // Human-readable message
	ToolName  string     `json:"tool_name,omitempty"`  // Tool name for tool events
	FilePath  string     `json:"file_path,omitempty"`  // File path for Write/Edit events
	Tokens    int        `json:"tokens,omitempty"`     // Total tokens for stop events
}

// updatesFilePath returns the path to a session's updates file
func (s *SessionStore) updatesFilePath(id string) string {
	return filepath.Join(s.sessionPath(id), updatesFile)
}

// AppendUpdate appends an update entry to the session's updates.jsonl file
func (s *SessionStore) AppendUpdate(id string, entry *UpdateEntry) error {
	// Ensure session directory exists
	sessionDir := s.sessionPath(id)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	updatesPath := s.updatesFilePath(id)
	lockPath := updatesPath + ".lock"

	// Acquire file lock
	fileLock := flock.New(lockPath)
	if err := fileLock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer fileLock.Unlock()

	// Marshal entry to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal update entry: %w", err)
	}

	// Open file in append mode
	f, err := os.OpenFile(updatesPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open updates file: %w", err)
	}
	defer f.Close()

	// Write JSON line
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write update entry: %w", err)
	}

	return nil
}

// LoadUpdates reads all update entries from a session's updates.jsonl file
func (s *SessionStore) LoadUpdates(id string) ([]*UpdateEntry, error) {
	updatesPath := s.updatesFilePath(id)

	f, err := os.Open(updatesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*UpdateEntry{}, nil
		}
		return nil, fmt.Errorf("failed to open updates file: %w", err)
	}
	defer f.Close()

	var entries []*UpdateEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry UpdateEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			// Skip malformed entries
			continue
		}
		entries = append(entries, &entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read updates file: %w", err)
	}

	return entries, nil
}

// LoadRecentUpdates loads the most recent N update entries
func (s *SessionStore) LoadRecentUpdates(id string, limit int) ([]*UpdateEntry, error) {
	entries, err := s.LoadUpdates(id)
	if err != nil {
		return nil, err
	}

	if len(entries) <= limit {
		return entries, nil
	}

	// Return only the most recent entries
	return entries[len(entries)-limit:], nil
}

// ClearUpdates removes all update entries for a session
func (s *SessionStore) ClearUpdates(id string) error {
	updatesPath := s.updatesFilePath(id)
	lockPath := updatesPath + ".lock"

	// Check if file exists
	if _, err := os.Stat(updatesPath); os.IsNotExist(err) {
		return nil
	}

	// Acquire file lock
	fileLock := flock.New(lockPath)
	if err := fileLock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer fileLock.Unlock()

	// Truncate the file
	if err := os.WriteFile(updatesPath, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to clear updates file: %w", err)
	}

	return nil
}

// Helper methods for creating update entries

// NewPhaseUpdate creates an update entry for an agent phase change
func NewPhaseUpdate(ballID, state, message string, iteration int) *UpdateEntry {
	return &UpdateEntry{
		Timestamp: time.Now(),
		Type:      UpdateTypePhase,
		BallID:    ballID,
		State:     state,
		Message:   message,
		Iteration: iteration,
	}
}

// NewToolUseUpdate creates an update entry for a tool use event
func NewToolUseUpdate(toolName, filePath string) *UpdateEntry {
	msg := toolName
	if filePath != "" {
		msg = fmt.Sprintf("%s: %s", toolName, filePath)
	}
	return &UpdateEntry{
		Timestamp: time.Now(),
		Type:      UpdateTypeToolUse,
		ToolName:  toolName,
		FilePath:  filePath,
		Message:   msg,
	}
}

// NewToolFailureUpdate creates an update entry for a tool failure event
func NewToolFailureUpdate(toolName string) *UpdateEntry {
	return &UpdateEntry{
		Timestamp: time.Now(),
		Type:      UpdateTypeToolFailure,
		ToolName:  toolName,
		Message:   fmt.Sprintf("%s failed", toolName),
	}
}

// NewStopUpdate creates an update entry for a stop event (end of turn)
func NewStopUpdate(inputTokens, outputTokens, cacheReadTokens int) *UpdateEntry {
	totalTokens := inputTokens + outputTokens
	return &UpdateEntry{
		Timestamp: time.Now(),
		Type:      UpdateTypeStop,
		Tokens:    totalTokens,
		Message:   fmt.Sprintf("Turn complete (%d tokens)", totalTokens),
	}
}

// NewSessionEndUpdate creates an update entry for session end
func NewSessionEndUpdate() *UpdateEntry {
	return &UpdateEntry{
		Timestamp: time.Now(),
		Type:      UpdateTypeSessionEnd,
		Message:   "Session ended",
	}
}
