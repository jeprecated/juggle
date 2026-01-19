package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbletea"
	"github.com/ohare93/juggle/internal/session"
	"github.com/ohare93/juggle/internal/watcher"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		// Update spinner only when in monitor view and agent is running
		if m.mode == agentMonitorView && m.agentStatus.Running && !m.agentMonitorPaused {
			var cmd tea.Cmd
			m.agentSpinner, cmd = m.agentSpinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		// Handle unified ball form view (all fields in one view)
		if m.mode == unifiedBallFormView {
			return m.handleUnifiedBallFormKey(msg)
		}

		// Handle input modes (text entry)
		if m.mode == inputSessionView || m.mode == inputBallView || m.mode == inputBlockedView || m.mode == inputTagView {
			return m.handleInputKey(msg)
		}

		// Handle session selector mode
		if m.mode == sessionSelectorView {
			return m.handleSessionSelectorKey(msg)
		}

		// Handle dependency selector mode
		if m.mode == dependencySelectorView {
			return m.handleDependencySelectorKey(msg)
		}

		// Handle panel search input
		if m.mode == panelSearchView {
			return m.handlePanelSearchKey(msg)
		}

		// Handle delete confirmation in split view
		if m.mode == confirmSplitDelete {
			return m.handleSplitConfirmDelete(msg)
		}

		// Handle agent cancel confirmation
		if m.mode == confirmAgentCancel {
			return m.handleAgentCancelConfirm(msg)
		}

		// Handle split help view
		if m.mode == splitHelpView {
			return m.handleSplitHelpKey(msg)
		}

		// Handle split view specific keys
		if m.mode == splitView {
			return m.handleSplitViewKey(msg)
		}

		// Handle history view keys
		if m.mode == historyView {
			return m.handleHistoryViewKey(msg)
		}

		// Handle history output view keys
		if m.mode == historyOutputView {
			return m.handleHistoryOutputViewKey(msg)
		}

		// Handle agent monitor view keys
		if m.mode == agentMonitorView {
			return m.handleAgentMonitorKey(msg)
		}

	case ballsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.balls = msg.balls
		m.applyFilters()
		// Reset cursor if it's out of bounds
		if m.cursor >= len(m.filteredBalls) {
			m.cursor = 0
		}
		m.addActivity("Balls loaded")
		return m, nil

	case sessionsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.sessions = msg.sessions
		// Reset session cursor if out of bounds
		if m.sessionCursor >= len(m.sessions) {
			m.sessionCursor = 0
		}

		// Handle pending session select (from mode toggle)
		if m.pendingSessionSelect != "" {
			found := false
			for i, sess := range m.sessions {
				if sess.ID == m.pendingSessionSelect {
					m.selectedSession = sess
					m.sessionCursor = i
					found = true
					break
				}
			}
			if !found && len(m.sessions) > 0 {
				// Selected session not in new list, select nearest (first session)
				m.sessionCursor = 0
				m.selectedSession = m.sessions[0]
				m.addActivity("Previous session not available, selected: " + m.sessions[0].ID)
			} else if !found {
				m.selectedSession = nil
				m.sessionCursor = 0
			}
			m.pendingSessionSelect = ""
		} else if m.selectedSession == nil {
			// Pre-select session on initial load
			if m.initialSessionID != "" {
				// Use the provided initial session ID
				for i, sess := range m.sessions {
					if sess.ID == m.initialSessionID {
						m.selectedSession = sess
						m.sessionCursor = i
						m.addActivity("Pre-selected session: " + sess.ID)
						break
					}
				}
				// Clear the initialSessionID after attempting selection
				m.initialSessionID = ""
			} else {
				// Default to "All" pseudo-session (first in filtered list)
				filtered := m.filterSessions()
				if len(filtered) > 0 {
					m.selectedSession = filtered[0] // PseudoSessionAll is first
					m.sessionCursor = 0
				}
			}
		}
		m.addActivity("Sessions loaded")

		// Scan for running daemons after sessions are loaded
		if m.store != nil {
			return m, scanRunningDaemonsCmd(m.store.ProjectDir(), m.sessions)
		}
		return m, nil

	case runningDaemonsScannedMsg:
		if msg.err != nil {
			m.addActivity("Error scanning daemons: " + msg.err.Error())
			return m, nil
		}
		m.runningDaemons = msg.daemons
		if len(msg.daemons) > 0 {
			m.addActivity(fmt.Sprintf("Found %d running agent daemon(s)", len(msg.daemons)))
		}
		return m, nil

	case ballUpdatedMsg:
		if msg.err != nil {
			m.message = "Error: " + msg.err.Error()
			m.addActivity("Error: " + msg.err.Error())
		} else {
			m.message = "Ball updated successfully"
			m.addActivity("Ball updated: " + msg.ball.ID)
		}
		// Reload balls
		return m, loadBalls(m.store, m.config, m.localOnly)

	case ballArchivedMsg:
		if msg.err != nil {
			m.message = "Error: " + msg.err.Error()
			m.addActivity("Error archiving: " + msg.err.Error())
		} else {
			m.message = "Ball archived successfully"
			m.addActivity("Archived ball: " + msg.ball.ID)
		}
		// Reload balls
		return m, loadBalls(m.store, m.config, m.localOnly)

	case watcherEventMsg:
		return m.handleWatcherEvent(msg.event)

	case watcherErrorMsg:
		m.addActivity("Watcher error: " + msg.err.Error())
		// Continue listening for more events
		if m.fileWatcher != nil {
			return m, listenForWatcherEvents(m.fileWatcher)
		}
		return m, nil

	case editorResultMsg:
		return m.handleEditorResult(msg)

	case agentStartedMsg:
		m.agentStatus = AgentStatus{
			Running:       true,
			SessionID:     msg.sessionID,
			Iteration:     0,
			MaxIterations: 10, // Default
		}
		// Add to runningDaemons
		if m.runningDaemons == nil {
			m.runningDaemons = make(map[string]*DaemonInfo)
		}
		projectDir := ""
		if m.store != nil {
			projectDir = m.store.ProjectDir()
		}
		m.runningDaemons[msg.sessionID] = &DaemonInfo{
			SessionID:  msg.sessionID,
			ProjectDir: projectDir,
			Running:    true,
			Iteration:  0,
			MaxIter:    10,
		}
		m.addActivity("Agent started for session: " + msg.sessionID)
		m.message = "Agent running..."
		return m, nil

	case agentProcessStartedMsg:
		// Store the process reference for cancellation
		m.agentProcess = msg.process
		m.agentStatus = AgentStatus{
			Running:       true,
			SessionID:     msg.sessionID,
			Iteration:     0,
			MaxIterations: 10, // Default
		}
		// Add to runningDaemons
		if m.runningDaemons == nil {
			m.runningDaemons = make(map[string]*DaemonInfo)
		}
		projectDir := ""
		if m.store != nil {
			projectDir = m.store.ProjectDir()
		}
		m.runningDaemons[msg.sessionID] = &DaemonInfo{
			SessionID:  msg.sessionID,
			ProjectDir: projectDir,
			Running:    true,
			Iteration:  0,
			MaxIter:    10,
		}
		m.addActivity("Agent process started for session: " + msg.sessionID)
		m.message = "Agent running... (X to cancel)"
		// Start waiting for the process completion and continue listening for output
		return m, tea.Batch(
			waitForAgentCmd(msg.process),
			listenForAgentOutput(m.agentOutputCh),
		)

	case agentCancelledMsg:
		m.agentStatus.Running = false
		m.agentProcess = nil
		// Close and nil out the output channel to prevent goroutine leaks
		if m.agentOutputCh != nil {
			close(m.agentOutputCh)
			m.agentOutputCh = nil
		}
		// Remove from runningDaemons
		if m.runningDaemons != nil {
			delete(m.runningDaemons, msg.sessionID)
		}
		m.message = "Agent cancelled"
		m.addActivity("Agent cancelled for session: " + msg.sessionID)
		m.addAgentOutput("=== Agent cancelled by user ===", true)
		// Reload balls to reflect any changes made before cancellation
		return m, loadBalls(m.store, m.config, m.localOnly)

	case agentIterationMsg:
		m.agentStatus.Iteration = msg.iteration
		m.agentStatus.MaxIterations = msg.maxIter
		m.addActivity(fmt.Sprintf("Agent iteration %d/%d", msg.iteration, msg.maxIter))
		return m, nil

	case agentFinishedMsg:
		m.agentStatus.Running = false
		m.agentProcess = nil // Clear process reference
		// Close and nil out the output channel to prevent goroutine leaks
		if m.agentOutputCh != nil {
			close(m.agentOutputCh)
			m.agentOutputCh = nil
		}
		// Remove from runningDaemons
		if m.runningDaemons != nil {
			delete(m.runningDaemons, msg.sessionID)
		}
		if msg.err != nil {
			m.message = "Agent error: " + msg.err.Error()
			m.addActivity("Agent error: " + msg.err.Error())
			m.addAgentOutput("=== Agent Error: "+msg.err.Error()+" ===", true)
		} else if msg.complete {
			m.message = "Agent complete!"
			m.addActivity("Agent completed: " + msg.sessionID)
			m.addAgentOutput("=== Agent completed ===", false)
		} else if msg.blocked {
			m.message = "Agent blocked: " + msg.blockedReason
			m.addActivity("Agent blocked: " + msg.blockedReason)
			m.addAgentOutput("=== Agent blocked: "+msg.blockedReason+" ===", true)
		} else {
			m.message = "Agent finished (max iterations)"
			m.addActivity("Agent finished: max iterations reached")
			m.addAgentOutput("=== Agent finished (max iterations) ===", false)
		}
		// Reload balls to reflect any changes
		return m, loadBalls(m.store, m.config, m.localOnly)

	case agentOutputMsg:
		// Add the output line to our buffer
		m.addAgentOutput(msg.line, msg.isError)
		// Continue listening for more output if agent is still running
		if m.agentStatus.Running && m.agentOutputCh != nil {
			return m, listenForAgentOutput(m.agentOutputCh)
		}
		return m, nil

	case historyLoadedMsg:
		if msg.err != nil {
			m.message = "Error loading history: " + msg.err.Error()
			m.addActivity("Error loading history: " + msg.err.Error())
			m.mode = splitView
			return m, nil
		}
		m.agentHistory = msg.history
		m.historyCursor = 0
		m.historyScrollOffset = 0
		m.mode = historyView
		m.addActivity("Loaded agent history: " + strconv.Itoa(len(msg.history)) + " runs")
		return m, nil

	case historyOutputLoadedMsg:
		if msg.err != nil {
			m.historyOutput = "Error loading output: " + msg.err.Error()
		} else {
			m.historyOutput = msg.content
		}
		m.historyOutputOffset = 0
		m.mode = historyOutputView
		return m, nil

	case daemonControlSentMsg:
		// Control command was sent successfully
		m.addActivity("Sent daemon command: " + msg.command)
		return m, nil

	case daemonControlErrorMsg:
		// Control command failed
		m.message = "Daemon control error: " + msg.err.Error()
		m.addActivity("Daemon control error: " + msg.err.Error())
		return m, nil

	case daemonStateLoadedMsg:
		if msg.err != nil {
			m.message = "Failed to load daemon state"
			return m, nil
		}
		// Update agent status from daemon state
		m.agentStatus.Running = msg.running
		m.agentStatus.Iteration = msg.iteration
		m.agentStatus.MaxIterations = msg.maxIterations
		m.agentStatus.CurrentBallID = msg.currentBallID
		m.agentStatus.CurrentBallTitle = msg.currentBallTitle
		m.agentStatus.ACsComplete = msg.acsComplete
		m.agentStatus.ACsTotal = msg.acsTotal
		m.agentStatus.Model = msg.model
		m.agentStatus.Provider = msg.provider
		m.agentStatus.Status = msg.status
		m.agentMonitorPaused = msg.paused
		// Use daemon's actual start time for elapsed calculation (not TUI connection time)
		if !msg.startedAt.IsZero() {
			m.agentMonitorStartTime = msg.startedAt
		}
		return m, nil

	case agentUpdateLoadedMsg:
		if msg.err != nil {
			// Silently ignore errors loading agent update
			return m, nil
		}
		// Update agent status with phase info
		if msg.phase != "" {
			m.agentStatus.Phase = msg.phase
			m.agentStatus.PhaseMessage = msg.message
		}
		return m, nil

	case agentMetricsLoadedMsg:
		if msg.err != nil {
			// Silently ignore errors loading agent metrics
			return m, nil
		}
		// Update metrics state
		m.agentMetrics = msg.metrics
		return m, nil

	case logTailerStartedMsg:
		// Store the log tailer and start listening for log lines
		m.agentLogTailer = msg.tailer
		return m, listenForLogTailCmd(m.agentLogTailer)

	case logTailLineMsg:
		// Add the log line to the agent output buffer
		m.addAgentOutput(msg.line, msg.isError)
		// Track error for prominent display
		if msg.isError {
			m.agentDaemonError = msg.line
		}
		// Continue listening for more log lines if in monitor mode
		if m.mode == agentMonitorView && m.agentLogTailer != nil && !m.agentLogTailer.IsClosed() {
			return m, listenForLogTailCmd(m.agentLogTailer)
		}
		return m, nil

	case logTailErrorMsg:
		// Log file error - might not exist yet, retry after a delay
		if m.mode == agentMonitorView && m.store != nil && m.agentStatus.SessionID != "" {
			// Retry starting the log tail after a brief delay
			return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return retryLogTailMsg{}
			})
		}
		return m, nil

	case retryLogTailMsg:
		// Retry starting the log tailer
		if m.mode == agentMonitorView && m.store != nil && m.agentStatus.SessionID != "" {
			// If reconnecting, read existing content; otherwise just tail new content
			return m, startLogTailCmd(m.store.ProjectDir(), m.agentStatus.SessionID, m.agentMonitorReconnected)
		}
		return m, nil

	case logTailPollMsg:
		// Poll again after a short delay
		if m.mode == agentMonitorView && msg.tailer != nil && !msg.tailer.IsClosed() {
			return m, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
				return listenForLogTailCmd(msg.tailer)()
			})
		}
		return m, nil

	case logTailClosedMsg:
		// Log tailer was closed - cleanup
		m.agentLogTailer = nil
		return m, nil
	}

	return m, nil
}

