package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ohare93/juggle/internal/session"
)

func (m Model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nPress q to quit", m.err))
	}

	switch m.mode {
	case splitView:
		return m.renderSplitView()
	case splitHelpView:
		return m.renderSplitHelpView()
	case inputSessionView, inputBallView, inputBlockedView:
		return m.renderInputView()
	case unifiedBallFormView:
		return m.renderUnifiedBallFormView()
	case inputTagView:
		return m.renderTagView()
	case sessionSelectorView:
		return m.renderSessionSelectorView()
	case dependencySelectorView:
		return m.renderDependencySelectorView()
	case confirmSplitDelete:
		return m.renderSplitConfirmDelete()
	case confirmAgentCancel:
		return m.renderAgentCancelConfirm()
	case panelSearchView:
		return m.renderPanelSearchView()
	case historyView:
		return m.renderHistoryView()
	case historyOutputView:
		return m.renderHistoryOutputView()
	case agentMonitorView:
		return m.renderAgentMonitorView()
	default:
		return "Unknown view"
	}
}

// renderInputView renders the text input dialog
func (m Model) renderInputView() string {
	var b strings.Builder

	// Determine title based on mode and action
	var title string
	switch m.mode {
	case inputSessionView:
		if m.inputAction == actionAdd {
			title = "Create New Session"
		} else {
			title = "Edit Session"
		}
	case inputBallView:
		if m.inputAction == actionAdd {
			title = "Create New Ball"
		} else {
			title = "Edit Ball"
		}
	case inputBlockedView:
		title = "Block Ball"
	}

	titleStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Render(title)
	b.WriteString(titleStyled + "\n\n")

	// Show context based on mode
	switch m.mode {
	case inputSessionView:
		if m.inputAction == actionEdit && m.editingSession != nil {
			b.WriteString(fmt.Sprintf("Session: %s\n\n", m.editingSession.ID))
		}
	case inputBallView:
		if m.inputAction == actionEdit && m.editingBall != nil {
			b.WriteString(fmt.Sprintf("Ball: %s\n\n", m.editingBall.ID))
		}
		if m.selectedSession != nil && m.inputAction == actionAdd {
			b.WriteString(fmt.Sprintf("Session: %s\n\n", m.selectedSession.ID))
		}
	case inputBlockedView:
		if m.editingBall != nil {
			b.WriteString(fmt.Sprintf("Ball: %s\n", m.editingBall.ID))
			b.WriteString(fmt.Sprintf("Title: %s\n\n", m.editingBall.Title))
		}
	}

	// Show input field
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		Width(50)
	b.WriteString(inputStyle.Render(m.textInput.View()) + "\n\n")

	// Show message if any
	if m.message != "" {
		b.WriteString(messageStyle.Render(m.message) + "\n\n")
	}

	// Help
	help := lipgloss.NewStyle().
		Faint(true).
		Render("Enter = submit | Esc = cancel")
	b.WriteString(help)

	return b.String()
}

// renderSplitConfirmDelete renders the delete confirmation for split view
func (m Model) renderSplitConfirmDelete() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("1")). // Red
		Render("Confirm Delete")
	b.WriteString(title + "\n\n")

	// Show what will be deleted
	switch m.confirmAction {
	case "delete_session":
		if m.sessionCursor < len(m.sessions) {
			sess := m.sessions[m.sessionCursor]
			b.WriteString(fmt.Sprintf("Session: %s\n", sess.ID))
			if sess.Description != "" {
				b.WriteString(fmt.Sprintf("Description: %s\n", sess.Description))
			}
			ballCount := m.countBallsForSession(sess.ID)
			b.WriteString(fmt.Sprintf("Balls: %d\n", ballCount))
		}
	case "delete_ball":
		// Use pendingDeleteBalls if available
		ballsToDelete := m.pendingDeleteBalls
		if len(ballsToDelete) == 0 {
			balls := m.getBallsForSession()
			if m.cursor < len(balls) {
				ballsToDelete = []*session.Ball{balls[m.cursor]}
			}
		}

		if len(ballsToDelete) == 1 {
			ball := ballsToDelete[0]
			b.WriteString(fmt.Sprintf("Ball: %s\n", ball.ID))
			b.WriteString(fmt.Sprintf("Title: %s\n", ball.Title))
			b.WriteString(fmt.Sprintf("State: %s\n", ball.State))
			b.WriteString(fmt.Sprintf("Criteria: %d\n", len(ball.AcceptanceCriteria)))
		} else if len(ballsToDelete) > 1 {
			b.WriteString(fmt.Sprintf("Balls: %d selected\n\n", len(ballsToDelete)))
			for i, ball := range ballsToDelete {
				if i >= 5 {
					b.WriteString(fmt.Sprintf("  ... and %d more\n", len(ballsToDelete)-5))
					break
				}
				b.WriteString(fmt.Sprintf("  • %s: %s\n", ball.ID, truncate(ball.Title, 40)))
			}
		}
	}

	b.WriteString("\n")

	warning := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")). // Yellow
		Render("This action cannot be undone.")
	b.WriteString(warning + "\n\n")

	prompt := lipgloss.NewStyle().
		Bold(true).
		Render("Delete? [y/N]")
	b.WriteString(prompt + "\n\n")

	help := lipgloss.NewStyle().
		Faint(true).
		Render("y = confirm | n/Esc = cancel")
	b.WriteString(help)

	return b.String()
}

