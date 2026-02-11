package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/ohare93/juggle/internal/agent/daemon"
	"github.com/ohare93/juggle/internal/session"
	"github.com/ohare93/juggle/internal/watcher"
)

type ballsLoadedMsg struct {
	balls []*session.Ball
	err   error
}

func loadBalls(store *session.Store, config *session.Config, localOnly bool) tea.Cmd {
	return func() tea.Msg {
		var balls []*session.Ball

		if localOnly {
			// Load only from current project
			localBalls, err := store.LoadBalls()
			if err != nil {
				return ballsLoadedMsg{err: err}
			}
			balls = localBalls
		} else {
			// Load from all discovered projects
			projects, err := session.DiscoverProjects(config)
			if err != nil {
				return ballsLoadedMsg{err: err}
			}

			balls, err = session.LoadAllBalls(projects)
			if err != nil {
				return ballsLoadedMsg{err: err}
			}
		}

		return ballsLoadedMsg{balls: balls}
	}
}

type ballUpdatedMsg struct {
	ball *session.Ball
	err  error
}

func updateBall(store *session.Store, ball *session.Ball) tea.Cmd {
	return func() tea.Msg {
		if err := store.UpdateBall(ball); err != nil {
			return ballUpdatedMsg{err: err}
		}
		return ballUpdatedMsg{ball: ball}
	}
}

type ballArchivedMsg struct {
	ball *session.Ball
	err  error
}

// updateAndArchiveBall updates the ball and then archives it
func updateAndArchiveBall(store *session.Store, ball *session.Ball) tea.Cmd {
	return func() tea.Msg {
		// First update the ball to persist state changes
		if err := store.UpdateBall(ball); err != nil {
			return ballArchivedMsg{err: err}
		}
		// Then archive it (moves from balls.jsonl to archive/balls.jsonl)
		if err := store.ArchiveBall(ball); err != nil {
			return ballArchivedMsg{err: err}
		}
		return ballArchivedMsg{ball: ball}
	}
}

// archiveBall archives a ball without updating it first (already in complete state)
func archiveBall(store *session.Store, ball *session.Ball) tea.Cmd {
	return func() tea.Msg {
		// Archive the ball (moves from balls.jsonl to archive/balls.jsonl)
		if err := store.ArchiveBall(ball); err != nil {
			return ballArchivedMsg{err: err}
		}
		return ballArchivedMsg{ball: ball}
	}
}

// Sessions loading for split view
type sessionsLoadedMsg struct {
	sessions []*session.JuggleSession
	err      error
}

func loadSessions(sessionStore *session.SessionStore, config *session.Config, localOnly bool) tea.Cmd {
	return func() tea.Msg {
		var sessions []*session.JuggleSession

		if localOnly {
			// Load only from current project
			if sessionStore == nil {
				return sessionsLoadedMsg{sessions: []*session.JuggleSession{}}
			}
			localSessions, err := sessionStore.ListSessions()
			if err != nil {
				return sessionsLoadedMsg{err: err}
			}
			sessions = localSessions
		} else {
			// Load from all discovered projects
			projects, err := session.DiscoverProjects(config)
			if err != nil {
				return sessionsLoadedMsg{err: err}
			}

			sessions, err = session.LoadAllSessions(projects)
			if err != nil {
				return sessionsLoadedMsg{err: err}
			}
		}

		return sessionsLoadedMsg{sessions: sessions}
	}
}

// Watcher event messages
type watcherEventMsg struct {
	event watcher.Event
}

type watcherErrorMsg struct {
	err error
}

// listenForWatcherEvents creates a command that listens for watcher events
func listenForWatcherEvents(w *watcher.Watcher) tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-w.Events:
			return watcherEventMsg{event: event}
		case err := <-w.Errors:
			return watcherErrorMsg{err: err}
		}
	}
}

// Agent-related messages
type agentStartedMsg struct {
	sessionID string
}

type agentIterationMsg struct {
	sessionID string
	iteration int
	maxIter   int
}

type agentFinishedMsg struct {
	sessionID     string
	complete      bool
	blocked       bool
	blockedReason string
	iterations    int
	ballsComplete int
	ballsTotal    int
	err           error
}

// agentOutputMsg is sent when agent produces output
type agentOutputMsg struct {
	line    string
	isError bool // true if this is stderr output
}