// handleEditorResult handles the result from external editor
func (m Model) handleEditorResult(msg editorResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.message = "Editor error: " + msg.err.Error()
		m.addActivity("Editor error: " + msg.err.Error())
		return m, nil
	}

	if msg.cancelled {
		m.message = "Edit cancelled (no changes)"
		m.addActivity("Edit cancelled for: " + msg.ball.ID)
		return m, nil
	}

	// Parse the edited YAML and apply changes
	if err := yamlToBall(msg.editedYAML, msg.ball); err != nil {
		m.message = "Parse error: " + err.Error()
		m.addActivity("Parse error: " + err.Error())
		return m, nil
	}

	// Save the updated ball
	store, err := session.NewStore(msg.ball.WorkingDir)
	if err != nil {
		m.message = "Error: " + err.Error()
		m.addActivity("Store error: " + err.Error())
		return m, nil
	}

	m.addActivity("Updated ball: " + msg.ball.ID)
	m.message = "Updated ball: " + msg.ball.ID
	return m, updateBall(store, msg.ball)
}

// handleSplitViewKey handles keyboard input for split view mode
// Uses two-key sequences for state changes (s+key) and toggles (t+key)
func (m Model) handleSplitViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle two-key sequences for state changes
	if m.pendingKeySequence == "s" {
		m.pendingKeySequence = ""
		return m.handleStateKeySequence(key)
	}

	// Handle two-key sequences for toggle filters
	if m.pendingKeySequence == "t" {
		m.pendingKeySequence = ""
		return m.handleToggleKeySequence(key)
	}

	// Handle two-key sequences for view columns (vc, vp, vt, va)
	if m.pendingKeySequence == "v" {
		m.pendingKeySequence = ""
		return m.handleViewColumnKeySequence(key)
	}

	// Handle two-key sequences for move ball to session (m + digit)
	if m.pendingKeySequence == "m" {
		m.pendingKeySequence = ""
		return m.handleMoveKeySequence(key, false) // false = replace sessions
	}

	// Handle two-key sequences for append ball to session (M + digit)
	if m.pendingKeySequence == "M" {
		m.pendingKeySequence = ""
		return m.handleMoveKeySequence(key, true) // true = append to sessions
	}

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "tab", "l":
		// Cycle to next panel
		m.message = ""
		switch m.activePanel {
		case SessionsPanel:
			m.activePanel = BallsPanel
		case BallsPanel:
			m.activePanel = ActivityPanel
		case ActivityPanel:
			m.activePanel = SessionsPanel
		}
		return m, nil

	case "shift+tab", "h":
		// Cycle to previous panel
		m.message = ""
		switch m.activePanel {
		case SessionsPanel:
			m.activePanel = ActivityPanel
		case BallsPanel:
			m.activePanel = SessionsPanel
		case ActivityPanel:
			m.activePanel = BallsPanel
		}
		return m, nil

	case "left":
		// Left arrow: Sessions <- Balls, Balls <- Activity
		m.message = ""
		switch m.activePanel {
		case BallsPanel:
			m.activePanel = SessionsPanel
		case ActivityPanel:
			m.activePanel = BallsPanel
			// From SessionsPanel, left does nothing (left edge)
		}
		return m, nil

	case "right":
		// Right arrow: Sessions -> Balls, Balls -> Activity
		m.message = ""
		switch m.activePanel {
		case SessionsPanel:
			m.activePanel = BallsPanel
		case BallsPanel:
			m.activePanel = ActivityPanel
			// From ActivityPanel, right does nothing (right edge)
		}
		return m, nil

	case "up", "k":
		m.message = ""
		// Up arrow: cycle from Activity back to Balls
		if m.activePanel == ActivityPanel && !m.agentOutputVisible {
			m.activePanel = BallsPanel
			return m, nil
		}
		// Route to agent output panel if visible
		if m.agentOutputVisible {
			return m.handleAgentOutputScrollUp()
		}
		return m.handleSplitViewNavUp()

	case "down", "j":
		m.message = ""
		// Route to agent output panel if visible
		if m.agentOutputVisible {
			return m.handleAgentOutputScrollDown()
		}
		// Check if at bottom of current panel before switching to Activity
		if m.activePanel == SessionsPanel {
			sessions := m.filterSessions()
			if m.sessionCursor < len(sessions)-1 {
				return m.handleSplitViewNavDown()
			}
			// At bottom of sessions, move to Activity
			m.activePanel = ActivityPanel
			return m, nil
		}
		if m.activePanel == BallsPanel {
			balls := m.filterBallsForSession()
			if m.cursor < len(balls)-1 {
				return m.handleSplitViewNavDown()
			}
			// At bottom of balls, move to Activity
			m.activePanel = ActivityPanel
			return m, nil
		}
		// In ActivityPanel, scroll within it
		return m.handleSplitViewNavDown()

	case "ctrl+d":
		// Page down in agent output panel if visible
		if m.agentOutputVisible {
			return m.handleAgentOutputPageDown()
		}
		// Page down in activity log
		if m.activePanel == ActivityPanel {
			return m.handleActivityLogPageDown()
		}
		return m, nil

	case "ctrl+u":
		// Page up in agent output panel if visible
		if m.agentOutputVisible {
			return m.handleAgentOutputPageUp()
		}
		// Page up in activity log (or clear filter in other panels)
		if m.activePanel == ActivityPanel {
			return m.handleActivityLogPageUp()
		}
		// Clear the current panel filter
		m.panelSearchQuery = ""
		m.panelSearchActive = false
		m.addActivity("Filter cleared")
		m.message = "Filter cleared"
		return m, nil

	case "g":
		// Handle gg for go to top of agent output panel if visible
		if m.agentOutputVisible {
			if m.lastKey == "g" {
				m.lastKey = ""
				return m.handleAgentOutputGoToTop()
			}
			m.lastKey = "g"
			return m, nil
		}
		// Handle gg for go to top of activity log
		if m.activePanel == ActivityPanel {
			if m.lastKey == "g" {
				m.lastKey = ""
				return m.handleActivityLogGoToTop()
			}
			m.lastKey = "g"
			return m, nil
		}
		return m, nil

	case "G":
		// Go to bottom of agent output panel if visible
		if m.agentOutputVisible {
			m.lastKey = ""
			return m.handleAgentOutputGoToBottom()
		}
		// Go to bottom of activity log
		if m.activePanel == ActivityPanel {
			m.lastKey = ""
			return m.handleActivityLogGoToBottom()
		}
		return m, nil

	case "enter":
		return m.handleSplitViewEnter()

	case "esc":
		// Clear any pending key sequences first
		if m.pendingKeySequence != "" {
			m.pendingKeySequence = ""
			m.message = "Cancelled"
			return m, nil
		}
		// Clear multi-select first, then deselect session
		if len(m.selectedBalls) > 0 {
			m.selectedBalls = make(map[string]bool)
			m.message = "Selection cleared"
			return m, nil
		}
		// Go back or deselect
		if m.selectedSession != nil {
			m.selectedSession = nil
			m.cursor = 0
		} else {
			return m, tea.Quit
		}
		return m, nil

	case " ":
		// Space key: toggle ball selection and move cursor down
		if m.activePanel == BallsPanel {
			balls := m.filterBallsForSession()
			if len(balls) > 0 && m.cursor < len(balls) {
				ball := balls[m.cursor]
				// Toggle selection
				if m.selectedBalls[ball.ID] {
					delete(m.selectedBalls, ball.ID)
				} else {
					m.selectedBalls[ball.ID] = true
				}
				// Move cursor down if not at end
				if m.cursor < len(balls)-1 {
					m.cursor++
				}
			}
			return m, nil
		}
		return m, nil

	case "s":
		// Start two-key sequence for state changes (sc=complete, sb=blocked, ss=start, sp=pending, sa=archive)
		if m.activePanel == BallsPanel {
			m.pendingKeySequence = "s"
			m.message = "s: State change... (c=complete, s=start, b=blocked, p=pending, a=archive)"
			return m, nil
		}
		return m, nil

	case "t":
		// Start two-key sequence for toggle filters (tc=complete, tb=blocked, ti=in_progress, tp=pending)
		m.pendingKeySequence = "t"
		m.message = "t: Toggle filter... (c=complete, b=blocked, i=in_progress, p=pending, a=all)"
		return m, nil

	case "R":
		// Refresh
		m.message = "Reloading..."
		m.addActivity("Refreshing data...")
		return m, tea.Batch(
			loadBalls(m.store, m.config, m.localOnly),
			loadSessions(m.sessionStore, m.config, m.localOnly),
		)

	case "?":
		// Show comprehensive help view
		m.helpScrollOffset = 0 // Reset scroll position
		m.mode = splitHelpView
		return m, nil

	case "a":
		// Add new item based on panel
		return m.handleSplitAddItem()

	case "e":
		// Edit selected item based on panel
		return m.handleSplitEditItem()

	case "d":
		// Delete selected item with confirmation
		return m.handleSplitDeletePrompt()

	case "/":
		// Open search/filter for current panel
		return m.handlePanelSearchStart()

	case "[":
		// Switch to previous session while in balls panel
		if m.activePanel == BallsPanel {
			return m.handleSessionSwitchPrev()
		}
		return m, nil

	case "]":
		// Switch to next session while in balls panel
		if m.activePanel == BallsPanel {
			return m.handleSessionSwitchNext()
		}
		return m, nil

	case "i":
		// Toggle bottom pane between activity log and ball detail
		return m.handleToggleBottomPane()

	case "P":
		// Toggle between local project only and all projects
		return m.handleToggleLocalOnly()

	case "o":
		// Toggle sort order for balls
		return m.handleToggleSortOrder()

	case "v":
		// Start two-key sequence for view column toggles (vp=priority, vt=tags, vm=model, va=all)
		if m.activePanel == BallsPanel {
			m.pendingKeySequence = "v"
			m.message = "v: View columns... (p=priority, t=tags, m=model, a=all)"
			return m, nil
		}
		return m, nil

	case "m":
		// Start two-key sequence for moving ball to session (m+1-9,0)
		if m.activePanel == BallsPanel {
			m.pendingKeySequence = "m"
			m.message = "m: Move ball to session... (1-9,0 = session number)"
			return m, nil
		}
		return m, nil

	case "M":
		// Start two-key sequence for appending ball to session (M+1-9,0)
		if m.activePanel == BallsPanel {
			m.pendingKeySequence = "M"
			m.message = "M: Add ball to session... (1-9,0 = session number, keeps existing)"
			return m, nil
		}
		return m, nil

	case "backspace":
		// Remove current session from selected ball
		if m.activePanel == BallsPanel {
			return m.handleRemoveCurrentSessionFromBall()
		}
		return m, nil

	case "O":
		// Toggle agent output panel visibility
		return m.handleToggleAgentOutput()

	case "E":
		// Toggle agent output panel expansion (when visible)
		if m.agentOutputVisible {
			return m.handleToggleAgentOutputExpand()
		}
		// Open editor for ball in BallsPanel
		if m.activePanel == BallsPanel {
			return m.handleBallEditInEditor()
		}
		return m, nil

	case "X":
		// Cancel running agent (with confirmation)
		return m.handleCancelAgent()

	case "H":
		// Show agent history view
		return m.handleShowHistory()

	case "W":
		// Enter agent monitor view for a running daemon
		// First check if we have an active agentStatus (from current TUI session)
		if m.agentStatus.Running {
			m.mode = agentMonitorView
			m.agentMonitorStartTime = m.nowFunc()
			// Start spinner and log tail (false = fresh agent from current session, only tail new content)
			cmds := []tea.Cmd{m.agentSpinner.Tick}
			if m.store != nil && m.agentStatus.SessionID != "" {
				cmds = append(cmds, startLogTailCmd(m.store.ProjectDir(), m.agentStatus.SessionID, false))
			}
			return m, tea.Batch(cmds...)
		}

		// Check for running daemon on the currently highlighted session (if in sessions panel)
		// or on the selected session
		var targetSessionID string
		if m.activePanel == SessionsPanel {
			sessions := m.filterSessions()
			if m.sessionCursor < len(sessions) {
				targetSessionID = sessions[m.sessionCursor].ID
			}
		} else if m.selectedSession != nil {
			targetSessionID = m.selectedSession.ID
		}

		// Check if this session has a running daemon
		if targetSessionID != "" && m.runningDaemons != nil {
			if daemonInfo, ok := m.runningDaemons[targetSessionID]; ok && daemonInfo.Running {
				// Set up agentStatus from daemon info
				m.agentStatus = AgentStatus{
					Running:       true,
					SessionID:     targetSessionID,
					Iteration:     daemonInfo.Iteration,
					MaxIterations: daemonInfo.MaxIter,
				}
				m.agentMonitorReconnected = true
				m.mode = agentMonitorView
				m.agentMonitorStartTime = m.nowFunc()

				// Start spinner and log tail, load daemon state for full info
				// true = reconnecting, read existing log content
				cmds := []tea.Cmd{m.agentSpinner.Tick}
				if m.store != nil {
					cmds = append(cmds, loadDaemonStateCmd(m.store.ProjectDir(), targetSessionID))
					cmds = append(cmds, startLogTailCmd(m.store.ProjectDir(), targetSessionID, true))
				}
				// Also load agent update for phase info
				if m.sessionStore != nil {
					cmds = append(cmds, loadAgentUpdateCmd(m.sessionStore, targetSessionID))
				}
				return m, tea.Batch(cmds...)
			}
		}

		// Check if any daemon is running at all (for helpful message)
		if m.runningDaemons != nil && len(m.runningDaemons) > 0 {
			// There are daemons running, but not for this session
			m.message = "Press W on a session with ▶ to monitor it"
		} else {
			m.message = "No agent running. Press 'A' on a session to start."
		}
		return m, nil

	case "y":
		// Copy ball ID to clipboard (in balls panel)
		if m.activePanel == BallsPanel {
			return m.handleCopyBallID()
		}
		return m, nil

	case "A":
		// Create followup ball (depends on current ball)
		if m.activePanel == BallsPanel {
			return m.handleSplitAddFollowup()
		}
		return m, nil
	}

	return m, nil
}

