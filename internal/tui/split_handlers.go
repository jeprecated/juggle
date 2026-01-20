package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ohare93/juggle/internal/session"
)

// handleStateKeySequence handles the second key in a state change sequence (s+key)
func (m Model) handleStateKeySequence(key string) (tea.Model, tea.Cmd) {
	m.message = ""

	if m.activePanel != BallsPanel {
		return m, nil
	}

	switch key {
	case "c":
		// sc = Complete ball
		return m.handleSplitCompleteBall()
	case "s":
		// ss = Start ball (set to in_progress)
		return m.handleSplitStartBall()
	case "b":
		// sb = Block ball
		return m.handleSplitBlockBall()
	case "p":
		// sp = Set to pending
		return m.handleSplitSetPending()
	case "a":
		// sa = Archive completed ball
		return m.handleSplitArchiveBall()
	case "esc":
		// Cancel sequence
		m.message = ""
		return m, nil
	default:
		m.message = "Unknown state: " + key + " (use c/s/b/p/a)"
		return m, nil
	}
}

// handleToggleKeySequence handles the second key in a toggle sequence (t+key)
func (m Model) handleToggleKeySequence(key string) (tea.Model, tea.Cmd) {
	m.message = ""

	// Track if we need to apply filters (for all filter-changing keys)
	needsFilterUpdate := true

	switch key {
	case "c":
		// tc = Toggle complete visibility
		m.filterStates["complete"] = !m.filterStates["complete"]
		if m.filterStates["complete"] {
			m.addActivity("Showing complete balls")
			m.message = "Complete: visible"
		} else {
			m.addActivity("Hiding complete balls")
			m.message = "Complete: hidden"
		}
	case "b":
		// tb = Toggle blocked visibility
		m.filterStates["blocked"] = !m.filterStates["blocked"]
		if m.filterStates["blocked"] {
			m.addActivity("Showing blocked balls")
			m.message = "Blocked: visible"
		} else {
			m.addActivity("Hiding blocked balls")
			m.message = "Blocked: hidden"
		}
	case "i":
		// ti = Toggle in_progress visibility
		m.filterStates["in_progress"] = !m.filterStates["in_progress"]
		if m.filterStates["in_progress"] {
			m.addActivity("Showing in-progress balls")
			m.message = "In-progress: visible"
		} else {
			m.addActivity("Hiding in-progress balls")
			m.message = "In-progress: hidden"
		}
	case "p":
		// tp = Toggle pending visibility
		m.filterStates["pending"] = !m.filterStates["pending"]
		if m.filterStates["pending"] {
			m.addActivity("Showing pending balls")
			m.message = "Pending: visible"
		} else {
			m.addActivity("Hiding pending balls")
			m.message = "Pending: hidden"
		}
	case "a":
		// ta = Show all states
		m.filterStates["pending"] = true
		m.filterStates["in_progress"] = true
		m.filterStates["blocked"] = true
		m.filterStates["complete"] = true
		m.addActivity("Showing all states")
		m.message = "All states visible"
	case "esc":
		// Cancel sequence
		m.message = ""
		needsFilterUpdate = false
	default:
		m.message = "Unknown toggle: " + key + " (use c/b/i/p/h/a)"
		needsFilterUpdate = false
	}

	// Apply filters and reset cursor for filter-changing operations
	if needsFilterUpdate {
		m.applyFilters()
		// Reset cursor if it's out of bounds
		if m.cursor >= len(m.filteredBalls) {
			m.cursor = 0
		}
	}

	return m, nil
}

// handleViewColumnKeySequence handles the second key in a view column sequence (v+key)
func (m Model) handleViewColumnKeySequence(key string) (tea.Model, tea.Cmd) {
	m.message = ""

	switch key {
	case "p":
		// vp = Toggle priority column visibility
		m.showPriorityColumn = !m.showPriorityColumn
		if m.showPriorityColumn {
			m.addActivity("Showing priority column")
			m.message = "Priority column: visible"
		} else {
			m.addActivity("Hiding priority column")
			m.message = "Priority column: hidden"
		}
		return m, nil
	case "t":
		// vt = Toggle tags column visibility
		m.showTagsColumn = !m.showTagsColumn
		if m.showTagsColumn {
			m.addActivity("Showing tags column")
			m.message = "Tags column: visible"
		} else {
			m.addActivity("Hiding tags column")
			m.message = "Tags column: hidden"
		}
		return m, nil
	case "m":
		// vm = Toggle model size column visibility
		m.showModelSizeColumn = !m.showModelSizeColumn
		if m.showModelSizeColumn {
			m.addActivity("Showing model size column")
			m.message = "Model size column: visible"
		} else {
			m.addActivity("Hiding model size column")
			m.message = "Model size column: hidden"
		}
		return m, nil
	case "a":
		// va = Toggle all columns visibility
		allVisible := m.showPriorityColumn && m.showTagsColumn && m.showModelSizeColumn
		if allVisible {
			// Hide all
			m.showPriorityColumn = false
			m.showTagsColumn = false
			m.showModelSizeColumn = false
			m.addActivity("Hiding all optional columns")
			m.message = "All columns: hidden"
		} else {
			// Show all
			m.showPriorityColumn = true
			m.showTagsColumn = true
			m.showModelSizeColumn = true
			m.addActivity("Showing all optional columns")
			m.message = "All columns: visible"
		}
		return m, nil
	case "esc":
		// Cancel sequence
		m.message = ""
		return m, nil
	default:
		m.message = "Unknown view column: " + key + " (use p/t/m/a)"
		return m, nil
	}
}

// handleSplitSetPending sets the selected ball(s) to pending state
// Supports multi-select: if balls are selected, operates on all selected balls.
func (m Model) handleSplitSetPending() (tea.Model, tea.Cmd) {
	balls := m.filterBallsForSession()
	var ballsToSet []*session.Ball

	if len(m.selectedBalls) > 0 {
		// Multi-select mode: operate on all selected balls
		for _, ball := range balls {
			if m.selectedBalls[ball.ID] {
				ballsToSet = append(ballsToSet, ball)
			}
		}
	} else {
		// Single ball mode: operate on cursor ball
		if len(balls) == 0 || m.cursor >= len(balls) {
			return m, nil
		}
		ballsToSet = append(ballsToSet, balls[m.cursor])
	}

	if len(ballsToSet) == 0 {
		return m, nil
	}

	var cmds []tea.Cmd
	for _, ball := range ballsToSet {
		if err := ball.SetState(session.StatePending); err != nil {
			m.message = "Error: " + err.Error()
			return m, nil
		}

		store, err := session.NewStore(ball.WorkingDir)
		if err != nil {
			m.message = "Error: " + err.Error()
			return m, nil
		}
		cmds = append(cmds, updateBall(store, ball))
	}

	if len(ballsToSet) == 1 {
		m.addActivity("Set pending: " + ballsToSet[0].ID)
	} else {
		m.addActivity(fmt.Sprintf("Set %d balls to pending", len(ballsToSet)))
	}

	// Clear multi-select after operation
	m.selectedBalls = make(map[string]bool)

	return m, tea.Batch(cmds...)
}