// renderAgentCancelConfirm renders the agent cancel confirmation dialog
func (m Model) renderAgentCancelConfirm() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("1")). // Red
		Render("Cancel Agent")
	b.WriteString(title + "\n\n")

	// Show running agent details
	if m.agentStatus.Running {
		b.WriteString(fmt.Sprintf("Session: %s\n", m.agentStatus.SessionID))
		b.WriteString(fmt.Sprintf("Progress: %d/%d iterations\n",
			m.agentStatus.Iteration,
			m.agentStatus.MaxIterations))
	}

	b.WriteString("\n")

	warning := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")). // Yellow
		Render("The agent will be terminated immediately.")
	b.WriteString(warning + "\n")

	info := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // Gray
		Render("Any completed work will be preserved, but the current task may be interrupted.")
	b.WriteString(info + "\n\n")

	prompt := lipgloss.NewStyle().
		Bold(true).
		Render("Cancel agent? [y/N]")
	b.WriteString(prompt + "\n\n")

	help := lipgloss.NewStyle().
		Faint(true).
		Render("y = terminate agent | n/Esc = continue running")
	b.WriteString(help)

	return b.String()
}

// renderPanelSearchView renders the search/filter input dialog
func (m Model) renderPanelSearchView() string {
	var b strings.Builder

	// Title based on active panel
	var panelName string
	switch m.activePanel {
	case SessionsPanel:
		panelName = "Sessions"
	case BallsPanel:
		panelName = "Balls"
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Render("Filter " + panelName)
	b.WriteString(title + "\n\n")

	// Show input field
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		Width(50)
	b.WriteString(inputStyle.Render(m.textInput.View()) + "\n\n")

	// Show message if any
	if m.message != "" {
		b.WriteString(messageStyle.Render(m.message) + "\n\n")
	}

	// Help
	help := lipgloss.NewStyle().
		Faint(true).
		Render("Enter = apply filter | Esc = cancel")
	b.WriteString(help + "\n")

	// Additional help
	if m.panelSearchQuery != "" {
		helpClear := lipgloss.NewStyle().
			Faint(true).
			Render("Current filter: " + m.panelSearchQuery + " (Ctrl+U to clear in panel)")
		b.WriteString(helpClear)
	}

	return b.String()
}

// renderSessionSelectorView renders the session selection dialog for tagging
func (m Model) renderSessionSelectorView() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Render("Select Sessions")
	b.WriteString(title + "\n\n")

	// Show ball context
	if m.editingBall != nil {
		b.WriteString(fmt.Sprintf("Ball: %s\n", m.editingBall.ID))
		b.WriteString(fmt.Sprintf("Title: %s\n\n", m.editingBall.Title))

		// Show current sessions/tags
		if len(m.editingBall.Tags) > 0 {
			currentLabel := lipgloss.NewStyle().
				Faint(true).
				Render("Current sessions:")
			b.WriteString(currentLabel + " ")

			tagStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))
			tags := strings.Join(m.editingBall.Tags, ", ")
			b.WriteString(tagStyle.Render(tags) + "\n\n")
		}
	}

	// Show session list
	sessionLabel := lipgloss.NewStyle().
		Bold(true).
		Render("Available sessions (Space = toggle, Enter = confirm):")
	b.WriteString(sessionLabel + "\n\n")

	if len(m.sessionSelectItems) == 0 {
		noSessions := lipgloss.NewStyle().
			Faint(true).
			Render("  No sessions available")
		b.WriteString(noSessions + "\n")
	} else {
		selectedStyle := lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15"))

		normalStyle := lipgloss.NewStyle()

		checkedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")) // Green for checked

		for i, sess := range m.sessionSelectItems {
			// Cursor indicator
			cursor := "  "
			if i == m.sessionSelectIndex {
				cursor = "> "
			}

			// Checkbox indicator
			checkbox := "[ ] "
			if m.sessionSelectActive != nil && m.sessionSelectActive[sess.ID] {
				checkbox = "[✓] "
			}

			line := fmt.Sprintf("%s%s%s", cursor, checkbox, sess.ID)
			if sess.Description != "" {
				line += fmt.Sprintf(" - %s", truncate(sess.Description, 35))
			}

			if i == m.sessionSelectIndex {
				b.WriteString(selectedStyle.Render(line) + "\n")
			} else if m.sessionSelectActive != nil && m.sessionSelectActive[sess.ID] {
				b.WriteString(checkedStyle.Render(line) + "\n")
			} else {
				b.WriteString(normalStyle.Render(line) + "\n")
			}
		}
	}

	// Show selected count
	selectedCount := 0
	if m.sessionSelectActive != nil {
		for _, selected := range m.sessionSelectActive {
			if selected {
				selectedCount++
			}
		}
	}
	if selectedCount > 0 {
		countLabel := lipgloss.NewStyle().
			Faint(true).
			Render(fmt.Sprintf("\nSelected: %d session(s)", selectedCount))
		b.WriteString(countLabel + "\n")
	}

	b.WriteString("\n")

	// Show message if any
	if m.message != "" {
		b.WriteString(messageStyle.Render(m.message) + "\n\n")
	}

	// Help
	help := lipgloss.NewStyle().
		Faint(true).
		Render("j/k = navigate | Space = toggle | Enter = confirm | Esc = cancel")
	b.WriteString(help)

	return b.String()
}