// handleSplitDeletePrompt shows delete confirmation
func (m Model) handleSplitDeletePrompt() (tea.Model, tea.Cmd) {
	switch m.activePanel {
	case SessionsPanel:
		sessions := m.filterSessions()
		if len(sessions) == 0 || m.sessionCursor >= len(sessions) {
			m.message = "No session selected"
			return m, nil
		}
		// Prevent deleting pseudo-sessions
		sess := sessions[m.sessionCursor]
		if sess.ID == PseudoSessionAll || sess.ID == PseudoSessionUntagged {
			m.message = "Cannot delete built-in session"
			return m, nil
		}
		m.confirmAction = "delete_session"
		m.mode = confirmSplitDelete
		m.addActivity("Confirming session deletion...")

	case BallsPanel:
		balls := m.filterBallsForSession()
		var ballsToDelete []*session.Ball

		if len(m.selectedBalls) > 0 {
			// Multi-select mode: collect all selected balls
			for _, ball := range balls {
				if m.selectedBalls[ball.ID] {
					ballsToDelete = append(ballsToDelete, ball)
				}
			}
		} else {
			// Single ball mode: operate on cursor ball
			if len(balls) == 0 || m.cursor >= len(balls) {
				m.message = "No ball selected"
				return m, nil
			}
			ballsToDelete = append(ballsToDelete, balls[m.cursor])
		}

		if len(ballsToDelete) == 0 {
			m.message = "No balls selected"
			return m, nil
		}

		m.pendingDeleteBalls = ballsToDelete
		m.confirmAction = "delete_ball"
		m.mode = confirmSplitDelete
		if len(ballsToDelete) == 1 {
			m.addActivity("Confirming deletion of ball: " + ballsToDelete[0].ID)
		} else {
			m.addActivity(fmt.Sprintf("Confirming deletion of %d balls...", len(ballsToDelete)))
		}
	}

	return m, nil
}