// handleSplitArchiveBall archives completed ball(s)
// Supports multi-select: if balls are selected, archives all selected completed balls.
func (m Model) handleSplitArchiveBall() (tea.Model, tea.Cmd) {
	balls := m.filterBallsForSession()
	var ballsToArchive []*session.Ball

	if len(m.selectedBalls) > 0 {
		// Multi-select mode: collect all selected completed balls
		for _, ball := range balls {
			if m.selectedBalls[ball.ID] {
				if ball.State == session.StateComplete {
					ballsToArchive = append(ballsToArchive, ball)
				}
			}
		}
		if len(ballsToArchive) == 0 {
			m.message = "No completed balls selected (only completed balls can be archived)"
			m.selectedBalls = make(map[string]bool) // Clear invalid selection
			return m, nil
		}
	} else {
		// Single ball mode: operate on cursor ball
		if len(balls) == 0 || m.cursor >= len(balls) {
			return m, nil
		}
		ball := balls[m.cursor]
		// Only archive completed balls
		if ball.State != session.StateComplete {
			m.message = "Can only archive completed balls (use sc first)"
			return m, nil
		}
		ballsToArchive = append(ballsToArchive, ball)
	}

	var cmds []tea.Cmd
	for _, ball := range ballsToArchive {
		store, err := session.NewStore(ball.WorkingDir)
		if err != nil {
			m.message = "Error: " + err.Error()
			return m, nil
		}
		cmds = append(cmds, archiveBall(store, ball))
	}

	if len(ballsToArchive) == 1 {
		m.addActivity("Archiving ball: " + ballsToArchive[0].ID)
	} else {
		m.addActivity(fmt.Sprintf("Archiving %d balls", len(ballsToArchive)))
	}

	// Clear multi-select after operation
	m.selectedBalls = make(map[string]bool)

	return m, tea.Batch(cmds...)
}

// handleSplitViewNavUp handles up navigation in split view
func (m Model) handleSplitViewNavUp() (tea.Model, tea.Cmd) {
	m.lastKey = "" // Clear gg state
	switch m.activePanel {
	case SessionsPanel:
		sessions := m.filterSessions()
		if m.sessionCursor > 0 && m.sessionCursor < len(sessions) {
			m.sessionCursor--
			// Scroll-to-select: automatically select the session when navigating
			m.selectedSession = sessions[m.sessionCursor]
			m.cursor = 0 // Reset ball cursor for new session
			m.ballsScrollOffset = 0 // Reset balls scroll offset for new session
			m.selectedBalls = make(map[string]bool) // Clear multi-select on session change
		} else if m.sessionCursor >= len(sessions) && len(sessions) > 0 {
			m.sessionCursor = len(sessions) - 1
			m.selectedSession = sessions[m.sessionCursor]
			m.cursor = 0
			m.ballsScrollOffset = 0
			m.selectedBalls = make(map[string]bool) // Clear multi-select on session change
		}
	case BallsPanel:
		if m.cursor > 0 {
			m.cursor--
			// Adjust scroll offset to keep cursor visible
			balls := m.filterBallsForSession()
			m.adjustBallsScrollOffset(balls)
		}
	case ActivityPanel:
		// Scroll based on bottom pane mode
		if m.bottomPaneMode == BottomPaneDetail {
			// Scroll up in detail view
			if m.detailScrollOffset > 0 {
				m.detailScrollOffset--
			}
		} else {
			// Scroll up one line in activity log
			if m.activityLogOffset > 0 {
				m.activityLogOffset--
			}
		}
	}
	return m, nil
}

// handleSplitViewNavDown handles down navigation in split view
func (m Model) handleSplitViewNavDown() (tea.Model, tea.Cmd) {
	m.lastKey = "" // Clear gg state
	switch m.activePanel {
	case SessionsPanel:
		sessions := m.filterSessions()
		if m.sessionCursor < len(sessions)-1 {
			m.sessionCursor++
			// Scroll-to-select: automatically select the session when navigating
			m.selectedSession = sessions[m.sessionCursor]
			m.cursor = 0 // Reset ball cursor for new session
			m.ballsScrollOffset = 0 // Reset balls scroll offset for new session
			m.selectedBalls = make(map[string]bool) // Clear multi-select on session change
		}
	case BallsPanel:
		balls := m.filterBallsForSession()
		if m.cursor < len(balls)-1 {
			m.cursor++
			// Adjust scroll offset to keep cursor visible
			m.adjustBallsScrollOffset(balls)
		}
	case ActivityPanel:
		// Scroll based on bottom pane mode
		if m.bottomPaneMode == BottomPaneDetail {
			// Scroll down in detail view
			m.detailScrollOffset++
			// The max offset will be clamped in the render function
		} else {
			// Scroll down one line in activity log
			maxOffset := m.getActivityLogMaxOffset()
			if m.activityLogOffset < maxOffset {
				m.activityLogOffset++
			}
		}
	}
	return m, nil
}

// handleSessionSwitchPrev switches to the previous session while in balls panel
func (m Model) handleSessionSwitchPrev() (tea.Model, tea.Cmd) {
	sessions := m.filterSessions()
	if len(sessions) == 0 {
		return m, nil
	}

	if m.sessionCursor > 0 {
		m.sessionCursor--
		m.selectedSession = sessions[m.sessionCursor]
		m.cursor = 0 // Reset ball cursor for new session
		m.ballsScrollOffset = 0 // Reset balls scroll offset for new session
		m.selectedBalls = make(map[string]bool) // Clear multi-select on session change
		m.addActivity("Switched to session: " + m.selectedSession.ID)
	}
	return m, nil
}

// handleSessionSwitchNext switches to the next session while in balls panel
func (m Model) handleSessionSwitchNext() (tea.Model, tea.Cmd) {
	sessions := m.filterSessions()
	if len(sessions) == 0 {
		return m, nil
	}

	if m.sessionCursor < len(sessions)-1 {
		m.sessionCursor++
		m.selectedSession = sessions[m.sessionCursor]
		m.cursor = 0 // Reset ball cursor for new session
		m.ballsScrollOffset = 0 // Reset balls scroll offset for new session
		m.selectedBalls = make(map[string]bool) // Clear multi-select on session change
		m.addActivity("Switched to session: " + m.selectedSession.ID)
	}
	return m, nil
}

// handleToggleBottomPane cycles through activity log, ball detail, split view, and updates
func (m Model) handleToggleBottomPane() (tea.Model, tea.Cmd) {
	switch m.bottomPaneMode {
	case BottomPaneActivity:
		m.bottomPaneMode = BottomPaneDetail
		m.detailScrollOffset = 0 // Reset scroll on mode change
		m.addActivity("Showing ball details in bottom pane")
	case BottomPaneDetail:
		m.bottomPaneMode = BottomPaneSplit
		m.addActivity("Showing split view (details + activity)")
	case BottomPaneSplit:
		m.bottomPaneMode = BottomPaneUpdates
		m.updatesHistoryOffset = 0 // Reset scroll on mode change
		m.addActivity("Showing updates history in bottom pane")
	case BottomPaneUpdates:
		m.bottomPaneMode = BottomPaneActivity
		m.addActivity("Showing activity log in bottom pane")
	}
	return m, nil
}