// agentStreamJSONMsg carries parsed stream-JSON events for real-time metric updates
type agentStreamJSONMsg struct {
	eventType    string // message_start, content_block_start, content_block_delta, etc.
	inputTokens  int    // Cumulative input tokens
	outputTokens int    // Cumulative output tokens
	cacheTokens  int    // Cumulative cache tokens
	activeTool   string // Current tool name (or empty)
	thinking     bool   // True if thinking block is active
}

// agentCancelledMsg is sent when the agent is cancelled by user
type agentCancelledMsg struct {
	sessionID string
}

// agentProcessStartedMsg is sent when agent process is started, providing reference for cancellation
type agentProcessStartedMsg struct {
	process   *AgentProcess
	sessionID string
}

// AgentStatus tracks the state of a running agent
type AgentStatus struct {
	Running          bool
	SessionID        string
	Iteration        int
	MaxIterations    int
	CurrentBallID    string
	CurrentBallTitle string
	ACsComplete      int
	ACsTotal         int
	Model            string
	Provider         string
	Status           string // Status message when stopped (e.g., "No workable balls", "Complete")
	Phase            string // Current agent phase (starting, working, blocked, testing, complete)
	PhaseMessage     string // Message describing current phase activity

	// Real-time stream-JSON metrics
	StreamJSONActive bool   // True if stream-json is enabled and being parsed
	LiveInputTokens  int    // Real-time input token count
	LiveOutputTokens int    // Real-time output token count
	LiveCacheTokens  int    // Real-time cache token count
	ActiveTool       string // Currently executing tool (e.g., "Read", "Edit")
	ThinkingActive   bool   // True when thinking block is active
}

// DaemonInfo stores information about a running daemon for a session
type DaemonInfo struct {
	SessionID  string
	ProjectDir string
	Running    bool
	Iteration  int
	MaxIter    int
}

// AgentProcess holds state for a running agent with output streaming
type AgentProcess struct {
	cmd        *exec.Cmd
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	outputCh   chan<- tea.Msg
	sessionID  string
	cancelled  atomic.Bool // Thread-safe cancellation flag
	cancel     context.CancelFunc
	wg         sync.WaitGroup // Tracks scanner goroutines
	waitOnce   sync.Once      // Ensures Wait() is only called once
	waitErr    error          // Stores the Wait() result
	waitDone   chan struct{}  // Signals when Wait() is complete
}

// Kill terminates the running agent process
func (p *AgentProcess) Kill() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	p.cancelled.Store(true)

	// Cancel context to signal scanner goroutines to stop
	if p.cancel != nil {
		p.cancel()
	}

	// Kill the process
	err := p.cmd.Process.Kill()

	// Wait for the process to exit (with timeout) - only if waitDone channel was created
	if p.waitDone != nil {
		select {
		case <-p.waitDone:
			// Process has exited
		case <-time.After(5 * time.Second):
			// Timeout waiting for exit - force continue
		}
	}

	// Wait for scanner goroutines to finish
	p.wg.Wait()

	return err
}

// IsCancelled returns true if the process was cancelled
func (p *AgentProcess) IsCancelled() bool {
	if p == nil {
		return false
	}
	return p.cancelled.Load()
}

// Wait waits for the process to complete and returns any error
// It is safe to call multiple times from multiple goroutines
func (p *AgentProcess) Wait() error {
	if p == nil || p.cmd == nil {
		return nil
	}
	// If waitDone channel wasn't created, we can't safely wait
	if p.waitDone == nil {
		return nil
	}
	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
		close(p.waitDone)
	})
	<-p.waitDone
	return p.waitErr
}

// launchAgentCmd creates a command that runs the agent for a session
func launchAgentCmd(sessionID string) tea.Cmd {
	return func() tea.Msg {
		// Launch "juggle agent run" as a subprocess
		// This allows the TUI to continue running while the agent works
		cmd := exec.Command("juggle", "agent", "run", sessionID)

		// Start the command in the background
		if err := cmd.Start(); err != nil {
			return agentFinishedMsg{
				sessionID: sessionID,
				err:       err,
			}
		}

		// Wait for the command to complete in this goroutine
		// The TUI will continue to be responsive because this runs
		// in a background goroutine (tea.Cmd runs async)
		if err := cmd.Wait(); err != nil {
			// Check if it was just a non-zero exit (common for blocked/incomplete)
			if _, ok := err.(*exec.ExitError); !ok {
				return agentFinishedMsg{
					sessionID: sessionID,
					err:       err,
				}
			}
		}

		// Agent finished - file watcher will pick up ball changes
		return agentFinishedMsg{
			sessionID: sessionID,
			complete:  true,
		}
	}
}

