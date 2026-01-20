package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// EventType represents the type of file change event
type EventType int

const (
	BallsChanged EventType = iota
	ProgressChanged
	SessionChanged
	AgentStateChanged    // Daemon state file (agent.state) changed
	AgentUpdateChanged   // Agent loop update file (agent-update.txt) changed
	AgentMetricsChanged  // Hook metrics file (agent-metrics.json) changed
	UpdatesHistoryChanged // Updates history file (updates.jsonl) changed
)

// Event represents a file change event
type Event struct {
	Type      EventType
	Path      string
	SessionID string // For progress changes, the session ID
}

// Watcher watches for file changes in juggle directories
type Watcher struct {
	watcher *fsnotify.Watcher
	Events  chan Event
	Errors  chan error
	done    chan struct{}
	mu      sync.Mutex
	running bool
}

// New creates a new file watcher
func New() (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	return &Watcher{
		watcher: fsWatcher,
		Events:  make(chan Event, 100),
		Errors:  make(chan error, 10),
		done:    make(chan struct{}),
	}, nil
}

// WatchProject adds watchers for a project's juggle files
func (w *Watcher) WatchProject(projectDir string) error {
	juggleDir := filepath.Join(projectDir, ".juggle")

	// Check if .juggle directory exists
	if _, err := os.Stat(juggleDir); os.IsNotExist(err) {
		return fmt.Errorf("juggle directory does not exist: %s", juggleDir)
	}

	// Watch the .juggle directory for balls.jsonl changes
	if err := w.watcher.Add(juggleDir); err != nil {
		return fmt.Errorf("failed to watch juggle directory: %w", err)
	}

	// Watch sessions directory if it exists
	sessionsDir := filepath.Join(juggleDir, "sessions")
	if _, err := os.Stat(sessionsDir); err == nil {
		// Watch the sessions directory
		if err := w.watcher.Add(sessionsDir); err != nil {
			return fmt.Errorf("failed to watch sessions directory: %w", err)
		}

		// Watch each session directory for progress.txt
		entries, err := os.ReadDir(sessionsDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					sessionDir := filepath.Join(sessionsDir, entry.Name())
					if err := w.watcher.Add(sessionDir); err != nil {
						// Log but don't fail - session might be inaccessible
						continue
					}
				}
			}
		}
	}

	return nil
}

// Start begins watching for file changes
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.eventLoop()
}

// eventLoop processes file system events
func (w *Watcher) eventLoop() {
	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Filter for write and create events
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Determine event type based on path
			e := w.classifyEvent(event.Name)
			if e != nil {
				// Non-blocking send
				select {
				case w.Events <- *e:
				default:
					// Channel full, skip event
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Non-blocking error send
			select {
			case w.Errors <- err:
			default:
			}
		}
	}
}

// classifyEvent determines the event type based on the file path
func (w *Watcher) classifyEvent(path string) *Event {
	base := filepath.Base(path)

	// Check for balls.jsonl
	if base == "balls.jsonl" {
		return &Event{
			Type: BallsChanged,
			Path: path,
		}
	}

	// Check for progress.txt in a session directory
	if base == "progress.txt" {
		// Extract session ID from path: .../sessions/<session-id>/progress.txt
		dir := filepath.Dir(path)
		sessionID := filepath.Base(dir)
		if strings.Contains(path, "sessions") {
			return &Event{
				Type:      ProgressChanged,
				Path:      path,
				SessionID: sessionID,
			}
		}
	}

	// Check for session.json changes
	if base == "session.json" {
		dir := filepath.Dir(path)
		sessionID := filepath.Base(dir)
		if strings.Contains(path, "sessions") {
			return &Event{
				Type:      SessionChanged,
				Path:      path,
				SessionID: sessionID,
			}
		}
	}

	// Check for agent.state changes (daemon state file)
	if base == "agent.state" {
		dir := filepath.Dir(path)
		sessionID := filepath.Base(dir)
		if strings.Contains(path, "sessions") {
			return &Event{
				Type:      AgentStateChanged,
				Path:      path,
				SessionID: sessionID,
			}
		}
	}

	// Check for agent-update.txt changes (loop update file)
	if base == "agent-update.txt" {
		dir := filepath.Dir(path)
		sessionID := filepath.Base(dir)
		if strings.Contains(path, "sessions") {
			return &Event{
				Type:      AgentUpdateChanged,
				Path:      path,
				SessionID: sessionID,
			}
		}
	}

	// Check for agent-metrics.json changes (hook metrics file)
	if base == "agent-metrics.json" {
		dir := filepath.Dir(path)
		sessionID := filepath.Base(dir)
		if strings.Contains(path, "sessions") {
			return &Event{
				Type:      AgentMetricsChanged,
				Path:      path,
				SessionID: sessionID,
			}
		}
	}

	// Check for updates.jsonl changes (updates history file)
	if base == "updates.jsonl" {
		dir := filepath.Dir(path)
		sessionID := filepath.Base(dir)
		if strings.Contains(path, "sessions") {
			return &Event{
				Type:      UpdatesHistoryChanged,
				Path:      path,
				SessionID: sessionID,
			}
		}
	}

	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	close(w.done)
	w.running = false
	return w.watcher.Close()
}

// Close is an alias for Stop
func (w *Watcher) Close() error {
	return w.Stop()
}