// handleSplitConfirmDelete handles yes/no for delete confirmation
func (m Model) handleSplitConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		return m.executeSplitDelete()
	case "n", "N", "esc":
		m.mode = splitView
		m.pendingDeleteBalls = nil // Clear pending delete
		m.message = "Cancelled"
		return m, nil
	}
	return m, nil
}

// executeSplitDelete performs the actual deletion
func (m Model) executeSplitDelete() (tea.Model, tea.Cmd) {
	switch m.confirmAction {
	case "delete_session":
		sessions := m.filterSessions()
		if m.sessionCursor >= len(sessions) {
			m.mode = splitView
			return m, nil
		}
		sess := sessions[m.sessionCursor]
		// Double-check we're not deleting a pseudo-session (shouldn't happen due to guard in handleSplitDeletePrompt)
		if sess.ID == PseudoSessionAll || sess.ID == PseudoSessionUntagged {
			m.message = "Cannot delete built-in session"
			m.mode = splitView
			return m, nil
		}
		err := m.sessionStore.DeleteSession(sess.ID)
		if err != nil {
			m.message = "Error deleting session: " + err.Error()
			m.mode = splitView
			return m, nil
		}
		m.addActivity("Deleted session: " + sess.ID)
		m.message = "Deleted session: " + sess.ID
		m.mode = splitView
		// Reset selection if we deleted the selected session
		if m.selectedSession != nil && m.selectedSession.ID == sess.ID {
			m.selectedSession = nil
		}
		return m, loadSessions(m.sessionStore, m.config, m.localOnly)

	case "delete_ball":
		// Use pendingDeleteBalls if available, otherwise fall back to cursor
		ballsToDelete := m.pendingDeleteBalls
		if len(ballsToDelete) == 0 {
			balls := m.filterBallsForSession()
			if m.cursor >= len(balls) {
				m.mode = splitView
				return m, nil
			}
			ballsToDelete = []*session.Ball{balls[m.cursor]}
		}

		// Delete all balls
		deletedCount := 0
		for _, ball := range ballsToDelete {
			store, err := session.NewStore(ball.WorkingDir)
			if err != nil {
				m.message = "Error: " + err.Error()
				m.mode = splitView
				return m, nil
			}
			err = store.DeleteBall(ball.ID)
			if err != nil {
				m.message = "Error deleting ball: " + err.Error()
				m.mode = splitView
				return m, nil
			}
			// Remove from selection
			delete(m.selectedBalls, ball.ID)
			deletedCount++
		}

		if deletedCount == 1 {
			m.addActivity("Deleted ball: " + ballsToDelete[0].ID)
			m.message = "Deleted ball: " + ballsToDelete[0].ID
		} else {
			m.addActivity(fmt.Sprintf("Deleted %d balls", deletedCount))
			m.message = fmt.Sprintf("Deleted %d balls", deletedCount)
		}

		// Clear state
		m.pendingDeleteBalls = nil
		m.selectedBalls = make(map[string]bool)
		m.mode = splitView
		return m, loadBalls(m.store, m.config, m.localOnly)
	}

	m.mode = splitView
	return m, nil
}