// launchAgentWithOutputCmd creates a command that runs the agent and streams output
// It returns the process reference via agentProcessStartedMsg for cancellation support
func launchAgentWithOutputCmd(sessionID string, outputCh chan<- tea.Msg) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("juggle", "agent", "run", sessionID)

		// Create pipes for stdout and stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return agentFinishedMsg{sessionID: sessionID, err: err}
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return agentFinishedMsg{sessionID: sessionID, err: err}
		}

		// Start the command
		if err := cmd.Start(); err != nil {
			return agentFinishedMsg{sessionID: sessionID, err: err}
		}

		// Create context for cancellation
		ctx, cancel := context.WithCancel(context.Background())

		// Create process reference for cancellation
		process := &AgentProcess{
			cmd:       cmd,
			stdout:    stdout,
			stderr:    stderr,
			outputCh:  outputCh,
			sessionID: sessionID,
			cancel:    cancel,
			waitDone:  make(chan struct{}),
		}

		// Stream stdout in a goroutine (tracked by WaitGroup)
		process.wg.Add(1)
		go func() {
			defer process.wg.Done()
			scanner := bufio.NewScanner(stdout)

			var sseMode bool
			var eventType string
			var dataLines []string

			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				default:
				}

				line := scanner.Text()

				// Detect SSE format
				if strings.HasPrefix(line, "event:") {
					sseMode = true
					eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
					dataLines = nil
					continue
				}

				if sseMode && strings.HasPrefix(line, "data:") {
					data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
					dataLines = append(dataLines, data)
					continue
				}

				if sseMode && line == "" {
					// End of SSE event - parse it
					if len(dataLines) > 0 {
						jsonData := strings.Join(dataLines, "\n")
						msg := parseStreamJSONEvent(eventType, jsonData)
						if msg != nil {
							select {
							case outputCh <- msg:
							case <-ctx.Done():
								return
							}
						}
					}
					sseMode = false
					eventType = ""
					dataLines = nil
					continue
				}

				if !sseMode {
					// Plain text output
					select {
					case outputCh <- agentOutputMsg{line: line, isError: false}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		// Stream stderr in a goroutine (tracked by WaitGroup)
		process.wg.Add(1)
		go func() {
			defer process.wg.Done()
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				default:
					// Non-blocking send to prevent blocking on cancelled processes
					select {
					case outputCh <- agentOutputMsg{line: scanner.Text(), isError: true}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		return agentProcessStartedMsg{process: process, sessionID: sessionID}
	}
}

// waitForAgentCmd waits for the agent process to complete
func waitForAgentCmd(process *AgentProcess) tea.Cmd {
	return func() tea.Msg {
		if process == nil || process.cmd == nil {
			return agentFinishedMsg{sessionID: "", complete: true}
		}

		// Wait for the command to finish using the thread-safe Wait method
		err := process.Wait()

		// Check if cancelled using the atomic flag
		if process.IsCancelled() {
			return agentCancelledMsg{sessionID: process.sessionID}
		}

		// Check for errors
		if err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				return agentFinishedMsg{sessionID: process.sessionID, err: err}
			}
		}

		return agentFinishedMsg{sessionID: process.sessionID, complete: true}
	}
}

// listenForAgentOutput returns a command that waits for an output message on the channel
func listenForAgentOutput(outputCh <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if outputCh == nil {
			return nil
		}
		select {
		case msg, ok := <-outputCh:
			if !ok {
				// Channel closed - agent has finished
				return nil
			}
			return msg
		case <-time.After(100 * time.Millisecond):
			// Return nil to keep the listener alive without blocking
			return nil
		}
	}
}

// historyLoadedMsg is sent when agent history has been loaded
type historyLoadedMsg struct {
	history []*session.AgentRunRecord
	err     error
}

// loadAgentHistory creates a command to load agent run history
func loadAgentHistory(projectDir string) tea.Cmd {
	return func() tea.Msg {
		historyStore, err := session.NewAgentHistoryStore(projectDir)
		if err != nil {
			return historyLoadedMsg{err: err}
		}

		// Load the 50 most recent runs
		records, err := historyStore.LoadRecentHistory(50)
		if err != nil {
			return historyLoadedMsg{err: err}
		}

		return historyLoadedMsg{history: records}
	}
}

// historyOutputLoadedMsg is sent when last_output.txt content is loaded
type historyOutputLoadedMsg struct {
	content string
	err     error
}