// handleCycleUpdateFilter cycles through update type filters when in Updates view.
// Cycles through: all → phase only → tool only → stop only → phase+tool → all
func (m Model) handleCycleUpdateFilter() (tea.Model, tea.Cmd) {
	if m.bottomPaneMode != BottomPaneUpdates {
		m.message = "Update filters only available in Updates view (press 'i' to cycle)"
		return m, nil
	}

	// Check current state to determine next state
	allOn := m.updatesTypeFilters[session.UpdateTypePhase] &&
		m.updatesTypeFilters[session.UpdateTypeToolUse] &&
		m.updatesTypeFilters[session.UpdateTypeToolFailure] &&
		m.updatesTypeFilters[session.UpdateTypeStop] &&
		m.updatesTypeFilters[session.UpdateTypeSessionEnd]

	phaseOnly := m.updatesTypeFilters[session.UpdateTypePhase] &&
		!m.updatesTypeFilters[session.UpdateTypeToolUse] &&
		!m.updatesTypeFilters[session.UpdateTypeToolFailure] &&
		!m.updatesTypeFilters[session.UpdateTypeStop] &&
		!m.updatesTypeFilters[session.UpdateTypeSessionEnd]

	toolOnly := !m.updatesTypeFilters[session.UpdateTypePhase] &&
		m.updatesTypeFilters[session.UpdateTypeToolUse] &&
		m.updatesTypeFilters[session.UpdateTypeToolFailure] &&
		!m.updatesTypeFilters[session.UpdateTypeStop] &&
		!m.updatesTypeFilters[session.UpdateTypeSessionEnd]

	stopOnly := !m.updatesTypeFilters[session.UpdateTypePhase] &&
		!m.updatesTypeFilters[session.UpdateTypeToolUse] &&
		!m.updatesTypeFilters[session.UpdateTypeToolFailure] &&
		m.updatesTypeFilters[session.UpdateTypeStop] &&
		!m.updatesTypeFilters[session.UpdateTypeSessionEnd]

	phaseAndTool := m.updatesTypeFilters[session.UpdateTypePhase] &&
		m.updatesTypeFilters[session.UpdateTypeToolUse] &&
		m.updatesTypeFilters[session.UpdateTypeToolFailure] &&
		!m.updatesTypeFilters[session.UpdateTypeStop] &&
		!m.updatesTypeFilters[session.UpdateTypeSessionEnd]

	// Cycle to next state
	switch {
	case allOn:
		// All on -> Phase only
		m.updatesTypeFilters[session.UpdateTypePhase] = true
		m.updatesTypeFilters[session.UpdateTypeToolUse] = false
		m.updatesTypeFilters[session.UpdateTypeToolFailure] = false
		m.updatesTypeFilters[session.UpdateTypeStop] = false
		m.updatesTypeFilters[session.UpdateTypeSessionEnd] = false
		m.message = "Updates: phase only"
		m.addActivity("Filtering updates: phase only")
	case phaseOnly:
		// Phase only -> Tool only
		m.updatesTypeFilters[session.UpdateTypePhase] = false
		m.updatesTypeFilters[session.UpdateTypeToolUse] = true
		m.updatesTypeFilters[session.UpdateTypeToolFailure] = true
		m.updatesTypeFilters[session.UpdateTypeStop] = false
		m.updatesTypeFilters[session.UpdateTypeSessionEnd] = false
		m.message = "Updates: tools only"
		m.addActivity("Filtering updates: tools only")
	case toolOnly:
		// Tool only -> Stop only
		m.updatesTypeFilters[session.UpdateTypePhase] = false
		m.updatesTypeFilters[session.UpdateTypeToolUse] = false
		m.updatesTypeFilters[session.UpdateTypeToolFailure] = false
		m.updatesTypeFilters[session.UpdateTypeStop] = true
		m.updatesTypeFilters[session.UpdateTypeSessionEnd] = false
		m.message = "Updates: stops only"
		m.addActivity("Filtering updates: stops only")
	case stopOnly:
		// Stop only -> Phase + Tool
		m.updatesTypeFilters[session.UpdateTypePhase] = true
		m.updatesTypeFilters[session.UpdateTypeToolUse] = true
		m.updatesTypeFilters[session.UpdateTypeToolFailure] = true
		m.updatesTypeFilters[session.UpdateTypeStop] = false
		m.updatesTypeFilters[session.UpdateTypeSessionEnd] = false
		m.message = "Updates: phase + tools"
		m.addActivity("Filtering updates: phase + tools")
	case phaseAndTool:
		// Phase + Tool -> All on
		m.updatesTypeFilters[session.UpdateTypePhase] = true
		m.updatesTypeFilters[session.UpdateTypeToolUse] = true
		m.updatesTypeFilters[session.UpdateTypeToolFailure] = true
		m.updatesTypeFilters[session.UpdateTypeStop] = true
		m.updatesTypeFilters[session.UpdateTypeSessionEnd] = true
		m.message = "Updates: showing all"
		m.addActivity("Filtering updates: showing all")
	default:
		// Unknown state -> Reset to all on
		m.updatesTypeFilters[session.UpdateTypePhase] = true
		m.updatesTypeFilters[session.UpdateTypeToolUse] = true
		m.updatesTypeFilters[session.UpdateTypeToolFailure] = true
		m.updatesTypeFilters[session.UpdateTypeStop] = true
		m.updatesTypeFilters[session.UpdateTypeSessionEnd] = true
		m.message = "Updates: showing all"
		m.addActivity("Filtering updates: showing all")
	}

	m.updatesHistoryOffset = 0 // Reset scroll when filter changes
	return m, nil
}

// handleToggleLocalOnly toggles between local project only and all projects
func (m Model) handleToggleLocalOnly() (tea.Model, tea.Cmd) {
	// Remember the currently selected session ID before reloading
	selectedSessionID := ""
	if m.selectedSession != nil {
		selectedSessionID = m.selectedSession.ID
	}

	m.localOnly = !m.localOnly
	if m.localOnly {
		m.addActivity("Showing local project only")
		m.message = "Showing local project only"
	} else {
		m.addActivity("Showing all projects")
		m.message = "Showing all projects"
	}

	// Store the selected session ID for cursor adjustment after reload
	m.pendingSessionSelect = selectedSessionID

	// Reload both balls and sessions with new scope
	return m, tea.Batch(
		loadBalls(m.store, m.config, m.localOnly),
		loadSessions(m.sessionStore, m.config, m.localOnly),
	)
}

// handleToggleSortOrder cycles through sort orders for balls.
// Each sort method has Desc then Asc before moving to the next method.
// Order: ID Desc → ID Asc → Priority Desc → Priority Asc → Activity Desc → Activity Asc → Created Desc → Created Asc → (loop)
func (m Model) handleToggleSortOrder() (tea.Model, tea.Cmd) {
	// Cycle through sort orders: each method has Desc then Asc
	switch m.sortOrder {
	case SortByIDASC:
		m.sortOrder = SortByIDDESC
		m.addActivity("Sort: ID descending")
		m.message = "Sort: ID descending"
	case SortByIDDESC:
		m.sortOrder = SortByPriorityDESC
		m.addActivity("Sort: Priority descending")
		m.message = "Sort: Priority descending (urgent first)"
	case SortByPriorityDESC:
		m.sortOrder = SortByPriorityASC
		m.addActivity("Sort: Priority ascending")
		m.message = "Sort: Priority ascending (low first)"
	case SortByPriorityASC:
		m.sortOrder = SortByLastActivityDESC
		m.addActivity("Sort: Activity descending")
		m.message = "Sort: Activity descending (recent first)"
	case SortByLastActivityDESC:
		m.sortOrder = SortByLastActivityASC
		m.addActivity("Sort: Activity ascending")
		m.message = "Sort: Activity ascending (oldest first)"
	case SortByLastActivityASC:
		m.sortOrder = SortByCreatedAtDESC
		m.addActivity("Sort: Created descending")
		m.message = "Sort: Created descending (newest first)"
	case SortByCreatedAtDESC:
		m.sortOrder = SortByCreatedAtASC
		m.addActivity("Sort: Created ascending")
		m.message = "Sort: Created ascending (oldest first)"
	case SortByCreatedAtASC:
		m.sortOrder = SortByIDASC
		m.addActivity("Sort: ID ascending")
		m.message = "Sort: ID ascending"
	}
	return m, nil
}