// handlePanelSearchStart initiates search/filter mode for the current panel
func (m Model) handlePanelSearchStart() (tea.Model, tea.Cmd) {
	m.textInput.Reset()
	m.textInput.Focus()

	switch m.activePanel {
	case SessionsPanel:
		m.textInput.Placeholder = "Filter sessions..."
	case BallsPanel:
		m.textInput.Placeholder = "Filter balls..."
	}

	// Pre-fill with current filter if any
	if m.panelSearchQuery != "" {
		m.textInput.SetValue(m.panelSearchQuery)
	}

	m.mode = panelSearchView
	m.addActivity("Search mode activated")
	return m, nil
}

// handlePanelSearchKey handles keyboard input in panel search mode
func (m Model) handlePanelSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel search, keep existing filter
		m.mode = splitView
		m.textInput.Blur()
		return m, nil

	case "enter":
		// Apply the filter
		value := strings.TrimSpace(m.textInput.Value())
		m.panelSearchQuery = value
		m.panelSearchActive = value != ""
		m.textInput.Blur()
		m.mode = splitView

		if m.panelSearchActive {
			m.addActivity("Filter applied: " + value)
			m.message = "Filter: " + value + " (Ctrl+U to clear)"
		} else {
			m.addActivity("Filter cleared")
			m.message = "Filter cleared"
		}
		return m, nil

	default:
		// Pass to textinput
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