// loadHistoryOutput creates a command to load the output file for a history record
func loadHistoryOutput(outputFile string) tea.Cmd {
	return func() tea.Msg {
		if outputFile == "" {
			return historyOutputLoadedMsg{content: "(no output file)", err: nil}
		}

		data, err := readFile(outputFile)
		if err != nil {
			return historyOutputLoadedMsg{content: "", err: err}
		}

		return historyOutputLoadedMsg{content: string(data), err: nil}
	}
}

// readFile is a helper to read file content
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Log tail messages and commands

// logTailLineMsg is sent when a new line is read from the agent log file
type logTailLineMsg struct {
	line    string
	isError bool
}

// logTailErrorMsg is sent when there's an error reading the log file
type logTailErrorMsg struct {
	err error
}

// logTailClosedMsg is sent when the log tail is closed
type logTailClosedMsg struct{}

// retryLogTailMsg is sent to retry starting the log tailer
type retryLogTailMsg struct{}

// LogTailer reads lines from a log file and streams them via a channel
type LogTailer struct {
	file     *os.File
	done     chan struct{}
	closed   bool
	mu       sync.Mutex
	offset   int64
	filePath string
}

// NewLogTailer creates a new log tailer for the given file path.
// If readExisting is true, it starts from the beginning of the file to read existing content.
// If readExisting is false, it seeks to the end and only reads new content.
func NewLogTailer(filePath string, readExisting bool) (*LogTailer, error) {
	// Open file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	var offset int64 = 0
	if !readExisting {
		// Seek to end of file (we only want new content)
		offset, err = file.Seek(0, io.SeekEnd)
		if err != nil {
			file.Close()
			return nil, err
		}
	}

	return &LogTailer{
		file:     file,
		done:     make(chan struct{}),
		offset:   offset,
		filePath: filePath,
	}, nil
}

// Close stops the log tailer
func (t *LogTailer) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	close(t.done)
	return t.file.Close()
}

// IsClosed returns whether the tailer is closed
func (t *LogTailer) IsClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

// startLogTailCmd creates a command that starts tailing a log file.
// If readExisting is true, it reads from the beginning of the file (for reconnecting to existing sessions).
// If readExisting is false, it starts from the end and only reads new content (for fresh agent starts).
func startLogTailCmd(projectDir, sessionID string, readExisting bool) tea.Cmd {
	return func() tea.Msg {
		logPath := filepath.Join(projectDir, ".juggle", "sessions", sessionID, "agent.log")

		tailer, err := NewLogTailer(logPath, readExisting)
		if err != nil {
			// Log file might not exist yet - that's OK, we'll try again
			return logTailErrorMsg{err: err}
		}

		return logTailerStartedMsg{tailer: tailer}
	}
}

// logTailerStartedMsg is sent when a log tailer has been started
type logTailerStartedMsg struct {
	tailer *LogTailer
}

// logTailPollMsg is sent to continue polling the log file
type logTailPollMsg struct {
	tailer *LogTailer
}

// listenForLogTailCmd creates a command that reads new lines from the log file
func listenForLogTailCmd(tailer *LogTailer) tea.Cmd {
	return func() tea.Msg {
		if tailer == nil || tailer.IsClosed() {
			return logTailClosedMsg{}
		}

		// Check for new content
		buf := make([]byte, 4096)
		n, err := tailer.file.Read(buf)
		if err != nil {
			if err == io.EOF {
				// No new content - signal to poll again after a delay
				return logTailPollMsg{tailer: tailer}
			}
			return logTailErrorMsg{err: err}
		}

		if n > 0 {
			// Parse lines from the buffer
			content := string(buf[:n])
			lines := strings.Split(content, "\n")

			// Return the first non-empty line
			for _, line := range lines {
				if line != "" {
					// Detect error lines
					isError := strings.HasPrefix(line, "Error:") ||
						strings.HasPrefix(line, "error:") ||
						strings.Contains(line, "panic:") ||
						strings.Contains(line, "FATAL")
					return logTailLineMsg{line: line, isError: isError}
				}
			}
		}

		// No content found, poll again
		return logTailPollMsg{tailer: tailer}
	}
}

// Daemon control messages and commands

// runningDaemonsScannedMsg is sent when daemon scan completes on startup
type runningDaemonsScannedMsg struct {
	daemons map[string]*DaemonInfo
	err     error
}