// handleToggleAgentOutput toggles the agent output panel visibility
func (m Model) handleToggleAgentOutput() (tea.Model, tea.Cmd) {
	m.agentOutputVisible = !m.agentOutputVisible
	if m.agentOutputVisible {
		m.addActivity("Agent output panel shown")
		m.message = "Agent output visible (O to hide, E to expand)"
	} else {
		m.addActivity("Agent output panel hidden")
		m.message = "Agent output hidden (O to show)"
	}
	return m, nil
}

// handleToggleAgentOutputExpand toggles the agent output panel between normal and expanded sizes
func (m Model) handleToggleAgentOutputExpand() (tea.Model, tea.Cmd) {
	if !m.agentOutputVisible {
		// Can't expand if not visible
		return m, nil
	}
	m.agentOutputExpanded = !m.agentOutputExpanded
	if m.agentOutputExpanded {
		m.addActivity("Agent output panel expanded")
		m.message = "Agent output expanded (E to collapse)"
	} else {
		m.addActivity("Agent output panel collapsed")
		m.message = "Agent output collapsed (E to expand)"
	}
	return m, nil
}

// handleAgentOutputScroll handles vim-style scrolling for the agent output panel
func (m Model) handleAgentOutputScrollUp() (tea.Model, tea.Cmd) {
	if m.agentOutputOffset > 0 {
		m.agentOutputOffset--
	}
	return m, nil
}

func (m Model) handleAgentOutputScrollDown() (tea.Model, tea.Cmd) {
	maxOffset := m.getAgentOutputMaxOffset()
	if m.agentOutputOffset < maxOffset {
		m.agentOutputOffset++
	}
	return m, nil
}

func (m Model) handleAgentOutputPageUp() (tea.Model, tea.Cmd) {
	pageSize := m.getAgentOutputVisibleLines() / 2
	if pageSize < 1 {
		pageSize = 1
	}
	m.agentOutputOffset -= pageSize
	if m.agentOutputOffset < 0 {
		m.agentOutputOffset = 0
	}
	return m, nil
}

func (m Model) handleAgentOutputPageDown() (tea.Model, tea.Cmd) {
	pageSize := m.getAgentOutputVisibleLines() / 2
	if pageSize < 1 {
		pageSize = 1
	}
	maxOffset := m.getAgentOutputMaxOffset()
	m.agentOutputOffset += pageSize
	if m.agentOutputOffset > maxOffset {
		m.agentOutputOffset = maxOffset
	}
	return m, nil
}

func (m Model) handleAgentOutputGoToTop() (tea.Model, tea.Cmd) {
	m.agentOutputOffset = 0
	return m, nil
}

func (m Model) handleAgentOutputGoToBottom() (tea.Model, tea.Cmd) {
	m.agentOutputOffset = m.getAgentOutputMaxOffset()
	return m, nil
}

// getActivityLogMaxOffset calculates the maximum scroll offset for activity log
func (m Model) getActivityLogMaxOffset() int {
	visibleLines := bottomPanelRows - 3 // Account for title and borders
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxOffset := len(m.activityLog) - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	return maxOffset
}

// adjustBallsScrollOffset adjusts the scroll offset to keep the cursor visible
func (m *Model) adjustBallsScrollOffset(balls []*session.Ball) {
	if len(balls) == 0 {
		m.ballsScrollOffset = 0
		return
	}

	// Calculate visible lines in balls panel
	// This should match the calculation in renderBallsPanel
	// mainHeight = m.height - bottomPanelRows - 4
	// ballsHeight = (mainHeight - 2) - 4
	mainHeight := m.height - bottomPanelRows - 4
	ballsHeight := mainHeight - 2 - 4
	if ballsHeight < 1 {
		ballsHeight = 1
	}

	// If cursor is above the visible area, scroll up
	if m.cursor < m.ballsScrollOffset {
		m.ballsScrollOffset = m.cursor
	}

	// Calculate visible area accounting for top scroll indicator
	// When scrollOffset > 0, we show a top indicator taking one line
	visibleArea := ballsHeight
	if m.ballsScrollOffset > 0 {
		visibleArea-- // One line used by "↑ N more items above"
	}
	if visibleArea < 1 {
		visibleArea = 1
	}

	// If cursor is below the visible area, scroll down
	// Important: after scrolling down, we'll have a top indicator (unless we scroll to 0)
	// So use (ballsHeight - 1) as the visible area for the new offset
	if m.cursor >= m.ballsScrollOffset+visibleArea {
		// Calculate what the new offset should be to make cursor visible
		// After scrolling, we'll have a top indicator, so visible = ballsHeight - 1
		newVisibleArea := ballsHeight - 1
		if newVisibleArea < 1 {
			newVisibleArea = 1
		}
		m.ballsScrollOffset = m.cursor - newVisibleArea + 1
	}

	// Ensure scroll offset is within valid bounds
	// When scrolled down (offset > 0), we lose one line to the top indicator
	// So maxOffset is calculated to ensure all items can be seen when scrolled to max
	// At maxOffset, we can show (ballsHeight - 1) items due to top indicator
	maxOffset := len(balls) - (ballsHeight - 1)
	if maxOffset < 0 {
		maxOffset = 0
	}
	// Special case: if all balls fit without scrolling, maxOffset should be 0
	if len(balls) <= ballsHeight {
		maxOffset = 0
	}
	if m.ballsScrollOffset > maxOffset {
		m.ballsScrollOffset = maxOffset
	}
	if m.ballsScrollOffset < 0 {
		m.ballsScrollOffset = 0
	}
}

// handleActivityLogPageDown scrolls down half a page in the activity log (or detail view)
func (m Model) handleActivityLogPageDown() (tea.Model, tea.Cmd) {
	m.lastKey = "" // Clear gg state
	pageSize := (bottomPanelRows - 3) / 2
	if pageSize < 1 {
		pageSize = 1
	}
	if m.bottomPaneMode == BottomPaneDetail {
		// Scroll detail view
		m.detailScrollOffset += pageSize
		// Will be clamped in render
	} else {
		maxOffset := m.getActivityLogMaxOffset()
		m.activityLogOffset += pageSize
		if m.activityLogOffset > maxOffset {
			m.activityLogOffset = maxOffset
		}
	}
	return m, nil
}

// handleActivityLogPageUp scrolls up half a page in the activity log (or detail view)
func (m Model) handleActivityLogPageUp() (tea.Model, tea.Cmd) {
	m.lastKey = "" // Clear gg state
	pageSize := (bottomPanelRows - 3) / 2
	if pageSize < 1 {
		pageSize = 1
	}
	if m.bottomPaneMode == BottomPaneDetail {
		// Scroll detail view
		m.detailScrollOffset -= pageSize
		if m.detailScrollOffset < 0 {
			m.detailScrollOffset = 0
		}
	} else {
		m.activityLogOffset -= pageSize
		if m.activityLogOffset < 0 {
			m.activityLogOffset = 0
		}
	}
	return m, nil
}

// handleActivityLogGoToTop scrolls to the top of the activity log (or detail view)
func (m Model) handleActivityLogGoToTop() (tea.Model, tea.Cmd) {
	if m.bottomPaneMode == BottomPaneDetail {
		m.detailScrollOffset = 0
	} else {
		m.activityLogOffset = 0
	}
	return m, nil
}