// renderDependencySelectorView renders the dependency selection dialog
func (m Model) renderDependencySelectorView() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Render("Select Dependencies")
	b.WriteString(title + "\n\n")

	instructions := lipgloss.NewStyle().
		Faint(true).
		Render("Use Space to toggle selection, Enter to confirm")
	b.WriteString(instructions + "\n\n")

	// Show ball list
	if len(m.dependencySelectBalls) == 0 {
		noBalls := lipgloss.NewStyle().
			Faint(true).
			Render("  No non-complete balls available")
		b.WriteString(noBalls + "\n")
	} else {
		selectedStyle := lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15"))
		checkedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
		uncheckedStyle := lipgloss.NewStyle().
			Faint(true)
		stateStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

		for i, ball := range m.dependencySelectBalls {
			cursor := "  "
			if i == m.dependencySelectIndex {
				cursor = "> "
			}

			// Checkbox
			checkbox := "[ ]"
			if m.dependencySelectActive[ball.ID] {
				checkbox = "[✓]"
			}

			// Ball info
			shortID := ball.ShortID()
			intent := truncate(ball.Title, 40)
			state := string(ball.State)

			fullLine := fmt.Sprintf("%s%s %s %s - %s", cursor, checkbox, shortID, "("+state+")", intent)

			if i == m.dependencySelectIndex {
				// Highlight the whole line when cursor is on it
				b.WriteString(selectedStyle.Render(fullLine) + "\n")
			} else if m.dependencySelectActive[ball.ID] {
				// Show checked items in green
				b.WriteString(fmt.Sprintf("%s%s %s %s - %s\n", cursor, checkedStyle.Render(checkbox), shortID, stateStyle.Render("("+state+")"), intent))
			} else {
				b.WriteString(fmt.Sprintf("%s%s %s %s - %s\n", cursor, uncheckedStyle.Render(checkbox), shortID, stateStyle.Render("("+state+")"), intent))
			}
		}
	}

	b.WriteString("\n")

	// Show current selection count
	selectedCount := len(m.dependencySelectActive)
	if selectedCount > 0 {
		countStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
		b.WriteString(countStyle.Render(fmt.Sprintf("Selected: %d", selectedCount)) + "\n\n")
	}

	// Show message if any
	if m.message != "" {
		b.WriteString(messageStyle.Render(m.message) + "\n\n")
	}

	// Help
	help := lipgloss.NewStyle().
		Faint(true).
		Render("j/k or ↑/↓ = navigate | Space = toggle | Enter = confirm | Esc = cancel")
	b.WriteString(help)

	return b.String()
}

// renderTagView renders the tag editing dialog
func (m Model) renderTagView() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Render("Edit Tags")
	b.WriteString(title + "\n\n")

	// Show ball context
	if m.editingBall != nil {
		b.WriteString(fmt.Sprintf("Ball: %s\n", m.editingBall.ID))
		b.WriteString(fmt.Sprintf("Title: %s\n\n", m.editingBall.Title))

		// Show current tags
		if len(m.editingBall.Tags) > 0 {
			tagsLabel := lipgloss.NewStyle().
				Bold(true).
				Render("Current Tags:")
			b.WriteString(tagsLabel + "\n")

			tagStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)

			for _, tag := range m.editingBall.Tags {
				b.WriteString("  " + tagStyle.Render(tag) + "\n")
			}
			b.WriteString("\n")
		} else {
			noTags := lipgloss.NewStyle().
				Faint(true).
				Render("No tags")
			b.WriteString(noTags + "\n\n")
		}
	}

	// Show input field
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		Width(50)
	b.WriteString(inputStyle.Render(m.textInput.View()) + "\n\n")

	// Show message if any
	if m.message != "" {
		b.WriteString(messageStyle.Render(m.message) + "\n\n")
	}

	// Help
	help := lipgloss.NewStyle().
		Faint(true).
		Render("Enter = submit | Esc = cancel\n")
	b.WriteString(help)

	helpAdd := lipgloss.NewStyle().
		Faint(true).
		Render("Type tag name to add | Prefix with - to remove (e.g., -mytag)")
	b.WriteString(helpAdd)

	return b.String()
}

// helpItem represents a key-description pair for help views
type helpItem struct {
	key  string
	desc string
}

// helpSection formats a section of help items with a title
func helpSection(title string, items []helpItem) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(title) + "\n")
	for _, item := range items {
		keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
		b.WriteString(fmt.Sprintf("  %s  %s\n", keyStyle.Render(item.key), item.desc))
	}
	b.WriteString("\n")

	return b.String()
}