// filterSessions returns sessions filtered by the panel search query
// It prepends pseudo-sessions ("All" and "Untagged") at the top
func (m *Model) filterSessions() []*session.JuggleSession {
	// Create pseudo-sessions
	pseudoSessions := []*session.JuggleSession{
		{ID: PseudoSessionAll, Description: "All balls across all sessions"},
		{ID: PseudoSessionUntagged, Description: "Balls with no session tags"},
	}

	// Combine pseudo-sessions with real sessions
	allSessions := make([]*session.JuggleSession, 0, len(pseudoSessions)+len(m.sessions))
	allSessions = append(allSessions, pseudoSessions...)
	allSessions = append(allSessions, m.sessions...)

	if !m.panelSearchActive || m.panelSearchQuery == "" {
		return allSessions
	}

	query := strings.ToLower(m.panelSearchQuery)
	filtered := make([]*session.JuggleSession, 0)
	for _, sess := range allSessions {
		if strings.Contains(strings.ToLower(sess.ID), query) ||
			strings.Contains(strings.ToLower(sess.Description), query) {
			filtered = append(filtered, sess)
		}
	}
	return filtered
}

// filterBallsForSession returns balls filtered by session and search query, sorted by current sort order
func (m *Model) filterBallsForSession() []*session.Ball {
	balls := m.getBallsForSession()

	var result []*session.Ball
	if !m.panelSearchActive || m.panelSearchQuery == "" {
		result = balls
	} else {
		query := strings.ToLower(m.panelSearchQuery)
		filtered := make([]*session.Ball, 0)
		for _, ball := range balls {
			if strings.Contains(strings.ToLower(ball.Title), query) ||
				strings.Contains(strings.ToLower(ball.ID), query) {
				filtered = append(filtered, ball)
			}
		}
		result = filtered
	}

	// Apply sorting
	m.sortBalls(result)
	return result
}

