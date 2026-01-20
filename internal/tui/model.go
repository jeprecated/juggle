package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/ohare93/juggle/internal/session"
	"github.com/ohare93/juggle/internal/watcher"
)

type viewMode int

const (
	splitView viewMode = iota // Three-panel split view (default)
	splitHelpView             // Comprehensive help view for split mode
	historyView               // Agent run history view
	agentMonitorView          // Full-screen agent monitoring dashboard

	// Input modes for CRUD operations
	inputSessionView           // Add/edit session
	inputBallView              // Add/edit ball (for title field)
	inputBlockedView           // Prompt for blocked reason
	inputTagView               // Add/remove tags
	sessionSelectorView        // Session selector for tagging balls
	dependencySelectorView     // Dependency selector for ball creation/editing
	confirmSplitDelete         // Delete confirmation in split view
	panelSearchView            // Search/filter within current panel
	confirmAgentCancel         // Agent cancel confirmation
	unifiedBallFormView        // Unified ball creation form - all fields in one view
	historyOutputView          // Viewing last_output.txt from history
)

// InputAction represents what action triggered the input mode
type InputAction int

const (
	actionAdd InputAction = iota
	actionEdit
)

// TagEditMode represents whether we're adding or removing a tag
type TagEditMode int

const (
	tagModeAdd TagEditMode = iota
	tagModeRemove
)

// Panel represents which panel is active in split view
type Panel int

const (
	SessionsPanel Panel = iota
	BallsPanel
	ActivityPanel
)

// BottomPaneMode represents what the bottom pane displays
type BottomPaneMode int

const (
	BottomPaneActivity BottomPaneMode = iota // Show activity log
	BottomPaneDetail                         // Show highlighted ball details
	BottomPaneSplit                          // Show both activity and details side by side
	BottomPaneUpdates                        // Show updates history (hook events + phase changes)
)

// SortOrder represents how balls are sorted
type SortOrder int

const (
	SortByIDASC           SortOrder = iota // Sort by ID ascending (default)
	SortByIDDESC                           // Sort by ID descending
	SortByPriorityDESC                     // Sort by priority descending (urgent first)
	SortByPriorityASC                      // Sort by priority ascending (low first)
	SortByLastActivityDESC                 // Sort by last activity descending (most recent first)
	SortByLastActivityASC                  // Sort by last activity ascending (oldest activity first)
	SortByCreatedAtDESC                    // Sort by creation time descending (newest first)
	SortByCreatedAtASC                     // Sort by creation time ascending (oldest first)
)

// Special pseudo-session IDs
const (
	PseudoSessionAll      = "__all__"
	PseudoSessionUntagged = "__untagged__"
)

// ActivityEntry represents a log entry in the activity log
type ActivityEntry struct {
	Time    time.Time
	Message string
}

// AgentOutputEntry represents a line of agent output
type AgentOutputEntry struct {
	Time    time.Time
	Line    string
	IsError bool // true if this is stderr output
}