// scanRunningDaemonsCmd scans all sessions for running daemons
func scanRunningDaemonsCmd(projectDir string, sessions []*session.JuggleSession) tea.Cmd {
	return func() tea.Msg {
		daemons := make(map[string]*DaemonInfo)

		for _, sess := range sessions {
			// Skip pseudo-sessions
			if sess.ID == "__all__" || sess.ID == "__untagged__" {
				continue
			}

			running, info, err := daemon.IsRunning(projectDir, sess.ID)
			if err != nil {
				continue // Skip this session on error
			}
			if running && info != nil {
				// Read current state to get iteration info
				state, _ := daemon.ReadStateFile(projectDir, sess.ID)
				iteration := 0
				maxIter := 0
				if state != nil {
					iteration = state.Iteration
					maxIter = state.MaxIterations
				} else if info != nil {
					maxIter = info.MaxIterations
				}

				daemons[sess.ID] = &DaemonInfo{
					SessionID:  sess.ID,
					ProjectDir: projectDir,
					Running:    true,
					Iteration:  iteration,
					MaxIter:    maxIter,
				}
			}
		}

		return runningDaemonsScannedMsg{daemons: daemons}
	}
}

// daemonControlSentMsg is sent when a control command was sent to the daemon
type daemonControlSentMsg struct {
	command string
}

// daemonControlErrorMsg is sent when sending a control command fails
type daemonControlErrorMsg struct {
	err error
}

// daemonStateLoadedMsg is sent when daemon state is loaded
type daemonStateLoadedMsg struct {
	running          bool
	paused           bool
	currentBallID    string
	currentBallTitle string
	iteration        int
	maxIterations    int
	acsComplete      int
	acsTotal         int
	model            string
	provider         string
	status           string    // Status message when stopped (e.g., "No workable balls")
	startedAt        time.Time // When the daemon actually started
	err              error
}

// sendDaemonControlCmd creates a command that sends a control command to the daemon
func sendDaemonControlCmd(projectDir, sessionID, command, args string) tea.Cmd {
	return func() tea.Msg {
		err := sendDaemonControl(projectDir, sessionID, command, args)
		if err != nil {
			return daemonControlErrorMsg{err: err}
		}
		return daemonControlSentMsg{command: command}
	}
}

// sendDaemonControl writes a control command to the daemon control file
func sendDaemonControl(projectDir, sessionID, command, args string) error {
	return daemon.SendControlCommand(projectDir, sessionID, command, args)
}

// loadDaemonStateCmd creates a command that loads the daemon state from the state file
func loadDaemonStateCmd(projectDir, sessionID string) tea.Cmd {
	return func() tea.Msg {
		state, err := daemon.ReadStateFile(projectDir, sessionID)
		if err != nil {
			return daemonStateLoadedMsg{err: err}
		}

		return daemonStateLoadedMsg{
			running:          state.Running,
			paused:           state.Paused,
			currentBallID:    state.CurrentBallID,
			currentBallTitle: state.CurrentBallTitle,
			iteration:        state.Iteration,
			maxIterations:    state.MaxIterations,
			acsComplete:      state.ACsComplete,
			acsTotal:         state.ACsTotal,
			model:            state.Model,
			provider:         state.Provider,
			status:           state.Status,
			startedAt:        state.StartedAt,
		}
	}
}

// agentUpdateLoadedMsg is sent when agent-update.txt is loaded
type agentUpdateLoadedMsg struct {
	phase   string
	message string
	ballID  string
	err     error
}

// loadAgentUpdateCmd creates a command that loads the agent update from the update file
func loadAgentUpdateCmd(sessionStore *session.SessionStore, sessionID string) tea.Cmd {
	return func() tea.Msg {
		content, err := sessionStore.LoadAgentUpdate(sessionID)
		if err != nil {
			return agentUpdateLoadedMsg{err: err}
		}

		if content == "" {
			return agentUpdateLoadedMsg{}
		}

		// Parse the content: [timestamp] ball=<id> state=<state> message=<msg>
		phase, message, ballID := parseAgentUpdate(content)
		return agentUpdateLoadedMsg{
			phase:   phase,
			message: message,
			ballID:  ballID,
		}
	}
}