// renderSplitHelpView renders the comprehensive help view for split mode
func (m Model) renderSplitHelpView() string {
	var b strings.Builder

	title := titleStyle.Render("Juggle TUI - Complete Keybindings Reference")
	b.WriteString(title + "\n\n")

	// Build all help sections - organized by category
	sections := []struct {
		title string
		items []helpItem
	}{
		{
			title: "Navigation",
			items: []helpItem{
				{"Tab / l", "Next panel (Sessions → Balls → Activity)"},
				{"Shift+Tab / h", "Previous panel"},
				{"j / ↓", "Move down / Scroll down"},
				{"k / ↑", "Move up / Scroll up"},
				{"[ / ]", "Previous / Next session (in Balls panel)"},
				{"Enter", "Select item / Edit"},
				{"Esc", "Back / Deselect / Close"},
			},
		},
		{
			title: "Sessions Panel",
			items: []helpItem{
				{"j/k", "Navigate sessions (auto-selects)"},
				{"Enter", "Select session and go to balls panel"},
				{"a", "Add new session"},
				{"e", "Edit session description"},
				{"d", "Delete session (with confirmation)"},
				{"/", "Filter sessions"},
				{"Ctrl+U", "Clear filter"},
			},
		},
		{
			title: "Balls Panel - State Changes (s + key)",
			items: []helpItem{
				{"s", "Start two-key state change sequence:"},
				{"  sc", "  Complete ball (→ complete, archives)"},
				{"  ss", "  Start ball (→ in_progress)"},
				{"  sb", "  Block ball (prompts for reason)"},
				{"  sp", "  Set to pending"},
				{"  sa", "  Archive completed ball"},
			},
		},
		{
			title: "Balls Panel - Toggle Filters (t + key)",
			items: []helpItem{
				{"t", "Start two-key toggle filter sequence:"},
				{"  tc", "  Toggle complete balls visibility"},
				{"  tb", "  Toggle blocked balls visibility"},
				{"  ti", "  Toggle in_progress balls visibility"},
				{"  tp", "  Toggle pending balls visibility"},
				{"  ta", "  Show all states"},
			},
		},
		{
			title: "Balls Panel - Other Actions",
			items: []helpItem{
				{"j/k", "Navigate balls"},
				{"Space", "Toggle ball selection (multi-select)"},
				{"a", "Add new ball (tagged to current session)"},
				{"A", "Add followup ball (depends on selected ball)"},
				{"e", "Edit ball in built-in form"},
				{"E", "Edit ball in $EDITOR (YAML format)"},
				{"d", "Delete ball (with confirmation)"},
				{"y", "Copy ball ID to clipboard"},
				{"[ / ]", "Switch session (previous / next)"},
				{"o", "Toggle sort order (ID↑ → ID↓ → Priority → Activity)"},
				{"/", "Filter balls"},
				{"Ctrl+U", "Clear filter"},
			},
		},
		{
			title: "Activity Log Panel",
			items: []helpItem{
				{"j/k", "Scroll one line"},
				{"Ctrl+D", "Page down (half screen)"},
				{"Ctrl+U", "Page up (half screen)"},
				{"gg", "Go to top"},
				{"G", "Go to bottom"},
			},
		},
		{
			title: "Balls Panel - View Columns (v + key)",
			items: []helpItem{
				{"v", "Start two-key view columns sequence:"},
				{"  vp", "  Toggle priority column visibility"},
				{"  vt", "  Toggle tags column visibility"},
				{"  vm", "  Toggle model size column visibility"},
				{"  va", "  Toggle all optional columns on/off"},
			},
		},
		{
			title: "Balls Panel - Session Shortcuts (m/M + digit)",
			items: []helpItem{
				{"m", "Start two-key move ball sequence:"},
				{"  m1-m9,m0", "  Move ball to session 1-9 or 10 (replaces all sessions)"},
				{"M", "Start two-key append session sequence:"},
				{"  M1-M9,M0", "  Add ball to session 1-9 or 10 (keeps existing sessions)"},
				{"Backspace", "Remove ball from current session"},
			},
		},
		{
			title: "View Options",
			items: []helpItem{
				{"i", "Cycle bottom pane (activity → detail → split → activity)"},
				{"O", "Toggle agent output panel (shows live agent stdout)"},
				{"P", "Toggle project scope (local ↔ all projects)"},
				{"R", "Refresh / Reload data"},
				{"?", "Toggle this help"},
			},
		},
		{
			title: "Agent Control",
			items: []helpItem{
				{"W", "Enter agent monitor view (for running agent)"},
				{"X", "Cancel running agent (with confirmation)"},
				{"O", "Toggle agent output visibility"},
				{"E", "Expand/collapse agent output panel"},
				{"H", "View agent run history"},
			},
		},
		{
			title: "Bottom Pane Modes",
			items: []helpItem{
				{"[Act]", "Activity log - shows recent actions"},
				{"[Detail]", "Ball details - shows full ball info with ACs"},
				{"[Split]", "Split view - shows both details and activity"},
			},
		},
		{
			title: "Input Dialogs",
			items: []helpItem{
				{"Enter", "Submit / Confirm"},
				{"Esc", "Cancel"},
			},
		},
		{
			title: "Delete Confirmation",
			items: []helpItem{
				{"y", "Confirm delete"},
				{"n / Esc", "Cancel delete"},
			},
		},
		{
			title: "Quit",
			items: []helpItem{
				{"q / Ctrl+C", "Quit application"},
			},
		},
	}

	// Build content lines
	var lines []string
	for _, section := range sections {
		lines = append(lines, titleStyle.Render(section.title))
		for _, item := range section.items {
			keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Width(15)
			line := fmt.Sprintf("  %s  %s", keyStyle.Render(item.key), item.desc)
			lines = append(lines, line)
		}
		lines = append(lines, "") // Empty line between sections
	}

	// Calculate visible area
	availableHeight := m.height - 5 // Account for title and footer
	if availableHeight < 5 {
		availableHeight = 5
	}

	totalLines := len(lines)
	maxOffset := totalLines - availableHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	// Clamp scroll offset
	if m.helpScrollOffset > maxOffset {
		m.helpScrollOffset = maxOffset
	}
	if m.helpScrollOffset < 0 {
		m.helpScrollOffset = 0
	}

	// Show scroll indicator at top if not at beginning
	if m.helpScrollOffset > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ↑ %d more lines above", m.helpScrollOffset)) + "\n")
		availableHeight--
	}

	// Render visible lines
	endIdx := m.helpScrollOffset + availableHeight
	if endIdx > totalLines {
		endIdx = totalLines
	}

	for i := m.helpScrollOffset; i < endIdx; i++ {
		b.WriteString(lines[i] + "\n")
	}

	// Show scroll indicator at bottom if more content
	remaining := totalLines - endIdx
	if remaining > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ↓ %d more lines below", remaining)) + "\n")
	}

	// Footer
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Faint(true)
	b.WriteString(footerStyle.Render("j/k = scroll | ? or Esc = close help"))

	return b.String()
}