type Model struct {
	store         *session.Store
	sessionStore  *session.SessionStore
	config        *session.Config
	localOnly     bool // restrict to local project only
	balls         []*session.Ball
	filteredBalls []*session.Ball

	// Session state (for split view)
	sessions        []*session.JuggleSession
	selectedSession *session.JuggleSession
	sessionCursor   int

	// View state
	mode   viewMode
	cursor int

	// Multi-select state for balls
	selectedBalls map[string]bool // Ball IDs that are currently selected (multi-select with Space)

	// Panel state (for split view)
	activePanel Panel

	// Activity log
	activityLog        []ActivityEntry
	activityLogOffset  int    // Scroll offset for activity log
	lastKey            string // Last key pressed (for gg detection)
	pendingKeySequence string // Pending key for two-key sequences (s, t, etc.)
	helpScrollOffset   int    // Scroll offset for help view
	ballsScrollOffset  int    // Scroll offset for balls panel viewport
	detailScrollOffset int    // Scroll offset for ball detail panel

	// Bottom pane mode (toggle between activity log and ball detail)
	bottomPaneMode BottomPaneMode

	// Sort order for balls
	sortOrder SortOrder

	// Column visibility for balls panel
	showPriorityColumn  bool // Show priority column in balls list
	showTagsColumn      bool // Show tags column in balls list
	showModelSizeColumn bool // Show model size column in balls list

	// Filter state
	filterStates         map[string]bool // State visibility toggles
	filterPriority       string
	searchQuery          string
	initialSessionID     string // Pre-select session by ID (from --session flag)
	panelSearchQuery     string // Current search/filter query within a panel
	panelSearchActive    bool   // Whether search/filter is active
	pendingSessionSelect string // Session ID to restore after mode switch

	// UI state
	width         int
	height        int
	message       string // Success/error messages
	err           error
	confirmAction string // What action is being confirmed (e.g., "delete")

	// Input state for CRUD operations
	textInput          textinput.Model
	contextInput       textarea.Model   // Multiline text input for context field
	inputAction        InputAction      // Add or Edit
	inputTarget        string           // What we're editing (e.g., "intent", "description")
	editingBall        *session.Ball            // Ball being edited (for edit action)
	pendingBlockBalls  []*session.Ball          // Balls waiting to be blocked (for multi-select block)
	pendingDeleteBalls []*session.Ball          // Balls waiting to be deleted (for multi-select delete)
	editingSession     *session.JuggleSession   // Session being edited (for edit action)
	tagEditMode           TagEditMode               // Whether adding or removing a tag
	sessionSelectItems    []*session.JuggleSession  // Sessions available for selection
	sessionSelectIndex    int                       // Current selection index in session selector
	sessionSelectActive   map[string]bool           // Which sessions are currently selected (multi-select)

	// Pending ball creation state (for unified ball creation form)
	pendingBallContext         string   // Context being created (first field)
	pendingBallIntent          string   // Title being created (was intent)
	pendingBallPriority        int      // Index in priority options (0=low, 1=medium, 2=high, 3=urgent)
	pendingBallTags            string   // Comma-separated tags
	pendingBallSession         int      // Index in session options (0=none, 1+ = session index)
	pendingBallModelSize       int      // Index in model size options (0=default, 1=small, 2=medium, 3=large)
	pendingBallAgentProvider   int      // Index in agent provider options (0=default, 1=claude, 2=opencode)
	pendingBallModelOverride   int      // Index in model override options (0=default, 1=opus, 2=sonnet, 3=haiku)
	pendingBallDependsOn       []string // Selected dependency ball IDs
	pendingBallBlockingReason  int      // Index in blocking reason options (0=blank, 1=Human needed, 2=Waiting for dependency, 3=Needs research, 4=custom)
	pendingBallCustomReason    string   // Custom blocking reason text (when pendingBallBlockingReason == 4)
	pendingBallFormField       int      // Current field in form (0=context, 1=title, 2+=ACs, then tags, session, model_size, priority, blocking_reason, depends_on, save)
	pendingAcceptanceCriteria  []string // Acceptance criteria being collected
	pendingACEditIndex         int      // Index of AC being edited (-1 = adding new, >= 0 = editing existing)
	pendingNewAC               string   // Content of the "new AC" field, preserved during navigation

	// AC Templates and repo/session level ACs (for ball creation form)
	acTemplates           []string // Selectable AC templates from project config
	acTemplateSelected    []bool   // Which templates are currently selected (added to ACs)
	acTemplateCursor      int      // Current cursor position in templates list (-1 = not on templates)
	repoLevelACs          []string // Repo-level ACs shown as reminders (not stored on ball)
	sessionLevelACs       []string // Session-level ACs shown as reminders (not stored on ball)

	// File autocomplete state for ball form
	fileAutocomplete *AutocompleteState // File path autocomplete suggestions

	// Dependency selector state
	dependencySelectBalls  []*session.Ball // Non-complete balls available for selection
	dependencySelectIndex  int             // Current selection index in dependency selector
	dependencySelectActive map[string]bool // Which dependencies are currently selected (by ID)

	// File watcher
	fileWatcher *watcher.Watcher

	// Agent state
	agentStatus AgentStatus // Status of running agent

	// Running daemons across all sessions (discovered on startup and updated via file watcher)
	runningDaemons map[string]*DaemonInfo // Map of sessionID -> daemon info

	// Agent output panel state
	agentOutputVisible  bool               // Whether agent output panel is shown
	agentOutputExpanded bool               // Whether agent output panel is expanded (half screen)
	agentOutput         []AgentOutputEntry // Buffer of agent output lines
	agentOutputOffset   int                // Scroll offset for agent output panel
	agentOutputCh       chan agentOutputMsg // Channel for receiving agent output

	// Agent process tracking for cancellation
	agentProcess *AgentProcess // Reference to running agent process for cancellation

	// Exit action - signals to caller what to do after TUI exits
	runAgentForBall string // Ball ID to run agent for after TUI exits (empty = no action)

	// Agent history state
	agentHistory        []*session.AgentRunRecord // Loaded agent run history
	historyCursor       int                       // Current selection in history view
	historyScrollOffset int                       // Scroll offset for history view
	historyOutput       string                    // Content of selected history's output file
	historyOutputOffset int                       // Scroll offset for output view

	// Agent monitor state
	agentMonitorPaused      bool            // Whether pause-on-next-iteration is pending
	agentMonitorReconnected bool            // True if reconnected to existing daemon
	agentMonitorStartTime   time.Time       // When the current agent run started
	agentSpinner            spinner.Model   // Spinner for agent running animation
	agentLogTailer          *LogTailer      // Log file tailer for streaming agent output
	agentDaemonError        string          // Error message from daemon (displayed prominently)
	agentMetrics            *AgentMetricsState // Hook-provided metrics (files changed, tool counts, tokens)

	// Updates history state (from hooks and agent loop updates)
	updatesHistory       []*session.UpdateEntry // All update entries from updates.jsonl
	updatesHistoryOffset int                    // Scroll offset for updates history view
	updatesTypeFilters   map[session.UpdateType]bool // Filter toggles for update types (true = visible)

	// Time provider for testability
	nowFunc func() time.Time // Can be overridden in tests
}