// handleActivityLogGoToBottom scrolls to the bottom of the activity log (or detail view)
func (m Model) handleActivityLogGoToBottom() (tea.Model, tea.Cmd) {
	if m.bottomPaneMode == BottomPaneDetail {
		// Set to large number, will be clamped in render
		m.detailScrollOffset = 1000
	} else {
		m.activityLogOffset = m.getActivityLogMaxOffset()
	}
	return m, nil
}

// handleSplitViewEnter handles enter key in split view
func (m Model) handleSplitViewEnter() (tea.Model, tea.Cmd) {
	switch m.activePanel {
	case SessionsPanel:
		// Select session and move focus to balls panel
		sessions := m.filterSessions()
		if len(sessions) > 0 && m.sessionCursor < len(sessions) {
			m.selectedSession = sessions[m.sessionCursor]
			m.cursor = 0 // Reset ball cursor for new session
			m.ballsScrollOffset = 0 // Reset balls scroll offset for new session
			m.selectedBalls = make(map[string]bool) // Clear multi-select on session change
			m.activePanel = BallsPanel
			m.addActivity("Selected session: " + m.selectedSession.ID)
		}
	case BallsPanel:
		// Open ball in edit mode (same as 'e' key)
		return m.handleSplitEditItem()
	}
	return m, nil
}

// handleSplitStartBall starts the selected ball(s) in split view
// Supports multi-select: if balls are selected, operates on all selected balls.
func (m Model) handleSplitStartBall() (tea.Model, tea.Cmd) {
	balls := m.filterBallsForSession()
	var ballsToStart []*session.Ball

	if len(m.selectedBalls) > 0 {
		// Multi-select mode: operate on all selected balls
		for _, ball := range balls {
			if m.selectedBalls[ball.ID] {
				ballsToStart = append(ballsToStart, ball)
			}
		}
	} else {
		// Single ball mode: operate on cursor ball
		if len(balls) == 0 || m.cursor >= len(balls) {
			return m, nil
		}
		ballsToStart = append(ballsToStart, balls[m.cursor])
	}

	if len(ballsToStart) == 0 {
		return m, nil
	}

	var cmds []tea.Cmd
	for _, ball := range ballsToStart {
		if err := ball.SetState(session.StateInProgress); err != nil {
			m.message = "Error: " + err.Error()
			return m, nil
		}

		store, err := session.NewStore(ball.WorkingDir)
		if err != nil {
			m.message = "Error: " + err.Error()
			return m, nil
		}
		cmds = append(cmds, updateBall(store, ball))
	}

	if len(ballsToStart) == 1 {
		m.addActivity("Started ball: " + ballsToStart[0].ID)
	} else {
		m.addActivity(fmt.Sprintf("Started %d balls", len(ballsToStart)))
	}

	// Clear multi-select after operation
	m.selectedBalls = make(map[string]bool)

	return m, tea.Batch(cmds...)
}

// handleSplitCompleteBall completes the selected ball(s) in split view and archives them
// Supports multi-select: if balls are selected, operates on all selected balls.
func (m Model) handleSplitCompleteBall() (tea.Model, tea.Cmd) {
	balls := m.filterBallsForSession()
	var ballsToComplete []*session.Ball

	if len(m.selectedBalls) > 0 {
		// Multi-select mode: operate on all selected balls
		for _, ball := range balls {
			if m.selectedBalls[ball.ID] {
				ballsToComplete = append(ballsToComplete, ball)
			}
		}
	} else {
		// Single ball mode: operate on cursor ball
		if len(balls) == 0 || m.cursor >= len(balls) {
			return m, nil
		}
		ballsToComplete = append(ballsToComplete, balls[m.cursor])
	}

	if len(ballsToComplete) == 0 {
		return m, nil
	}

	var cmds []tea.Cmd
	for _, ball := range ballsToComplete {
		if err := ball.SetState(session.StateComplete); err != nil {
			m.message = "Error: " + err.Error()
			return m, nil
		}

		store, err := session.NewStore(ball.WorkingDir)
		if err != nil {
			m.message = "Error: " + err.Error()
			return m, nil
		}
		// Update and archive the completed ball
		cmds = append(cmds, updateAndArchiveBall(store, ball))
	}

	if len(ballsToComplete) == 1 {
		m.addActivity("Completing ball: " + ballsToComplete[0].ID)
	} else {
		m.addActivity(fmt.Sprintf("Completing %d balls", len(ballsToComplete)))
	}

	// Clear multi-select after operation
	m.selectedBalls = make(map[string]bool)

	return m, tea.Batch(cmds...)
}

// handleSplitBlockBall prompts for a blocked reason
// Supports multi-select: if balls are selected, the reason will apply to all selected balls.
func (m Model) handleSplitBlockBall() (tea.Model, tea.Cmd) {
	balls := m.filterBallsForSession()
	var ballsToBlock []*session.Ball

	if len(m.selectedBalls) > 0 {
		// Multi-select mode: collect all selected balls
		for _, ball := range balls {
			if m.selectedBalls[ball.ID] {
				ballsToBlock = append(ballsToBlock, ball)
			}
		}
	} else {
		// Single ball mode: operate on cursor ball
		if len(balls) == 0 || m.cursor >= len(balls) {
			return m, nil
		}
		ballsToBlock = append(ballsToBlock, balls[m.cursor])
	}

	if len(ballsToBlock) == 0 {
		return m, nil
	}

	// Store balls to block for when reason is submitted
	m.pendingBlockBalls = ballsToBlock
	m.editingBall = ballsToBlock[0] // Keep for backwards compatibility
	m.textInput.Reset()
	m.textInput.Focus()
	m.textInput.Placeholder = "Blocked reason (e.g., waiting for API access)"
	m.inputTarget = "blocked_reason"
	m.mode = inputBlockedView

	if len(ballsToBlock) == 1 {
		m.addActivity("Blocking ball: " + ballsToBlock[0].ID)
	} else {
		m.addActivity(fmt.Sprintf("Blocking %d balls...", len(ballsToBlock)))
	}

	return m, nil
}

func (m *Model) handleStartBall() (tea.Model, tea.Cmd) {
	ball := m.filteredBalls[m.cursor]

	// Update state to in_progress
	if err := ball.SetState(session.StateInProgress); err != nil {
		m.message = "Error: " + err.Error()
		return m, nil
	}

	// Get the store for this ball's working directory
	store, err := session.NewStore(ball.WorkingDir)
	if err != nil {
		m.message = "Error creating store: " + err.Error()
		return m, nil
	}
	return m, updateBall(store, ball)
}

func (m *Model) handleCompleteBall() (tea.Model, tea.Cmd) {
	ball := m.filteredBalls[m.cursor]

	// Update state to complete
	if err := ball.SetState(session.StateComplete); err != nil {
		m.message = "Error: " + err.Error()
		return m, nil
	}

	// Get the store for this ball's working directory
	store, err := session.NewStore(ball.WorkingDir)
	if err != nil {
		m.message = "Error creating store: " + err.Error()
		return m, nil
	}
	// Update and archive the completed ball
	return m, updateAndArchiveBall(store, ball)
}