// renderHistoryView renders the agent run history view
func (m Model) renderHistoryView() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("33")).
		MarginBottom(1)
	b.WriteString(titleStyle.Render("📜 Agent Run History") + "\n\n")

	if len(m.agentHistory) == 0 {
		b.WriteString("No agent runs recorded yet.\n\n")
		b.WriteString(helpStyle.Render("Press H or Esc to return"))
		return b.String()
	}

	// Column headers
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	b.WriteString(headerStyle.Render(fmt.Sprintf("  %-19s  %-15s  %-6s  %-14s  %-8s  %-7s\n",
		"Date", "Session", "Iter", "Result", "Duration", "Balls")))
	b.WriteString(strings.Repeat("─", 80) + "\n")

	// Calculate visible area
	visibleLines := m.height - 10 // Account for header, footer
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Determine range to display
	startIdx := m.historyScrollOffset
	endIdx := startIdx + visibleLines
	if endIdx > len(m.agentHistory) {
		endIdx = len(m.agentHistory)
	}

	// Render history entries
	for i := startIdx; i < endIdx; i++ {
		record := m.agentHistory[i]

		// Format the entry
		cursor := "  "
		lineStyle := lipgloss.NewStyle()
		if i == m.historyCursor {
			cursor = "▶ "
			lineStyle = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
		}

		// Format date
		dateStr := record.StartedAt.Format("2006-01-02 15:04:05")

		// Format session (truncate if needed)
		sessionStr := record.SessionID
		if len(sessionStr) > 15 {
			sessionStr = sessionStr[:12] + "..."
		}

		// Format iterations
		iterStr := fmt.Sprintf("%d/%d", record.Iterations, record.MaxIterations)

		// Format result with styling
		resultStr := formatHistoryResult(record.Result)

		// Format duration
		duration := record.Duration()
		durationStr := formatDuration(duration)

		// Format balls
		ballsStr := fmt.Sprintf("%d/%d", record.BallsComplete, record.BallsTotal)

		line := fmt.Sprintf("%s%-19s  %-15s  %-6s  %-14s  %-8s  %-7s",
			cursor, dateStr, sessionStr, iterStr, resultStr, durationStr, ballsStr)
		b.WriteString(lineStyle.Render(line) + "\n")
	}

	// Scroll indicators
	if m.historyScrollOffset > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ↑ %d more above\n", m.historyScrollOffset)))
	}
	if endIdx < len(m.agentHistory) {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ↓ %d more below\n", len(m.agentHistory)-endIdx)))
	}

	b.WriteString("\n")

	// Show details for selected record
	if m.historyCursor < len(m.agentHistory) {
		record := m.agentHistory[m.historyCursor]
		detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		b.WriteString(detailStyle.Render("─── Selected Run Details ───") + "\n")

		if record.BlockedReason != "" {
			b.WriteString(detailStyle.Render(fmt.Sprintf("Blocked: %s\n", record.BlockedReason)))
		}
		if record.TimeoutMessage != "" {
			b.WriteString(detailStyle.Render(fmt.Sprintf("Timeout: %s\n", record.TimeoutMessage)))
		}
		if record.ErrorMessage != "" {
			b.WriteString(detailStyle.Render(fmt.Sprintf("Error: %s\n", record.ErrorMessage)))
		}
		if record.TotalWaitTime > 0 {
			b.WriteString(detailStyle.Render(fmt.Sprintf("Rate Limit Wait: %s\n", formatDuration(record.TotalWaitTime))))
		}
		if record.OutputFile != "" {
			b.WriteString(detailStyle.Render(fmt.Sprintf("Output: %s\n", record.OutputFile)))
		}
	}

	b.WriteString("\n")

	// Help
	help := lipgloss.NewStyle().Faint(true).Render("j/k = navigate | Enter = view output | H/Esc = close | gg/G = top/bottom")
	b.WriteString(help)

	return b.String()
}

// formatHistoryResult formats the result field with appropriate styling
func formatHistoryResult(result string) string {
	switch result {
	case "complete":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("✓ Complete")
	case "blocked":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("⊘ Blocked")
	case "timeout":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("⏱ Timeout")
	case "max_iterations":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Render("⟳ MaxIter")
	case "rate_limit":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("⚠ RateLimit")
	case "cancelled":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("✗ Cancelled")
	case "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗ Error")
	default:
		return result
	}
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// renderHistoryOutputView renders the output file content
func (m Model) renderHistoryOutputView() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("33")).
		MarginBottom(1)

	if m.historyCursor < len(m.agentHistory) {
		record := m.agentHistory[m.historyCursor]
		b.WriteString(titleStyle.Render(fmt.Sprintf("📄 Output: %s (%s)", record.SessionID, record.StartedAt.Format("2006-01-02 15:04"))) + "\n")
	} else {
		b.WriteString(titleStyle.Render("📄 Agent Output") + "\n")
	}
	b.WriteString(strings.Repeat("─", 80) + "\n")

	// Split content into lines
	lines := strings.Split(m.historyOutput, "\n")

	// Calculate visible area
	visibleLines := m.height - 6 // Account for header, footer
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Clamp offset
	maxOffset := len(lines) - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.historyOutputOffset
	if offset > maxOffset {
		offset = maxOffset
	}

	// Render visible lines
	endIdx := offset + visibleLines
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	for i := offset; i < endIdx; i++ {
		b.WriteString(lines[i] + "\n")
	}

	// Scroll indicators
	if offset > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("↑ %d lines above\n", offset)))
	}
	if endIdx < len(lines) {
		b.WriteString(helpStyle.Render(fmt.Sprintf("↓ %d lines below\n", len(lines)-endIdx)))
	}

	b.WriteString("\n")

	// Help
	help := lipgloss.NewStyle().Faint(true).Render("j/k = scroll | ctrl+d/u = page | gg/G = top/bottom | b/Esc = back to history")
	b.WriteString(help)

	return b.String()
}

// renderAutocompletePopup renders the file autocomplete suggestions popup
func (m Model) renderAutocompletePopup() string {
	if m.fileAutocomplete == nil || !m.fileAutocomplete.Active || len(m.fileAutocomplete.Suggestions) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	// Style for popup
	popupStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("6"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	var content strings.Builder
	for i, suggestion := range m.fileAutocomplete.Suggestions {
		if i == m.fileAutocomplete.Selected {
			content.WriteString(selectedStyle.Render(suggestion))
		} else {
			content.WriteString(normalStyle.Render(suggestion))
		}
		content.WriteString("\n")
	}
	// Remove trailing newline
	contentStr := strings.TrimSuffix(content.String(), "\n")

	b.WriteString(popupStyle.Render(contentStr))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Faint(true).Render("Tab = accept | ↑/↓ = select | Space = dismiss"))

	return b.String()
}