// newAgentSpinner creates a spinner for the agent monitor view
func newAgentSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return s
}

// newContextTextarea creates a textarea for the context field with appropriate settings
func newContextTextarea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Background context for this task"
	ta.CharLimit = 2000 // Allow longer context
	ta.SetWidth(60)
	ta.SetHeight(1) // Start with 1 line, will grow dynamically
	ta.ShowLineNumbers = false
	return ta
}

// InitialSplitModel creates a model for the split-view mode
func InitialSplitModel(store *session.Store, sessionStore *session.SessionStore, config *session.Config, localOnly bool) Model {
	return InitialSplitModelWithWatcher(store, sessionStore, config, localOnly, nil, "")
}

// InitialSplitModelWithWatcher creates a model for the new split-view mode with file watching
func InitialSplitModelWithWatcher(store *session.Store, sessionStore *session.SessionStore, config *session.Config, localOnly bool, w *watcher.Watcher, initialSessionID string) Model {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 40

	return Model{
		store:            store,
		sessionStore:     sessionStore,
		config:           config,
		localOnly:        localOnly,
		mode:             splitView,
		activePanel:      BallsPanel,
		initialSessionID: initialSessionID,
		filterStates: map[string]bool{
			"pending":     true,
			"in_progress": true,
			"blocked":     true,
			"complete":    false, // Hidden by default
		},
		// Column visibility defaults (all hidden by default for compact view)
		showPriorityColumn:  false,
		showTagsColumn:      false,
		showModelSizeColumn: false,
		cursor:              0,
		selectedBalls:       make(map[string]bool),
		sessionCursor:       0,
		activityLog:         make([]ActivityEntry, 0),
		// Updates history filter defaults (all visible by default)
		updatesTypeFilters: map[session.UpdateType]bool{
			session.UpdateTypePhase:       true,
			session.UpdateTypeToolUse:     true,
			session.UpdateTypeToolFailure: true,
			session.UpdateTypeStop:        true,
			session.UpdateTypeSessionEnd:  true,
		},
		textInput:           ti,
		contextInput:        newContextTextarea(),
		fileWatcher:         w,
		nowFunc:             time.Now,
		agentSpinner:        newAgentSpinner(),
		runningDaemons:      make(map[string]*DaemonInfo),
	}
}