func (m *Model) handleDropBall() (tea.Model, tea.Cmd) {
	ball := m.filteredBalls[m.cursor]

	// Update state to blocked
	if err := ball.SetBlocked("dropped"); err != nil {
		m.message = "Error: " + err.Error()
		return m, nil
	}

	// Get the store for this ball's working directory
	store, err := session.NewStore(ball.WorkingDir)
	if err != nil {
		m.message = "Error creating store: " + err.Error()
		return m, nil
	}
	return m, updateBall(store, ball)
}

func (m *Model) handleStateFilter(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "1":
		// Show all - set all to true
		m.filterStates["pending"] = true
		m.filterStates["in_progress"] = true
		m.filterStates["blocked"] = true
		m.filterStates["complete"] = true
		m.message = "Filter: showing all states"
	case "2":
		m.filterStates["pending"] = !m.filterStates["pending"]
	case "3":
		m.filterStates["in_progress"] = !m.filterStates["in_progress"]
	case "4":
		m.filterStates["blocked"] = !m.filterStates["blocked"]
	case "5":
		m.filterStates["complete"] = !m.filterStates["complete"]
	}

	// Build message showing active filters
	if key != "1" {
		var active []string
		for state, visible := range m.filterStates {
			if visible {
				active = append(active, state)
			}
		}
		m.message = "Showing: " + strings.Join(active, ", ")
	}

	m.applyFilters()
	m.cursor = 0
	return m, nil
}

func (m *Model) applyFilters() {
	m.filteredBalls = make([]*session.Ball, 0)

	for _, ball := range m.balls {
		// Check if this ball's state is visible
		if m.filterStates[string(ball.State)] {
			m.filteredBalls = append(m.filteredBalls, ball)
		}
	}
}

func (m *Model) handleCycleState() (tea.Model, tea.Cmd) {
	ball := m.filteredBalls[m.cursor]

	// Determine next state
	var nextState session.BallState

	switch ball.State {
	case session.StatePending:
		nextState = session.StateInProgress
	case session.StateInProgress:
		nextState = session.StateComplete
	case session.StateComplete:
		nextState = session.StateBlocked
	case session.StateBlocked:
		nextState = session.StatePending
	default:
		nextState = session.StatePending
	}

	if err := ball.SetState(nextState); err != nil {
		m.message = "Error: " + err.Error()
		return m, nil
	}

	store, err := session.NewStore(ball.WorkingDir)
	if err != nil {
		m.message = "Error: " + err.Error()
		return m, nil
	}

	m.message = "Cycled to: " + formatState(ball)
	return m, updateBall(store, ball)
}

func (m *Model) handleSetReady() (tea.Model, tea.Cmd) {
	ball := m.filteredBalls[m.cursor]

	// Set to pending state
	if err := ball.SetState(session.StatePending); err != nil {
		m.message = "Error: " + err.Error()
		return m, nil
	}

	store, err := session.NewStore(ball.WorkingDir)
	if err != nil {
		m.message = "Error: " + err.Error()
		return m, nil
	}

	m.message = "Ball set to pending"
	return m, updateBall(store, ball)
}

func (m *Model) handleCyclePriority() (tea.Model, tea.Cmd) {
	ball := m.filteredBalls[m.cursor]

	// Determine next priority
	var nextPriority session.Priority
	switch ball.Priority {
	case session.PriorityLow:
		nextPriority = session.PriorityMedium
	case session.PriorityMedium:
		nextPriority = session.PriorityHigh
	case session.PriorityHigh:
		nextPriority = session.PriorityUrgent
	case session.PriorityUrgent:
		nextPriority = session.PriorityLow
	default:
		nextPriority = session.PriorityMedium
	}

	ball.Priority = nextPriority

	store, err := session.NewStore(ball.WorkingDir)
	if err != nil {
		m.message = "Error: " + err.Error()
		return m, nil
	}

	m.message = "Priority: " + string(nextPriority)
	return m, updateBall(store, ball)
}

// loadACTemplatesAndRepoACs loads AC templates and repo/session level ACs for the ball form
func (m *Model) loadACTemplatesAndRepoACs() {
	if m.store == nil {
		return
	}
	projectDir := m.store.ProjectDir()

	// Load AC templates from project config
	templates, err := session.GetProjectACTemplates(projectDir)
	if err == nil && len(templates) > 0 {
		m.acTemplates = templates
		m.acTemplateSelected = make([]bool, len(templates))
		m.acTemplateCursor = -1 // Not on templates initially
	} else {
		m.acTemplates = nil
		m.acTemplateSelected = nil
		m.acTemplateCursor = -1
	}

	// Load repo-level ACs
	repoACs, err := session.GetProjectAcceptanceCriteria(projectDir)
	if err == nil {
		m.repoLevelACs = repoACs
	} else {
		m.repoLevelACs = nil
	}

	// Load session-level ACs if a session is selected
	m.sessionLevelACs = nil
	if m.selectedSession != nil && m.selectedSession.ID != PseudoSessionAll && m.selectedSession.ID != PseudoSessionUntagged {
		m.sessionLevelACs = m.selectedSession.AcceptanceCriteria
	}
}

// handleSplitAddItem handles adding a new item based on active panel
func (m Model) handleSplitAddItem() (tea.Model, tea.Cmd) {
	m.inputAction = actionAdd
	m.textInput.Reset()
	m.textInput.Focus()

	switch m.activePanel {
	case SessionsPanel:
		m.textInput.Placeholder = "Session ID (e.g., feature-auth)"
		m.inputTarget = "session_id"
		m.mode = inputSessionView
		m.addActivity("Adding new session...")
	case BallsPanel:
		// Use unified ball form - all fields in one view
		m.pendingBallIntent = ""
		m.pendingBallContext = ""
		m.pendingBallPriority = 1 // Default to medium
		m.pendingBallTags = ""
		m.pendingAcceptanceCriteria = []string{}
		m.pendingACEditIndex = -1
		m.pendingBallDependsOn = nil
		m.pendingBallModelSize = 0
		// Initialize file autocomplete for @ mentions
		if m.store != nil {
			m.fileAutocomplete = NewAutocompleteState(m.store.ProjectDir())
		}
		// Load AC templates and repo/session level ACs
		m.loadACTemplatesAndRepoACs()
		// Default session to currently selected one (if a real session is selected)
		m.pendingBallSession = 0 // Start with (none)
		if m.selectedSession != nil && m.selectedSession.ID != PseudoSessionAll && m.selectedSession.ID != PseudoSessionUntagged {
			// Find the index of the selected session in real sessions
			realSessionIdx := 0
			for _, sess := range m.sessions {
				if sess.ID == PseudoSessionAll || sess.ID == PseudoSessionUntagged {
					continue
				}
				realSessionIdx++
				if sess.ID == m.selectedSession.ID {
					m.pendingBallSession = realSessionIdx
					break
				}
			}
		}
		m.pendingBallFormField = 0 // Start at context field
		m.contextInput.SetValue("")
		m.contextInput.Focus()
		m.textInput.Blur()
		m.textInput.Placeholder = "Background context for this task"
		m.mode = unifiedBallFormView
		m.addActivity("Creating new ball...")
	}

	return m, nil
}