// parseAgentUpdate parses the agent-update.txt content
// Format: [timestamp] ball=<id> state=<state> message=<msg>
func parseAgentUpdate(content string) (phase, message, ballID string) {
	// Find ball=
	if idx := strings.Index(content, "ball="); idx != -1 {
		rest := content[idx+5:]
		if spaceIdx := strings.Index(rest, " "); spaceIdx != -1 {
			ballID = rest[:spaceIdx]
			rest = rest[spaceIdx+1:]
		}
		// Find state=
		if stateIdx := strings.Index(rest, "state="); stateIdx != -1 {
			rest = rest[stateIdx+6:]
			if spaceIdx := strings.Index(rest, " "); spaceIdx != -1 {
				phase = rest[:spaceIdx]
				rest = rest[spaceIdx+1:]
			}
			// Find message=
			if msgIdx := strings.Index(rest, "message="); msgIdx != -1 {
				message = strings.TrimSpace(rest[msgIdx+8:])
			}
		}
	}
	return
}

// AgentMetricsState holds metrics from Claude Code hooks
type AgentMetricsState struct {
	FilesChanged    []string
	ToolCounts      map[string]int
	ToolFailures    int
	TotalTools      int
	TurnCount       int
	InputTokens     int
	OutputTokens    int
	CacheReadTokens int
	LastActivity    time.Time
	SessionEnded    bool
}

// agentMetricsLoadedMsg is sent when agent-metrics.json is loaded
type agentMetricsLoadedMsg struct {
	metrics *AgentMetricsState
	err     error
}

// loadAgentMetricsCmd creates a command that loads the agent metrics from the metrics file
func loadAgentMetricsCmd(sessionStore *session.SessionStore, sessionID string) tea.Cmd {
	return func() tea.Msg {
		metrics, err := sessionStore.LoadMetrics(sessionID)
		if err != nil {
			return agentMetricsLoadedMsg{err: err}
		}

		return agentMetricsLoadedMsg{
			metrics: &AgentMetricsState{
				FilesChanged:    metrics.FilesChanged,
				ToolCounts:      metrics.ToolCounts,
				ToolFailures:    metrics.ToolFailures,
				TotalTools:      metrics.TotalTools,
				TurnCount:       metrics.TurnCount,
				InputTokens:     metrics.InputTokens,
				OutputTokens:    metrics.OutputTokens,
				CacheReadTokens: metrics.CacheReadTokens,
				LastActivity:    metrics.LastActivity,
				SessionEnded:    metrics.SessionEnded,
			},
		}
	}
}

// updatesHistoryLoadedMsg is sent when updates.jsonl is loaded
type updatesHistoryLoadedMsg struct {
	updates []*session.UpdateEntry
	err     error
}

// loadUpdatesHistoryCmd creates a command that loads the updates history from the JSONL file
func loadUpdatesHistoryCmd(sessionStore *session.SessionStore, sessionID string) tea.Cmd {
	return func() tea.Msg {
		updates, err := sessionStore.LoadUpdates(sessionID)
		if err != nil {
			return updatesHistoryLoadedMsg{err: err}
		}
		return updatesHistoryLoadedMsg{updates: updates}
	}
}

// parseStreamJSONEvent parses a stream-JSON SSE event and returns appropriate message
func parseStreamJSONEvent(eventType, jsonData string) tea.Msg {
	switch eventType {
	case "message_start":
		var data struct {
			Message struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(jsonData), &data); err == nil {
			return agentStreamJSONMsg{
				eventType:    "message_start",
				inputTokens:  data.Message.Usage.InputTokens,
				outputTokens: data.Message.Usage.OutputTokens,
			}
		}

	case "content_block_start":
		var data struct {
			Index        int `json:"index"`
			ContentBlock struct {
				Type string `json:"type"` // "text", "tool_use", "thinking"
				Name string `json:"name,omitempty"`
			} `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(jsonData), &data); err == nil {
			msg := agentStreamJSONMsg{eventType: "content_block_start"}
			if data.ContentBlock.Type == "tool_use" {
				msg.activeTool = data.ContentBlock.Name
			} else if data.ContentBlock.Type == "thinking" {
				msg.thinking = true
			}
			return msg
		}

	case "content_block_delta":
		var data struct {
			Index int `json:"index"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text,omitempty"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(jsonData), &data); err == nil {
			if data.Delta.Type == "text_delta" && data.Delta.Text != "" {
				// Send text output for display
				return agentOutputMsg{line: data.Delta.Text, isError: false}
			}
		}

	case "content_block_stop":
		return agentStreamJSONMsg{
			eventType:  "content_block_stop",
			activeTool: "", // Clear active tool
			thinking:   false,
		}

	case "message_delta":
		var data struct {
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(jsonData), &data); err == nil {
			return agentStreamJSONMsg{
				eventType:    "message_delta",
				outputTokens: data.Usage.OutputTokens,
			}
		}
	}

	return nil
}