// InitialMonitorModel creates a model that starts directly in agent monitor view
// This is used when launching from `juggle agent run --monitor`
func InitialMonitorModel(store *session.Store, sessionStore *session.SessionStore, config *session.Config, localOnly bool, w *watcher.Watcher, sessionID string, daemonRunning bool) Model {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 40

	return Model{
		store:            store,
		sessionStore:     sessionStore,
		config:           config,
		localOnly:        localOnly,
		mode:             agentMonitorView,
		activePanel:      BallsPanel,
		initialSessionID: sessionID,
		filterStates: map[string]bool{
			"pending":     true,
			"in_progress": true,
			"blocked":     true,
			"complete":    false,
		},
		showPriorityColumn:  false,
		showTagsColumn:      false,
		showModelSizeColumn: false,
		cursor:              0,
		selectedBalls:       make(map[string]bool),
		sessionCursor:       0,
		activityLog:         make([]ActivityEntry, 0),
		// Updates history filter defaults (all visible by default)
		updatesTypeFilters: map[session.UpdateType]bool{
			session.UpdateTypePhase:       true,
			session.UpdateTypeToolUse:     true,
			session.UpdateTypeToolFailure: true,
			session.UpdateTypeStop:        true,
			session.UpdateTypeSessionEnd:  true,
		},
		textInput:           ti,
		contextInput:        newContextTextarea(),
		fileWatcher:         w,
		nowFunc:             time.Now,
		agentSpinner:        newAgentSpinner(),
		runningDaemons:      make(map[string]*DaemonInfo),
		// Set agent status so monitor view knows what to display
		agentStatus: AgentStatus{
			Running:   daemonRunning,
			SessionID: sessionID,
		},
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		loadBalls(m.store, m.config, m.localOnly),
		loadSessions(m.sessionStore, m.config, m.localOnly),
	}
	// Start file watcher if available
	if m.fileWatcher != nil {
		cmds = append(cmds, listenForWatcherEvents(m.fileWatcher))
	}
	// If starting in monitor mode with a session, load daemon state, start spinner, and start log tail
	// true = starting in monitor mode means reconnecting to existing session, read existing content
	if m.mode == agentMonitorView && m.agentStatus.SessionID != "" && m.store != nil {
		cmds = append(cmds, loadDaemonStateCmd(m.store.ProjectDir(), m.agentStatus.SessionID))
		cmds = append(cmds, m.agentSpinner.Tick)
		cmds = append(cmds, startLogTailCmd(m.store.ProjectDir(), m.agentStatus.SessionID, true))
		// Also load agent update for phase info, metrics, and updates history
		if m.sessionStore != nil {
			cmds = append(cmds, loadAgentUpdateCmd(m.sessionStore, m.agentStatus.SessionID))
			cmds = append(cmds, loadAgentMetricsCmd(m.sessionStore, m.agentStatus.SessionID))
			cmds = append(cmds, loadUpdatesHistoryCmd(m.sessionStore, m.agentStatus.SessionID))
		}
	}
	return tea.Batch(cmds...)
}

// addActivity adds an entry to the activity log
func (m *Model) addActivity(msg string) {
	nowTime := time.Now()
	if m.nowFunc != nil {
		nowTime = m.nowFunc()
	}
	entry := ActivityEntry{
		Time:    nowTime,
		Message: msg,
	}
	// Keep last 100 entries
	if len(m.activityLog) >= 100 {
		m.activityLog = m.activityLog[1:]
		// Adjust offset when we remove an entry
		if m.activityLogOffset > 0 {
			m.activityLogOffset--
		}
	}
	m.activityLog = append(m.activityLog, entry)

	// Auto-scroll to bottom unless actively viewing the activity panel
	// (user might be scrolled up to read history)
	if m.activePanel != ActivityPanel {
		m.activityLogOffset = m.getActivityLogMaxOffset()
	}
}

