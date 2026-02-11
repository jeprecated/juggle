package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Monitor view styles
var (
	monitorTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("3")). // Yellow
				Padding(0, 1)

	monitorProgressBarFilled = lipgloss.NewStyle().
					Foreground(lipgloss.Color("2")). // Green
					Render("█")

	monitorProgressBarEmpty = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")). // Gray
				Render("░")

	monitorMetricLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Width(15)

	monitorMetricValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("6")) // Cyan

	monitorControlsStyle = lipgloss.NewStyle().
				Faint(true)

	monitorSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// renderAgentMonitorView renders the full-screen agent monitoring dashboard
func (m Model) renderAgentMonitorView() string {
	// Guard against rendering before window size is received
	if m.width < 40 || m.height < 15 {
		return "Loading..."
	}

	var b strings.Builder

	// Title bar with session info
	b.WriteString(m.renderMonitorTitleBar())
	b.WriteString("\n")

	// Progress bar
	b.WriteString(m.renderMonitorProgressBar())
	b.WriteString("\n")

	// Calculate output section height
	// title(2) + progress(2) + metrics(7 with phase) + controls(2) + margins
	outputHeight := m.height - 15
	if outputHeight < 5 {
		outputHeight = 5
	}

	// Output section (reuses existing agent output panel rendering)
	b.WriteString(m.renderMonitorOutputSection(outputHeight))

	// Separator
	b.WriteString(monitorSeparatorStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Metrics panel
	b.WriteString(m.renderMonitorMetricsPanel())

	// Controls panel
	b.WriteString(m.renderMonitorControlsPanel())

	return b.String()
}

// renderMonitorTitleBar renders the top title bar with status
func (m Model) renderMonitorTitleBar() string {
	var status string
	statusColor := lipgloss.Color("2") // Green

	if m.agentDaemonError != "" {
		status = "⚠ Error"
		statusColor = lipgloss.Color("1") // Red
	} else if m.agentMonitorPaused {
		status = "Pausing..."
		statusColor = lipgloss.Color("3") // Yellow
	} else if !m.agentStatus.Running {
		if m.agentStatus.Status != "" {
			status = m.agentStatus.Status
		} else {
			status = "Stopped"
		}
		statusColor = lipgloss.Color("1") // Red
	} else {
		// Running with spinner animation
		status = m.agentSpinner.View() + " Running"
	}

	statusStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(statusColor)

	title := fmt.Sprintf("Agent Monitor: %s [%s] - Iteration %d/%d",
		m.agentStatus.SessionID,
		statusStyle.Render(status),
		m.agentStatus.Iteration,
		m.agentStatus.MaxIterations)

	if m.agentMonitorReconnected {
		title += lipgloss.NewStyle().Faint(true).Render(" (reconnected)")
	}

	return monitorTitleStyle.Render(title)
}

// renderMonitorProgressBar renders the iteration progress bar
func (m Model) renderMonitorProgressBar() string {
	width := m.width - 12 // Leave room for percentage
	if width < 20 {
		width = 20
	}

	var progress float64
	if m.agentStatus.MaxIterations > 0 {
		progress = float64(m.agentStatus.Iteration) / float64(m.agentStatus.MaxIterations)
	}

	filled := int(float64(width) * progress)
	if filled > width {
		filled = width
	}
	empty := width - filled

	percentage := fmt.Sprintf(" %3.0f%%", progress*100)

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	return "  " + filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty)) +
		percentage + "\n"
}