// renderUnifiedBallFormView renders the unified ball creation/editing form with all fields visible
// Field order: Context, Title, Acceptance Criteria, Tags, Session, Model Size, Agent Provider, Model Override, Priority, Blocking Reason, Depends On, Save
func (m Model) renderUnifiedBallFormView() string {
	var b strings.Builder

	// Show different title for create vs edit
	title := "Create New Ball"
	if m.inputAction == actionEdit && m.editingBall != nil {
		title = "Edit Ball: " + m.editingBall.ID
	}
	titleStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Render(title)
	b.WriteString(titleStyled + "\n\n")

	// Field indices are dynamic due to variable AC count
	// Order: Context(0), Title(1), ACs(2 to 2+len(ACs)), Tags, Session, ModelSize, AgentProvider, ModelOverride, Priority, BlockingReason, DependsOn, Save
	const (
		fieldContext = 0
		fieldIntent  = 1 // Title field (was intent)
		fieldACStart = 2 // ACs start at index 2
	)
	// Dynamic field indices calculated after ACs
	fieldACEnd := fieldACStart + len(m.pendingAcceptanceCriteria) // The "new AC" field
	fieldTags := fieldACEnd + 1
	fieldSession := fieldTags + 1
	fieldModelSize := fieldSession + 1
	fieldAgentProvider := fieldModelSize + 1
	fieldModelOverride := fieldAgentProvider + 1
	fieldPriority := fieldModelOverride + 1
	fieldBlockingReason := fieldPriority + 1
	fieldDependsOn := fieldBlockingReason + 1
	fieldSave := fieldDependsOn + 1
	fieldRunNow := fieldSave + 1

	// Build sessions list for display
	sessionOptions := []string{"(none)"}
	for _, sess := range m.sessions {
		if sess.ID != PseudoSessionAll && sess.ID != PseudoSessionUntagged {
			sessionOptions = append(sessionOptions, sess.ID)
		}
	}

	// Styles
	activeFieldStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	normalStyle := lipgloss.NewStyle()
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	optionSelectedStyle := lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("6")).Foreground(lipgloss.Color("0"))
	optionNormalStyle := lipgloss.NewStyle().Faint(true)
	acNumberStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	editingACStyle := lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("240"))
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow for warnings

	// --- Context field (multiline) ---
	labelStyle := normalStyle
	if m.pendingBallFormField == fieldContext {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Context:"))
	if m.pendingBallFormField == fieldContext {
		// Render textarea for editing
		b.WriteString("\n")
		b.WriteString(m.contextInput.View())
		// Show autocomplete popup if active on this field
		if popup := m.renderAutocompletePopup(); popup != "" {
			b.WriteString(popup)
		}
	} else {
		b.WriteString(" ")
		if m.pendingBallContext == "" {
			b.WriteString(optionNormalStyle.Render("(empty)"))
		} else {
			// Wrap long context to multiple lines (max 60 chars per line)
			wrapped := wrapText(m.pendingBallContext, 60)
			lines := strings.Split(wrapped, "\n")
			if len(lines) == 1 {
				b.WriteString(m.pendingBallContext)
			} else {
				// First line on same line as label
				b.WriteString(lines[0])
				// Subsequent lines indented to align with content
				indent := "         " // "Context: " is 9 chars
				for i := 1; i < len(lines); i++ {
					b.WriteString("\n" + indent + lines[i])
				}
			}
		}
	}
	b.WriteString("\n")

	// --- Title field (was Intent) ---
	labelStyle = normalStyle
	if m.pendingBallFormField == fieldIntent {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Title: "))
	if m.pendingBallFormField == fieldIntent {
		b.WriteString(m.textInput.View())
		// Show character count only when user has entered content (not when showing placeholder)
		titleLen := len(m.textInput.Value())
		if titleLen > 0 {
			countStyle := lipgloss.NewStyle().Faint(true)
			if titleLen >= 50 {
				countStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
			} else if titleLen >= 40 {
				countStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow
			}
			b.WriteString(countStyle.Render(fmt.Sprintf(" (%d/50)", titleLen)))
		}
		// Show autocomplete popup if active on this field
		if popup := m.renderAutocompletePopup(); popup != "" {
			b.WriteString(popup)
		}
	} else {
		if m.pendingBallIntent == "" {
			// Show context-derived placeholder when unfocused, or (empty) if no context
			if m.pendingBallContext != "" {
				placeholder := generateTitlePlaceholderFromContext(m.pendingBallContext)
				if placeholder != "" {
					b.WriteString(optionNormalStyle.Render(placeholder))
				} else {
					b.WriteString(optionNormalStyle.Render("(empty)"))
				}
			} else {
				b.WriteString(optionNormalStyle.Render("(empty)"))
			}
		} else {
			titleLen := len(m.pendingBallIntent)
			b.WriteString(m.pendingBallIntent)
			// Show character count with color if over 40 chars
			if titleLen >= 50 {
				countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
				b.WriteString(countStyle.Render(fmt.Sprintf(" (%d/50)", titleLen)))
			} else if titleLen >= 40 {
				countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow
				b.WriteString(countStyle.Render(fmt.Sprintf(" (%d/50)", titleLen)))
			}
		}
	}
	b.WriteString("\n\n")

	// --- Acceptance Criteria section (now after Title) ---
	acLabel := normalStyle
	isOnACField := m.pendingBallFormField >= fieldACStart && m.pendingBallFormField <= fieldACEnd
	if isOnACField {
		acLabel = activeFieldStyle
	}
	acHeaderText := "Acceptance Criteria:"
	// Show warning if empty AC list
	if len(m.pendingAcceptanceCriteria) == 0 && !isOnACField {
		acHeaderText += warningStyle.Render(" (none - consider adding criteria)")
	}
	b.WriteString(acLabel.Render(acHeaderText) + "\n")

	// Show existing ACs with ability to edit
	for i, ac := range m.pendingAcceptanceCriteria {
		acFieldIndex := fieldACStart + i
		if m.pendingBallFormField == acFieldIndex {
			// This AC is being edited
			b.WriteString(acNumberStyle.Render(fmt.Sprintf("  %d. ", i+1)))
			b.WriteString(m.textInput.View())
			// Show autocomplete popup if active on this AC field
			if popup := m.renderAutocompletePopup(); popup != "" {
				b.WriteString(popup)
			}
		} else {
			b.WriteString(acNumberStyle.Render(fmt.Sprintf("  %d. ", i+1)))
			b.WriteString(ac)
		}
		b.WriteString("\n")
	}

	// Show new AC input field (always at the end of ACs)
	if m.pendingBallFormField == fieldACEnd {
		// Show input for new AC
		b.WriteString(editingACStyle.Render("  + "))
		b.WriteString(m.textInput.View())
		// Show autocomplete popup if active on new AC field
		if popup := m.renderAutocompletePopup(); popup != "" {
			b.WriteString(popup)
		}
		b.WriteString("\n")
	} else {
		// Show pending new AC content if exists, otherwise show placeholder
		if m.pendingNewAC != "" {
			b.WriteString(acNumberStyle.Render("  + "))
			b.WriteString(m.pendingNewAC)
		} else {
			b.WriteString(optionNormalStyle.Render("  + (add criterion)"))
		}
		b.WriteString("\n")
	}

	// Show AC templates as selectable options (if any)
	if len(m.acTemplates) > 0 {
		templateLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
		b.WriteString(templateLabelStyle.Render("  Templates (j/k to navigate, Enter to add):") + "\n")
		for i, template := range m.acTemplates {
			cursor := "  "
			if m.acTemplateCursor == i {
				cursor = "> "
			}
			checkbox := "[ ]"
			templateStyle := optionNormalStyle
			if m.acTemplateSelected != nil && i < len(m.acTemplateSelected) && m.acTemplateSelected[i] {
				checkbox = "[✓]"
				templateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
			}
			// Highlight current selection
			if m.acTemplateCursor == i {
				highlightStyle := lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("240")).Foreground(lipgloss.Color("15"))
				b.WriteString(highlightStyle.Render(cursor + checkbox + " " + truncate(template, 53)) + "\n")
			} else {
				b.WriteString(templateStyle.Render(cursor + checkbox + " " + truncate(template, 53)) + "\n")
			}
		}
	}

	// Show repo/session level ACs as non-selectable reminders (if any)
	hasReminders := len(m.repoLevelACs) > 0 || len(m.sessionLevelACs) > 0
	if hasReminders {
		reminderLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
		reminderACStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		b.WriteString(reminderLabelStyle.Render("  Auto-applied (not stored on ball):") + "\n")
		for _, ac := range m.repoLevelACs {
			b.WriteString(reminderACStyle.Render("    [repo] " + truncate(ac, 50)) + "\n")
		}
		for _, ac := range m.sessionLevelACs {
			b.WriteString(reminderACStyle.Render("    [session] " + truncate(ac, 47)) + "\n")
		}
	}
	b.WriteString("\n")

	// --- Tags field ---
	labelStyle = normalStyle
	if m.pendingBallFormField == fieldTags {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Tags: "))
	if m.pendingBallFormField == fieldTags {
		b.WriteString(m.textInput.View())
	} else {
		if m.pendingBallTags == "" {
			b.WriteString(optionNormalStyle.Render("(empty)"))
		} else {
			b.WriteString(m.pendingBallTags)
		}
	}
	b.WriteString("\n")

	// --- Session field ---
	labelStyle = normalStyle
	if m.pendingBallFormField == fieldSession {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Session: "))
	for j, opt := range sessionOptions {
		if j > 0 {
			b.WriteString(" | ")
		}
		if j == m.pendingBallSession {
			if m.pendingBallFormField == fieldSession {
				b.WriteString(optionSelectedStyle.Render(opt))
			} else {
				b.WriteString(selectedStyle.Render(opt))
			}
		} else {
			b.WriteString(optionNormalStyle.Render(opt))
		}
	}
	b.WriteString("\n")

	// --- Model Size field ---
	modelSizes := []string{"(default)", "small", "medium", "large"}
	labelStyle = normalStyle
	if m.pendingBallFormField == fieldModelSize {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Model Size: "))
	for j, opt := range modelSizes {
		if j > 0 {
			b.WriteString(" | ")
		}
		if j == m.pendingBallModelSize {
			if m.pendingBallFormField == fieldModelSize {
				b.WriteString(optionSelectedStyle.Render(opt))
			} else {
				b.WriteString(selectedStyle.Render(opt))
			}
		} else {
			b.WriteString(optionNormalStyle.Render(opt))
		}
	}
	b.WriteString("\n")

	// --- Agent Provider field ---
	agentProviders := []string{"(default)", "claude", "opencode"}
	labelStyle = normalStyle
	if m.pendingBallFormField == fieldAgentProvider {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Agent Provider: "))
	for j, opt := range agentProviders {
		if j > 0 {
			b.WriteString(" | ")
		}
		if j == m.pendingBallAgentProvider {
			if m.pendingBallFormField == fieldAgentProvider {
				b.WriteString(optionSelectedStyle.Render(opt))
			} else {
				b.WriteString(selectedStyle.Render(opt))
			}
		} else {
			b.WriteString(optionNormalStyle.Render(opt))
		}
	}
	b.WriteString("\n")

	// --- Model Override field ---
	modelOverrides := []string{"(default)", "opus", "sonnet", "haiku"}
	labelStyle = normalStyle
	if m.pendingBallFormField == fieldModelOverride {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Model Override: "))
	for j, opt := range modelOverrides {
		if j > 0 {
			b.WriteString(" | ")
		}
		if j == m.pendingBallModelOverride {
			if m.pendingBallFormField == fieldModelOverride {
				b.WriteString(optionSelectedStyle.Render(opt))
			} else {
				b.WriteString(selectedStyle.Render(opt))
			}
		} else {
			b.WriteString(optionNormalStyle.Render(opt))
		}
	}
	b.WriteString("\n")

	// --- Priority field ---
	priorityOptions := []string{"low", "medium", "high", "urgent"}
	priorityColors := []string{"245", "6", "214", "196"} // gray, cyan, orange, red
	labelStyle = normalStyle
	if m.pendingBallFormField == fieldPriority {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Priority: "))
	for j, opt := range priorityOptions {
		if j > 0 {
			b.WriteString(" | ")
		}
		if j == m.pendingBallPriority {
			if m.pendingBallFormField == fieldPriority {
				b.WriteString(optionSelectedStyle.Render(opt))
			} else {
				// Use priority color for selected option when not focused
				colorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(priorityColors[j]))
				b.WriteString(colorStyle.Render(opt))
			}
		} else {
			b.WriteString(optionNormalStyle.Render(opt))
		}
	}
	b.WriteString("\n")

	// --- Blocking Reason field ---
	// Options: 0=(blank), 1=Human needed, 2=Waiting for dependency, 3=Needs research, 4=(custom)
	blockingReasonOptions := []string{"(blank)", "Human needed", "Waiting for dependency", "Needs research", "(custom)"}
	labelStyle = normalStyle
	if m.pendingBallFormField == fieldBlockingReason {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Blocking Reason: "))
	// Check if custom mode (4) with text input
	if m.pendingBallFormField == fieldBlockingReason && m.pendingBallBlockingReason == 4 {
		// Show text input for custom reason
		b.WriteString(m.textInput.View())
	} else {
		for j, opt := range blockingReasonOptions {
			if j > 0 {
				b.WriteString(" | ")
			}
			if j == m.pendingBallBlockingReason {
				if m.pendingBallFormField == fieldBlockingReason {
					b.WriteString(optionSelectedStyle.Render(opt))
				} else {
					// Show custom text if custom option selected
					if j == 4 && m.pendingBallCustomReason != "" {
						b.WriteString(selectedStyle.Render(m.pendingBallCustomReason))
					} else {
						b.WriteString(selectedStyle.Render(opt))
					}
				}
			} else {
				b.WriteString(optionNormalStyle.Render(opt))
			}
		}
	}
	b.WriteString("\n")

	// --- Depends On field ---
	labelStyle = normalStyle
	if m.pendingBallFormField == fieldDependsOn {
		labelStyle = activeFieldStyle
	}
	b.WriteString(labelStyle.Render("Depends On: "))
	if len(m.pendingBallDependsOn) == 0 {
		if m.pendingBallFormField == fieldDependsOn {
			b.WriteString(optionSelectedStyle.Render("(none) - press Enter to select"))
		} else {
			b.WriteString(optionNormalStyle.Render("(none)"))
		}
	} else {
		// Show selected dependencies
		depDisplay := strings.Join(m.pendingBallDependsOn, ", ")
		if m.pendingBallFormField == fieldDependsOn {
			b.WriteString(selectedStyle.Render(depDisplay) + optionNormalStyle.Render(" - press Enter to edit"))
		} else {
			b.WriteString(depDisplay)
		}
	}
	b.WriteString("\n\n")

	// --- Save button and Run now button (side by side) ---
	saveButtonStyle := lipgloss.NewStyle().Padding(0, 2)
	if m.pendingBallFormField == fieldSave {
		saveButtonStyle = saveButtonStyle.Bold(true).Background(lipgloss.Color("2")).Foreground(lipgloss.Color("0"))
	} else {
		saveButtonStyle = saveButtonStyle.Foreground(lipgloss.Color("2")).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("2"))
	}

	runNowButtonStyle := lipgloss.NewStyle().Padding(0, 2).MarginLeft(2)
	if m.pendingBallFormField == fieldRunNow {
		runNowButtonStyle = runNowButtonStyle.Bold(true).Background(lipgloss.Color("5")).Foreground(lipgloss.Color("0"))
	} else {
		runNowButtonStyle = runNowButtonStyle.Foreground(lipgloss.Color("5")).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("5"))
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, saveButtonStyle.Render("[ Save ]"), runNowButtonStyle.Render("[ Run now ]")) + "\n\n")

	// Show message if any
	if m.message != "" {
		b.WriteString(messageStyle.Render(m.message) + "\n\n")
	}

	// Help
	help := lipgloss.NewStyle().
		Faint(true).
		Render("↑/↓ = navigate | Tab = next | ←/→ = cycle options | Enter = next/add | Ctrl+S = save | Esc = cancel")
	b.WriteString(help)

	return b.String()
}

// wrapText wraps text to fit within maxWidth characters per line
func wrapText(text string, maxWidth int) string {
	if len(text) <= maxWidth {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)
		if i == 0 {
			// First word
			result.WriteString(word)
			lineLen = wordLen
		} else if lineLen+1+wordLen <= maxWidth {
			// Word fits on current line
			result.WriteString(" ")
			result.WriteString(word)
			lineLen += 1 + wordLen
		} else {
			// Start new line
			result.WriteString("\n")
			result.WriteString(word)
			lineLen = wordLen
		}
	}

	return result.String()
}