// SelectedSessionID returns the ID of the currently selected session (if any)
func (m Model) SelectedSessionID() string {
	if m.selectedSession != nil {
		return m.selectedSession.ID
	}
	return ""
}

// RunAgentForBall returns the ball ID to run the agent for after TUI exits.
// Returns empty string if no agent run is requested.
func (m Model) RunAgentForBall() string {
	return m.runAgentForBall
}

// addAgentOutput adds a line to the agent output buffer
func (m *Model) addAgentOutput(line string, isError bool) {
	entry := AgentOutputEntry{
		Time:    time.Now(),
		Line:    line,
		IsError: isError,
	}
	// Keep last 500 lines
	if len(m.agentOutput) >= 500 {
		m.agentOutput = m.agentOutput[1:]
		// Adjust offset when we remove an entry
		if m.agentOutputOffset > 0 {
			m.agentOutputOffset--
		}
	}
	m.agentOutput = append(m.agentOutput, entry)

	// Auto-scroll to bottom when new output arrives
	m.agentOutputOffset = m.getAgentOutputMaxOffset()
}

// clearAgentOutput clears the agent output buffer
func (m *Model) clearAgentOutput() {
	m.agentOutput = make([]AgentOutputEntry, 0)
	m.agentOutputOffset = 0
}

// getAgentOutputMaxOffset returns the maximum scroll offset for the agent output panel
func (m Model) getAgentOutputMaxOffset() int {
	visibleLines := m.getAgentOutputVisibleLines()
	maxOffset := len(m.agentOutput) - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	return maxOffset
}

// getUpdatesHistoryMaxOffset returns the maximum scroll offset for updates history
func (m Model) getUpdatesHistoryMaxOffset() int {
	visibleLines := m.getAgentOutputVisibleLines() // Reuse same height calculation
	filtered := m.getFilteredUpdates()
	maxOffset := len(filtered) - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	return maxOffset
}

// getFilteredUpdates returns updates filtered by the type filters
func (m Model) getFilteredUpdates() []*session.UpdateEntry {
	if m.updatesTypeFilters == nil {
		return m.updatesHistory
	}
	var filtered []*session.UpdateEntry
	for _, u := range m.updatesHistory {
		if m.updatesTypeFilters[u.Type] {
			filtered = append(filtered, u)
		}
	}
	return filtered
}

// getAgentOutputVisibleLines returns the number of visible lines in the agent output panel
func (m Model) getAgentOutputVisibleLines() int {
	// Agent output panel takes up the right portion of the screen
	height := m.height - 6 // Account for borders, title, status
	if height < 3 {
		height = 3
	}
	return height
}

// getBallsForSession returns balls that belong to the selected session (by tag)
func (m *Model) getBallsForSession() []*session.Ball {
	if m.selectedSession == nil {
		return m.filteredBalls
	}

	// Handle pseudo-sessions
	switch m.selectedSession.ID {
	case PseudoSessionAll:
		// Return all balls
		return m.filteredBalls
	case PseudoSessionUntagged:
		// Return balls with no session tags (no tags that match any real session)
		untaggedBalls := make([]*session.Ball, 0)
		sessionIDs := make(map[string]bool)
		for _, sess := range m.sessions {
			sessionIDs[sess.ID] = true
		}
		for _, ball := range m.filteredBalls {
			hasSessionTag := false
			for _, tag := range ball.Tags {
				if sessionIDs[tag] {
					hasSessionTag = true
					break
				}
			}
			if !hasSessionTag {
				untaggedBalls = append(untaggedBalls, ball)
			}
		}
		return untaggedBalls
	default:
		// Regular session - return balls with matching tag
		sessionBalls := make([]*session.Ball, 0)
		for _, ball := range m.filteredBalls {
			for _, tag := range ball.Tags {
				if tag == m.selectedSession.ID {
					sessionBalls = append(sessionBalls, ball)
					break
				}
			}
		}
		return sessionBalls
	}
}