// renderMonitorOutputSection renders the scrollable output section
func (m Model) renderMonitorOutputSection(height int) string {
	var b strings.Builder

	// Section title with scroll position
	title := "Output"
	if len(m.agentOutput) > 0 {
		title = fmt.Sprintf("Output [%d/%d]", m.agentOutputOffset+1, len(m.agentOutput))
	}
	titleStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		Render(title)
	b.WriteString("  " + titleStyled + "\n")
	b.WriteString("  " + monitorSeparatorStyle.Render(strings.Repeat("─", m.width-4)) + "\n")

	// Show error banner if there's an error
	if m.agentDaemonError != "" {
		errorStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")). // White
			Background(lipgloss.Color("1")).  // Red background
			Padding(0, 1)
		errorBanner := errorStyle.Render("⚠ DAEMON ERROR")
		b.WriteString("  " + errorBanner + "\n")
		errorMsgStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")). // Red
			Bold(true)
		// Truncate error message if too long
		errMsg := m.agentDaemonError
		if len(errMsg) > m.width-6 {
			errMsg = errMsg[:m.width-9] + "..."
		}
		b.WriteString("  " + errorMsgStyle.Render(errMsg) + "\n")
		b.WriteString("\n")
		height -= 3 // Account for error banner
	}

	if len(m.agentOutput) == 0 {
		emptyMsg := "  No agent output"
		if !m.agentStatus.Running {
			emptyMsg += " - Press Esc to return"
		}
		b.WriteString(helpStyle.Render(emptyMsg) + "\n")
		// Pad remaining height
		for i := 0; i < height-3; i++ {
			b.WriteString("\n")
		}
		return b.String()
	}

	// Calculate visible range
	visibleLines := height - 2 // Account for title and separator
	if visibleLines < 1 {
		visibleLines = 1
	}

	startIdx := m.agentOutputOffset
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + visibleLines
	if endIdx > len(m.agentOutput) {
		endIdx = len(m.agentOutput)
	}

	// Check if we need scroll indicators
	needTopIndicator := startIdx > 0
	needBottomIndicator := endIdx < len(m.agentOutput)

	// Adjust visible lines for scroll indicators
	if needTopIndicator {
		visibleLines--
		b.WriteString(helpStyle.Render("  ▲ more above") + "\n")
	}
	if needBottomIndicator {
		visibleLines--
	}

	// Recalculate end index after indicator adjustment
	endIdx = startIdx + visibleLines
	if endIdx > len(m.agentOutput) {
		endIdx = len(m.agentOutput)
	}

	// Render visible lines
	for i := startIdx; i < endIdx; i++ {
		entry := m.agentOutput[i]
		line := entry.Line
		// Truncate long lines
		if len(line) > m.width-4 {
			line = line[:m.width-7] + "..."
		}
		if entry.IsError {
			b.WriteString("  " + errorStyle.Render(line) + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}

	// Pad if we have fewer lines than space
	linesRendered := endIdx - startIdx
	if needTopIndicator {
		linesRendered++
	}
	for i := linesRendered; i < height-2; i++ {
		b.WriteString("\n")
	}

	if needBottomIndicator {
		b.WriteString(helpStyle.Render("  ▼ more below") + "\n")
	}

	return b.String()
}

// renderMonitorMetricsPanel renders the metrics section
func (m Model) renderMonitorMetricsPanel() string {
	var b strings.Builder

	// Get current ball info from agent status
	sessionID := "—"
	ballID := "—"
	ballTitle := "—"
	acProgress := "—"
	startTime := "—"
	elapsed := "—"
	model := "—"

	if m.agentStatus.SessionID != "" {
		sessionID = m.agentStatus.SessionID
	}

	if m.agentStatus.CurrentBallID != "" {
		ballID = m.agentStatus.CurrentBallID
		if m.agentStatus.CurrentBallTitle != "" {
			// Truncate title if too long
			title := m.agentStatus.CurrentBallTitle
			if len(title) > 30 {
				title = title[:27] + "..."
			}
			ballTitle = title
		}
	}

	if m.agentStatus.ACsTotal > 0 {
		acProgress = fmt.Sprintf("%d/%d", m.agentStatus.ACsComplete, m.agentStatus.ACsTotal)
	}

	if m.agentStatus.Model != "" {
		model = m.agentStatus.Model
	}

	if !m.agentMonitorStartTime.IsZero() {
		startTime = m.agentMonitorStartTime.Format("15:04:05")
		elapsed = formatDuration(time.Since(m.agentMonitorStartTime))
	}

	// Get phase info
	phase := "—"
	phaseMessage := ""
	if m.agentStatus.Phase != "" {
		phase = m.agentStatus.Phase
		if m.agentStatus.PhaseMessage != "" {
			// Truncate message if too long
			msg := m.agentStatus.PhaseMessage
			if len(msg) > 40 {
				msg = msg[:37] + "..."
			}
			phaseMessage = msg
		}
	}

	// Row 1: Session and Started
	b.WriteString(fmt.Sprintf("  %s %s    %s %s\n",
		monitorMetricLabelStyle.Render("Session:"),
		monitorMetricValueStyle.Render(sessionID),
		monitorMetricLabelStyle.Render("Started:"),
		monitorMetricValueStyle.Render(startTime)))

	// Row 2: Ball ID and AC Progress
	b.WriteString(fmt.Sprintf("  %s %s    %s %s\n",
		monitorMetricLabelStyle.Render("Ball:"),
		monitorMetricValueStyle.Render(ballID),
		monitorMetricLabelStyle.Render("ACs:"),
		monitorMetricValueStyle.Render(acProgress)))

	// Row 3: Ball Title and Elapsed
	b.WriteString(fmt.Sprintf("  %s %s    %s %s\n",
		monitorMetricLabelStyle.Render("Title:"),
		monitorMetricValueStyle.Render(ballTitle),
		monitorMetricLabelStyle.Render("Elapsed:"),
		monitorMetricValueStyle.Render(elapsed)))

	// Row 4: Model and Phase
	b.WriteString(fmt.Sprintf("  %s %s    %s %s\n",
		monitorMetricLabelStyle.Render("Model:"),
		monitorMetricValueStyle.Render(model),
		monitorMetricLabelStyle.Render("Phase:"),
		monitorMetricValueStyle.Render(phase)))

	// Row 5: Phase message (if present)
	if phaseMessage != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n",
			monitorMetricLabelStyle.Render("Status:"),
			monitorMetricValueStyle.Render(phaseMessage)))
	}

	// Row 6: Hook metrics (if available)
	if m.agentMetrics != nil && m.agentMetrics.TotalTools > 0 {
		// Format tools info
		toolsInfo := fmt.Sprintf("%d", m.agentMetrics.TotalTools)
		if m.agentMetrics.ToolFailures > 0 {
			toolsInfo += fmt.Sprintf(" (%d failed)", m.agentMetrics.ToolFailures)
		}

		// Format files info
		filesInfo := fmt.Sprintf("%d", len(m.agentMetrics.FilesChanged))

		// Format tokens info
		tokensInfo := "—"
		totalTokens := m.agentMetrics.InputTokens + m.agentMetrics.OutputTokens
		if totalTokens > 0 {
			tokensInfo = formatTokenCount(totalTokens)
			if m.agentMetrics.CacheReadTokens > 0 {
				tokensInfo += fmt.Sprintf(" (cache: %s)", formatTokenCount(m.agentMetrics.CacheReadTokens))
			}
		}

		b.WriteString(fmt.Sprintf("  %s %s    %s %s    %s %s\n",
			monitorMetricLabelStyle.Render("Tools:"),
			monitorMetricValueStyle.Render(toolsInfo),
			monitorMetricLabelStyle.Render("Files:"),
			monitorMetricValueStyle.Render(filesInfo),
			monitorMetricLabelStyle.Render("Tokens:"),
			monitorMetricValueStyle.Render(tokensInfo)))
	}

	// Row 7: Live Stream-JSON Metrics (if active)
	if m.agentStatus.StreamJSONActive {
		liveTokensInfo := fmt.Sprintf("%s in, %s out",
			formatTokenCount(m.agentStatus.LiveInputTokens),
			formatTokenCount(m.agentStatus.LiveOutputTokens))
		if m.agentStatus.LiveCacheTokens > 0 {
			liveTokensInfo += fmt.Sprintf(", %s cached", formatTokenCount(m.agentStatus.LiveCacheTokens))
		}

		b.WriteString(fmt.Sprintf("  %s %s\n",
			monitorMetricLabelStyle.Render("Live Tokens:"),
			monitorMetricValueStyle.Render(liveTokensInfo)))

		// Active tool indicator
		if m.agentStatus.ActiveTool != "" {
			toolIndicator := fmt.Sprintf("🔧 %s", m.agentStatus.ActiveTool)
			b.WriteString(fmt.Sprintf("  %s %s\n",
				monitorMetricLabelStyle.Render("Active Tool:"),
				monitorMetricValueStyle.Render(toolIndicator)))
		}

		// Thinking indicator
		if m.agentStatus.ThinkingActive {
			b.WriteString(fmt.Sprintf("  %s\n",
				monitorMetricValueStyle.Render("🤔 Thinking...")))
		}
	}

	return b.String()
}

// formatTokenCount formats token counts with K/M suffixes
func formatTokenCount(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// renderMonitorControlsPanel renders the controls help line
func (m Model) renderMonitorControlsPanel() string {
	var controls []string

	if m.agentStatus.Running {
		if m.agentMonitorPaused {
			controls = append(controls, "r:Resume")
		} else {
			controls = append(controls, "p:Pause")
		}
		controls = append(controls,
			"m:Model",
			"n:Skip ball",
			"X:Cancel",
		)
	}

	controls = append(controls,
		"Esc:Back",
		"q:Detach",
	)

	return "\n  " + monitorControlsStyle.Render(strings.Join(controls, " | "))
}