// handleSplitAddFollowup creates a new ball that depends on the currently selected ball
func (m Model) handleSplitAddFollowup() (tea.Model, tea.Cmd) {
	balls := m.filterBallsForSession()
	if len(balls) == 0 || m.cursor >= len(balls) {
		m.message = "No ball selected"
		return m, nil
	}

	parentBall := balls[m.cursor]

	m.inputAction = actionAdd
	m.textInput.Reset()

	// Use unified ball form - all fields in one view
	m.pendingBallIntent = ""
	m.pendingBallContext = ""
	m.pendingBallPriority = 1 // Default to medium
	m.pendingBallTags = ""
	m.pendingAcceptanceCriteria = []string{}
	m.pendingACEditIndex = -1
	m.pendingNewAC = ""
	m.pendingBallModelSize = 0

	// Pre-fill depends_on with the current ball's ID
	m.pendingBallDependsOn = []string{parentBall.ID}

	// Initialize file autocomplete for @ mentions
	if m.store != nil {
		m.fileAutocomplete = NewAutocompleteState(m.store.ProjectDir())
	}
	// Load AC templates and repo/session level ACs
	m.loadACTemplatesAndRepoACs()

	// Default session to currently selected one (if a real session is selected)
	m.pendingBallSession = 0 // Start with (none)
	if m.selectedSession != nil && m.selectedSession.ID != PseudoSessionAll && m.selectedSession.ID != PseudoSessionUntagged {
		// Find the index of the selected session in real sessions
		realSessionIdx := 0
		for _, sess := range m.sessions {
			if sess.ID == PseudoSessionAll || sess.ID == PseudoSessionUntagged {
				continue
			}
			realSessionIdx++
			if sess.ID == m.selectedSession.ID {
				m.pendingBallSession = realSessionIdx
				break
			}
		}
	}

	m.pendingBallFormField = 0 // Start at context field
	m.contextInput.SetValue("")
	m.contextInput.Focus()
	m.textInput.Blur()
	m.textInput.Placeholder = "Background context for this task"
	m.mode = unifiedBallFormView
	m.addActivity("Creating followup ball (depends on: " + parentBall.ID + ")...")

	return m, nil
}

// handleSplitEditItem handles editing the selected item
func (m Model) handleSplitEditItem() (tea.Model, tea.Cmd) {
	m.inputAction = actionEdit
	m.textInput.Reset()
	m.textInput.Focus()

	switch m.activePanel {
	case SessionsPanel:
		sessions := m.filterSessions()
		if len(sessions) == 0 || m.sessionCursor >= len(sessions) {
			m.message = "No session selected"
			return m, nil
		}
		sess := sessions[m.sessionCursor]
		// Prevent editing pseudo-sessions
		if sess.ID == PseudoSessionAll || sess.ID == PseudoSessionUntagged {
			m.message = "Cannot edit built-in session"
			return m, nil
		}
		m.editingSession = sess
		m.textInput.Placeholder = "Session description"
		m.textInput.SetValue(sess.Description)
		m.inputTarget = "session_description"
		m.mode = inputSessionView
		m.addActivity("Editing session: " + sess.ID)

	case BallsPanel:
		balls := m.filterBallsForSession()
		if len(balls) == 0 || m.cursor >= len(balls) {
			m.message = "No ball selected"
			return m, nil
		}
		// Edit only works on cursor ball, clear any multi-selection
		if len(m.selectedBalls) > 1 {
			m.message = "Edit works on one ball at a time - editing cursor ball"
			m.addActivity("Multi-select cleared for edit")
		}
		m.selectedBalls = make(map[string]bool)
		ball := balls[m.cursor]
		m.editingBall = ball

		// Initialize file autocomplete for @ mentions
		if m.store != nil {
			m.fileAutocomplete = NewAutocompleteState(m.store.ProjectDir())
		}
		// Load AC templates and repo/session level ACs
		m.loadACTemplatesAndRepoACs()

		// Use unified ball form with prepopulated fields
		m.pendingBallContext = ball.Context
		m.pendingBallIntent = ball.Title
		// Convert priority to index (low=0, medium=1, high=2, urgent=3)
		switch ball.Priority {
		case session.PriorityLow:
			m.pendingBallPriority = 0
		case session.PriorityMedium:
			m.pendingBallPriority = 1
		case session.PriorityHigh:
			m.pendingBallPriority = 2
		case session.PriorityUrgent:
			m.pendingBallPriority = 3
		default:
			m.pendingBallPriority = 1 // Default to medium
		}
		m.pendingBallTags = strings.Join(ball.Tags, ", ")
		m.pendingAcceptanceCriteria = make([]string, len(ball.AcceptanceCriteria))
		copy(m.pendingAcceptanceCriteria, ball.AcceptanceCriteria)
		m.pendingACEditIndex = -1
		m.pendingBallDependsOn = make([]string, len(ball.DependsOn))
		copy(m.pendingBallDependsOn, ball.DependsOn)

		// Convert model size to index (blank=0, small=1, medium=2, large=3)
		switch ball.ModelSize {
		case session.ModelSizeSmall:
			m.pendingBallModelSize = 1
		case session.ModelSizeMedium:
			m.pendingBallModelSize = 2
		case session.ModelSizeLarge:
			m.pendingBallModelSize = 3
		default:
			m.pendingBallModelSize = 0 // Default
		}

		// Convert agent provider to index (blank=0, claude=1, opencode=2)
		switch ball.AgentProvider {
		case "claude":
			m.pendingBallAgentProvider = 1
		case "opencode":
			m.pendingBallAgentProvider = 2
		default:
			m.pendingBallAgentProvider = 0 // Default
		}

		// Convert model override to index (blank=0, opus=1, sonnet=2, haiku=3)
		switch ball.ModelOverride {
		case "opus":
			m.pendingBallModelOverride = 1
		case "sonnet":
			m.pendingBallModelOverride = 2
		case "haiku":
			m.pendingBallModelOverride = 3
		default:
			m.pendingBallModelOverride = 0 // Default
		}

		// Find session index from tags (first tag that matches a session)
		m.pendingBallSession = 0 // Default to (none)
		for _, tag := range ball.Tags {
			realSessionIdx := 0
			for _, sess := range m.sessions {
				if sess.ID == PseudoSessionAll || sess.ID == PseudoSessionUntagged {
					continue
				}
				realSessionIdx++
				if sess.ID == tag {
					m.pendingBallSession = realSessionIdx
					break
				}
			}
			if m.pendingBallSession > 0 {
				break // Found a session match
			}
		}

		// Convert blocking reason to index (0=blank, 1=Human needed, 2=Waiting for dependency, 3=Needs research, 4=custom)
		m.pendingBallBlockingReason = 0 // Default to blank
		m.pendingBallCustomReason = ""
		if ball.BlockedReason != "" {
			switch ball.BlockedReason {
			case "Human needed":
				m.pendingBallBlockingReason = 1
			case "Waiting for dependency":
				m.pendingBallBlockingReason = 2
			case "Needs research":
				m.pendingBallBlockingReason = 3
			default:
				// Custom reason
				m.pendingBallBlockingReason = 4
				m.pendingBallCustomReason = ball.BlockedReason
			}
		}

		m.pendingBallFormField = 0 // Start at context field
		m.contextInput.SetValue(ball.Context)
		m.contextInput.Focus()
		adjustContextTextareaHeight(&m)
		m.textInput.Blur()
		m.textInput.Placeholder = "Background context for this task"
		m.mode = unifiedBallFormView
		m.addActivity("Editing ball: " + ball.ID)
	}

	return m, nil
}

// handleBallEditInEditor opens the selected ball in an external editor (E key)
func (m Model) handleBallEditInEditor() (tea.Model, tea.Cmd) {
	if m.activePanel != BallsPanel {
		return m, nil
	}

	balls := m.filterBallsForSession()
	if len(balls) == 0 || m.cursor >= len(balls) {
		m.message = "No ball selected"
		return m, nil
	}

	ball := balls[m.cursor]
	m.editingBall = ball
	m.inputAction = actionEdit
	m.addActivity("Opening editor for: " + ball.ID)
	return m, openEditorCmd(ball)
}