// sortBalls sorts a slice of balls according to the current sort order
func (m *Model) sortBalls(balls []*session.Ball) {
	switch m.sortOrder {
	case SortByIDASC:
		sort.Slice(balls, func(i, j int) bool {
			return compareBallIDs(balls[i].ID, balls[j].ID) < 0
		})
	case SortByIDDESC:
		sort.Slice(balls, func(i, j int) bool {
			return compareBallIDs(balls[i].ID, balls[j].ID) > 0
		})
	case SortByPriorityDESC:
		sort.Slice(balls, func(i, j int) bool {
			// Higher priority first
			if balls[i].PriorityWeight() != balls[j].PriorityWeight() {
				return balls[i].PriorityWeight() > balls[j].PriorityWeight()
			}
			// Then by ID ascending
			return compareBallIDs(balls[i].ID, balls[j].ID) < 0
		})
	case SortByPriorityASC:
		sort.Slice(balls, func(i, j int) bool {
			// Lower priority first
			if balls[i].PriorityWeight() != balls[j].PriorityWeight() {
				return balls[i].PriorityWeight() < balls[j].PriorityWeight()
			}
			// Then by ID ascending
			return compareBallIDs(balls[i].ID, balls[j].ID) < 0
		})
	case SortByLastActivityDESC:
		sort.Slice(balls, func(i, j int) bool {
			// More recent first
			return balls[i].LastActivity.After(balls[j].LastActivity)
		})
	case SortByLastActivityASC:
		sort.Slice(balls, func(i, j int) bool {
			// Older activity first
			return balls[i].LastActivity.Before(balls[j].LastActivity)
		})
	case SortByCreatedAtDESC:
		sort.Slice(balls, func(i, j int) bool {
			// Newer creation time first (StartedAt is set at creation time)
			return balls[i].StartedAt.After(balls[j].StartedAt)
		})
	case SortByCreatedAtASC:
		sort.Slice(balls, func(i, j int) bool {
			// Older creation time first
			return balls[i].StartedAt.Before(balls[j].StartedAt)
		})
	}
}