// handleCopyBallID copies the current ball's ID to the system clipboard (split view)
// Format: "projectName:ballID" (e.g., "myproject:myproject-abc123")
func (m Model) handleCopyBallID() (tea.Model, tea.Cmd) {
	// Get the current ball under cursor
	balls := m.filterBallsForSession()
	if len(balls) == 0 || m.cursor >= len(balls) {
		m.message = "No ball selected"
		return m, nil
	}
	ball := balls[m.cursor]

	// Format: "projectName:ballID"
	copyText := ball.FolderName() + ":" + ball.ID

	// Copy to clipboard
	if err := copyToClipboard(copyText); err != nil {
		m.message = "Clipboard unavailable: " + err.Error()
		m.addActivity("Clipboard error: " + err.Error())
		return m, nil
	}

	m.message = "Copied: " + copyText
	m.addActivity("Copied ball ID to clipboard: " + copyText)
	return m, nil
}

// copyToClipboard copies text to the system clipboard
// Supports Linux (xclip/xsel), macOS (pbcopy), and Windows (clip)
func copyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, fall back to xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return exec.ErrNotFound
		}
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		return exec.ErrNotFound
	}

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := pipe.Write([]byte(text)); err != nil {
		return err
	}

	if err := pipe.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

// handleMoveKeySequence handles the second key (digit) in a move/append sequence.
// If appendOnly is false, removes all existing session tags before adding the target.
// If appendOnly is true, just adds the target session without removing others.
// Supports multi-select: if balls are selected, operates on all selected balls.
func (m Model) handleMoveKeySequence(key string, appendOnly bool) (tea.Model, tea.Cmd) {
	m.message = ""

	// Must be a digit 0-9
	if !isDigitKey(key) {
		m.message = "Invalid key: " + key + " (use 1-9 or 0)"
		return m, nil
	}

	// Map key to session index (1-9 -> 0-8, 0 -> 9)
	idx := keyToSessionIndex(key)

	// Get real sessions (excluding pseudo), matching what's displayed in the sessions panel
	// Use filterSessions() to respect any active panel search filter
	realSessions := getRealSessions(m.filterSessions())
	if idx >= len(realSessions) {
		m.message = "No session at position " + key
		return m, nil
	}
	targetSession := realSessions[idx]

	// Determine which balls to operate on
	balls := m.filterBallsForSession()
	var ballsToMove []*session.Ball

	if len(m.selectedBalls) > 0 {
		// Multi-select mode: operate on all selected balls
		for _, ball := range balls {
			if m.selectedBalls[ball.ID] {
				ballsToMove = append(ballsToMove, ball)
			}
		}
	} else {
		// Single ball mode: operate on cursor ball
		if len(balls) == 0 || m.cursor >= len(balls) {
			m.message = "No ball selected"
			return m, nil
		}
		ballsToMove = append(ballsToMove, balls[m.cursor])
	}

	if len(ballsToMove) == 0 {
		m.message = "No balls to move"
		return m, nil
	}

	// Get all real sessions for removing tags (when not append-only)
	allRealSessions := getRealSessions(m.sessions)

	// Process all balls
	var cmds []tea.Cmd
	for _, ball := range ballsToMove {
		if !appendOnly {
			// Move: remove all session tags from all real sessions (not just filtered ones)
			// This ensures ball is only in the target session, regardless of filter state
			for _, sess := range allRealSessions {
				ball.RemoveTag(sess.ID)
			}
		}

		// Add target session tag
		ball.AddTag(targetSession.ID)

		// Persist
		store, err := session.NewStore(ball.WorkingDir)
		if err != nil {
			m.message = "Error: " + err.Error()
			return m, nil
		}
		cmds = append(cmds, updateBall(store, ball))
	}

	action := "Moved"
	if appendOnly {
		action = "Added"
	}
	if len(ballsToMove) == 1 {
		m.message = action + " ball to session: " + targetSession.ID
		m.addActivity(action + " " + ballsToMove[0].ID + " to session: " + targetSession.ID)
	} else {
		m.message = fmt.Sprintf("%s %d balls to session: %s", action, len(ballsToMove), targetSession.ID)
		m.addActivity(fmt.Sprintf("%s %d balls to session: %s", action, len(ballsToMove), targetSession.ID))
	}

	// Clear multi-select after operation
	m.selectedBalls = make(map[string]bool)

	return m, tea.Batch(cmds...)
}

// handleRemoveCurrentSessionFromBall removes the currently selected session from the ball's tags.
// Only works if a real session (not pseudo-session) is selected.
// Supports multi-select: if balls are selected, operates on all selected balls.
func (m Model) handleRemoveCurrentSessionFromBall() (tea.Model, tea.Cmd) {
	m.message = ""

	// Only works if a real session is selected
	if m.selectedSession == nil {
		m.message = "No session selected"
		return m, nil
	}
	if m.selectedSession.ID == PseudoSessionAll || m.selectedSession.ID == PseudoSessionUntagged {
		m.message = "Select a real session to remove"
		return m, nil
	}

	// Determine which balls to operate on
	balls := m.filterBallsForSession()
	var ballsToRemove []*session.Ball

	if len(m.selectedBalls) > 0 {
		// Multi-select mode: operate on all selected balls
		for _, ball := range balls {
			if m.selectedBalls[ball.ID] {
				ballsToRemove = append(ballsToRemove, ball)
			}
		}
	} else {
		// Single ball mode: operate on cursor ball
		if len(balls) == 0 || m.cursor >= len(balls) {
			m.message = "No ball selected"
			return m, nil
		}
		ballsToRemove = append(ballsToRemove, balls[m.cursor])
	}

	if len(ballsToRemove) == 0 {
		m.message = "No balls to remove from session"
		return m, nil
	}

	// Process all balls
	var cmds []tea.Cmd
	removedCount := 0
	for _, ball := range ballsToRemove {
		// Try to remove the session tag
		if ball.RemoveTag(m.selectedSession.ID) {
			removedCount++
			// Persist
			store, err := session.NewStore(ball.WorkingDir)
			if err != nil {
				m.message = "Error: " + err.Error()
				return m, nil
			}
			cmds = append(cmds, updateBall(store, ball))
		}
	}

	if removedCount == 0 {
		m.message = "No balls were in this session"
		return m, nil
	}

	if removedCount == 1 {
		m.message = "Removed from session: " + m.selectedSession.ID
		m.addActivity("Removed " + ballsToRemove[0].ID + " from session: " + m.selectedSession.ID)
	} else {
		m.message = fmt.Sprintf("Removed %d balls from session: %s", removedCount, m.selectedSession.ID)
		m.addActivity(fmt.Sprintf("Removed %d balls from session: %s", removedCount, m.selectedSession.ID))
	}

	// Clear multi-select after operation
	m.selectedBalls = make(map[string]bool)

	return m, tea.Batch(cmds...)
}

// isDigitKey returns true if the key is a digit 0-9
func isDigitKey(key string) bool {
	return len(key) == 1 && key[0] >= '0' && key[0] <= '9'
}

// keyToSessionIndex converts a digit key to a session index.
// Keys 1-9 map to indices 0-8, key 0 maps to index 9.
func keyToSessionIndex(key string) int {
	if key == "0" {
		return 9
	}
	return int(key[0] - '1')
}