// compareBallIDs compares two ball IDs numerically
// IDs are in format "project-N" where N is a number
func compareBallIDs(id1, id2 string) int {
	// Extract numeric parts for comparison
	num1 := extractBallNumber(id1)
	num2 := extractBallNumber(id2)

	// If both have numeric parts, compare numerically first
	if num1 != -1 && num2 != -1 {
		if num1 < num2 {
			return -1
		} else if num1 > num2 {
			return 1
		}
		// Numbers are equal, fall through to string comparison to compare prefixes
	}

	// Fall back to string comparison (also handles same numbers with different prefixes)
	if id1 < id2 {
		return -1
	} else if id1 > id2 {
		return 1
	}
	return 0
}

// extractBallNumber extracts the numeric suffix from a ball ID
// Returns -1 if no numeric suffix is found
func extractBallNumber(id string) int {
	lastHyphen := strings.LastIndex(id, "-")
	if lastHyphen >= 0 && lastHyphen < len(id)-1 {
		numStr := id[lastHyphen+1:]
		num, err := strconv.Atoi(numStr)
		if err == nil {
			return num
		}
	}
	return -1
}

// handleWatcherEvent handles file system change events
func (m Model) handleWatcherEvent(event watcher.Event) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch event.Type {
	case watcher.BallsChanged:
		m.addActivity("File changed: balls.jsonl - reloading...")
		cmds = append(cmds, loadBalls(m.store, m.config, m.localOnly))

	case watcher.SessionChanged:
		msg := "Session file changed"
		if event.SessionID != "" {
			msg += ": " + event.SessionID
		}
		m.addActivity(msg + " - reloading...")
		cmds = append(cmds, loadSessions(m.sessionStore, m.config, m.localOnly))

	case watcher.ProgressChanged:
		msg := "Progress updated"
		if event.SessionID != "" {
			msg += " for session: " + event.SessionID
		}
		m.addActivity(msg)
		// Progress changes don't require reloading UI data,
		// but log it for awareness

	case watcher.AgentStateChanged:
		// Daemon state file changed - update monitor view if active
		if event.SessionID != "" && event.SessionID == m.agentStatus.SessionID {
			// Load the updated daemon state
			cmds = append(cmds, loadDaemonStateCmd(m.store.ProjectDir(), event.SessionID))
		}

	case watcher.AgentUpdateChanged:
		// Agent loop update file changed - update phase display if in monitor view
		if event.SessionID != "" && event.SessionID == m.agentStatus.SessionID {
			// Load the updated agent phase info
			cmds = append(cmds, loadAgentUpdateCmd(m.sessionStore, event.SessionID))
		}

	case watcher.AgentMetricsChanged:
		// Agent metrics file changed - update metrics display if in monitor view
		if event.SessionID != "" && event.SessionID == m.agentStatus.SessionID {
			// Load the updated agent metrics
			cmds = append(cmds, loadAgentMetricsCmd(m.sessionStore, event.SessionID))
		}
	}

	// Continue listening for more events
	if m.fileWatcher != nil {
		cmds = append(cmds, listenForWatcherEvents(m.fileWatcher))
	}

	return m, tea.Batch(cmds...)
}

// handleSplitHelpKey handles keyboard input in split help view
func (m Model) handleSplitHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "?", "esc":
		// Close help view
		m.mode = splitView
		return m, nil

	case "j", "down":
		// Scroll down
		m.helpScrollOffset++
		return m, nil

	case "k", "up":
		// Scroll up
		if m.helpScrollOffset > 0 {
			m.helpScrollOffset--
		}
		return m, nil

	case "ctrl+d":
		// Page down
		m.helpScrollOffset += 10
		return m, nil

	case "ctrl+u":
		// Page up
		m.helpScrollOffset -= 10
		if m.helpScrollOffset < 0 {
			m.helpScrollOffset = 0
		}
		return m, nil

	case "g":
		// Handle gg for go to top
		if m.lastKey == "g" {
			m.lastKey = ""
			m.helpScrollOffset = 0
			return m, nil
		}
		m.lastKey = "g"
		return m, nil

	case "G":
		// Go to bottom (set to large number, will be clamped in render)
		m.lastKey = ""
		m.helpScrollOffset = 1000 // Large number, will be clamped
		return m, nil
	}

	// Reset gg detection for any other key
	m.lastKey = ""
	return m, nil
}
