package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/session"
	"github.com/spf13/cobra"
)

var (
	exportFormat      string
	exportOutput      string
	exportIncludeDone bool
	exportBallIDs     string
	exportFilterState string
	exportSession     string
	exportBallID      string // Single ball filter for focused agent prompts
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export balls to JSON, CSV, Ralph, or agent format",
	Long: `Export session data to JSON, CSV, Ralph, or agent format for analysis or agent use.

By default exports active balls (excluding complete) from the current project only.
Use --all to export from all discovered projects.
Use --include-done to also include complete balls.

Special session "all":
Use --session all to export ALL balls in the repo without session filtering.
This is useful for working on balls that aren't tagged to any specific session.

Filters are applied in order:
1. --session (if specified, exports only balls with matching session tag; "all" = no filter)
2. --ball-ids (if specified, only these balls)
3. --filter-state (if specified, only balls in these states)
4. --include-done (if false, excludes completed balls)

The Ralph format (--format ralph) is designed for agent loops and includes:
- <context> section from the session's context
- <progress> section from the session's progress.txt
- <tasks> section with balls, their state, priority, and acceptance criteria

The Agent format (--format agent) is a self-contained prompt for AI agents:
- <context> section from the session's context
- <progress> section with last 50 lines of progress.txt
- <balls> section with all session balls (state, acceptance criteria)
- <instructions> section with the agent prompt template
Can be piped directly to 'claude -p'.

Examples:
  # Export current project balls
  juggle export --format json --output balls.json

  # Export all discovered project balls
  juggle export --all --format csv

  # Export session in Ralph format for agent use
  juggle export --session my-feature --format ralph

  # Export ALL balls in repo as agent prompt (no session filter)
  juggle export --session all --format agent | claude -p

  # Export session as complete agent prompt
  juggle export --session my-feature --format agent | claude -p

  # Export specific balls by ID (supports full or short IDs)
  juggle export --ball-ids "juggle-5,48" --format json

  # Export only in_progress balls
  juggle export --filter-state in_progress --format json

  # Combine filters: export pending and in_progress balls from all projects
  juggle export --all --filter-state "pending,in_progress" --format csv`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVar(&exportFormat, "format", "json", "Export format: json, csv, ralph, or agent")
	exportCmd.Flags().StringVar(&exportOutput, "output", "", "Output file path (default: stdout)")
	exportCmd.Flags().BoolVar(&exportIncludeDone, "include-done", false, "Include complete balls in export (by default excluded from all formats)")
	exportCmd.Flags().StringVar(&exportBallIDs, "ball-ids", "", "Filter by specific ball IDs (comma-separated, supports full or short IDs)")
	exportCmd.Flags().StringVar(&exportFilterState, "filter-state", "", "Filter by states (comma-separated: pending, in_progress, blocked, complete)")
	exportCmd.Flags().StringVar(&exportSession, "session", "", "Export balls from a specific session (for ralph format, includes context and progress)")
	exportCmd.Flags().StringVar(&exportBallID, "ball", "", "Export a single ball by ID (for focused agent prompts)")
}

func runExport(cmd *cobra.Command, args []string) error {
	// Validate format
	if exportFormat != "json" && exportFormat != "csv" && exportFormat != "ralph" && exportFormat != "agent" {
		return fmt.Errorf("invalid format: %s (must be json, csv, ralph, or agent)", exportFormat)
	}

	// Ralph and agent formats require --session (but "all" is a special meta-session)
	if (exportFormat == "ralph" || exportFormat == "agent") && exportSession == "" {
		return fmt.Errorf("%s format requires --session flag (use 'all' for all balls in repo)", exportFormat)
	}

	// Get current directory
	cwd, err := GetWorkingDir()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Load config to discover projects
	config, err := LoadConfigForCommand()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create store for current directory (needed for DiscoverProjectsForCommand)
	store, err := NewStoreForCommand(cwd)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Discover projects (respects --all flag)
	projects, err := DiscoverProjectsForCommand(config, store)
	if err != nil {
		return fmt.Errorf("failed to discover projects: %w", err)
	}

	if len(projects) == 0 {
		return fmt.Errorf("no projects with .juggle directories found")
	}

	// Load all balls from discovered projects
	allBalls, err := session.LoadAllBalls(projects)
	if err != nil {
		return fmt.Errorf("failed to load balls: %w", err)
	}

	// Apply filters in order: session → ball-ids → filter-state → include-done
	balls := allBalls

	// Filter 0: --session (if specified, filter by session tag)
	// "all" is a special meta-session that means "all balls in repo" (no session filtering)
	if exportSession != "" && exportSession != "all" {
		filteredBalls := make([]*session.Ball, 0)
		for _, ball := range balls {
			for _, tag := range ball.Tags {
				if tag == exportSession {
					filteredBalls = append(filteredBalls, ball)
					break
				}
			}
		}
		balls = filteredBalls
	}

	// Filter 0.5: --ball (if specified, filter to single ball by ID)
	if exportBallID != "" {
		matches := session.ResolveBallByPrefix(balls, exportBallID)
		if len(matches) == 0 {
			return fmt.Errorf("ball not found: %s", exportBallID)
		}
		if len(matches) > 1 {
			matchingIDs := make([]string, len(matches))
			for i, m := range matches {
				matchingIDs[i] = m.ID
			}
			return fmt.Errorf("ambiguous ID '%s' matches %d balls: %s", exportBallID, len(matches), strings.Join(matchingIDs, ", "))
		}
		balls = []*session.Ball{matches[0]}
	}

	// Filter 1: --ball-ids (if specified)
	if exportBallIDs != "" {
		balls, err = filterByBallIDs(balls, exportBallIDs, projects)
		if err != nil {
			return err
		}
	}

	// Filter 2: --filter-state (if specified)
	if exportFilterState != "" {
		balls, err = filterByState(balls, exportFilterState)
		if err != nil {
			return err
		}
	}

	// Filter 3: --include-done (always applied - excludes complete balls unless flag is set)
	if !exportIncludeDone {
		filteredBalls := make([]*session.Ball, 0)
		for _, ball := range balls {
			if ball.State != session.StateComplete {
				filteredBalls = append(filteredBalls, ball)
			}
		}
		balls = filteredBalls
	}

	// Filter 4: For ralph/agent formats, exclude blocked balls (they require human intervention)
	// Warn if blocked balls were found and would have been included
	if exportFormat == "ralph" || exportFormat == "agent" {
		var blockedBalls []*session.Ball
		filteredBalls := make([]*session.Ball, 0)
		for _, ball := range balls {
			if ball.State == session.StateBlocked {
				blockedBalls = append(blockedBalls, ball)
			} else {
				filteredBalls = append(filteredBalls, ball)
			}
		}
		// Warn about excluded blocked balls (write to stderr so it doesn't pollute stdout export)
		if len(blockedBalls) > 0 {
			fmt.Fprintf(os.Stderr, "⚠ Warning: %d blocked ball(s) excluded from %s export:\n", len(blockedBalls), exportFormat)
			for _, ball := range blockedBalls {
				fmt.Fprintf(os.Stderr, "  - %s: %s\n", ball.ID, ball.Title)
			}
			fmt.Fprintln(os.Stderr)
		}
		balls = filteredBalls
	}

	// For ralph/agent formats, we allow empty balls (session might just have context)
	if len(balls) == 0 && exportFormat != "ralph" && exportFormat != "agent" {
		return fmt.Errorf("no balls to export")
	}

	// Export based on format
	var output []byte
	switch exportFormat {
	case "json":
		output, err = exportJSON(balls)
	case "csv":
		output, err = exportCSV(balls)
	case "ralph":
		output, err = exportRalph(cwd, exportSession, balls)
	case "agent":
		output, err = exportAgent(cwd, exportSession, balls, false, exportBallID != "") // debug only via agent run --debug
	}

	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}

	// Write to file or stdout
	if exportOutput != "" {
		if err := os.WriteFile(exportOutput, output, 0644); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
		fmt.Printf("✓ Exported %d ball(s) to %s\n", len(balls), exportOutput)
	} else {
		fmt.Print(string(output))
	}

	return nil
}

// filterByBallIDs filters balls by specific IDs (supports full and short IDs)
func filterByBallIDs(balls []*session.Ball, ballIDsStr string, projects []string) ([]*session.Ball, error) {
	// Parse comma-separated list
	idStrs := strings.Split(ballIDsStr, ",")
	requestedIDs := make([]string, 0, len(idStrs))
	for _, id := range idStrs {
		id = strings.TrimSpace(id)
		if id != "" {
			requestedIDs = append(requestedIDs, id)
		}
	}

	if len(requestedIDs) == 0 {
		return balls, nil
	}

	// Resolve each requested ID using prefix matching
	filteredBalls := make([]*session.Ball, 0)
	seenBalls := make(map[string]bool)

	for _, requestedID := range requestedIDs {
		matches := session.ResolveBallByPrefix(balls, requestedID)
		if len(matches) == 0 {
			return nil, fmt.Errorf("ball ID not found: %s", requestedID)
		}
		if len(matches) > 1 {
			matchingIDs := make([]string, len(matches))
			for i, m := range matches {
				matchingIDs[i] = m.ID
			}
			return nil, fmt.Errorf("ambiguous ID '%s' matches %d balls: %s", requestedID, len(matches), strings.Join(matchingIDs, ", "))
		}
		ball := matches[0]
		if !seenBalls[ball.ID] {
			filteredBalls = append(filteredBalls, ball)
			seenBalls[ball.ID] = true
		}
	}

	return filteredBalls, nil
}

// filterByState filters balls by state(s)
// Supports states: "pending", "in_progress", "blocked", "complete", "researched"
func filterByState(balls []*session.Ball, stateStr string) ([]*session.Ball, error) {
	// Parse comma-separated list
	stateStrs := strings.Split(stateStr, ",")
	stateFilters := make([]session.BallState, 0, len(stateStrs))

	for _, s := range stateStrs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		if !session.ValidateBallState(s) {
			return nil, fmt.Errorf("invalid state: %s (must be pending, in_progress, blocked, complete, or researched)", s)
		}
		stateFilters = append(stateFilters, session.BallState(s))
	}

	if len(stateFilters) == 0 {
		return balls, nil
	}

	// Filter balls
	filteredBalls := make([]*session.Ball, 0)
	for _, ball := range balls {
		for _, filter := range stateFilters {
			if ball.State == filter {
				filteredBalls = append(filteredBalls, ball)
				break
			}
		}
	}

	return filteredBalls, nil
}

func exportJSON(balls []*session.Ball) ([]byte, error) {
	// Create export structure
	export := struct {
		ExportedAt string             `json:"exported_at"`
		TotalBalls int                `json:"total_balls"`
		Balls      []*session.Ball `json:"balls"`
	}{
		ExportedAt: fmt.Sprintf("%d", 1),
		TotalBalls: len(balls),
		Balls:      balls,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, err
	}

	return data, nil
}

func exportCSV(balls []*session.Ball) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"ID",
		"Project",
		"Intent",
		"Priority",
		"State",
		"BlockedReason",
		"StartedAt",
		"CompletedAt",
		"LastActivity",
		"Tags",
		"CompletionNote",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write rows
	for _, ball := range balls {
		completedAt := ""
		if ball.CompletedAt != nil {
			completedAt = ball.CompletedAt.Format("2006-01-02 15:04:05")
		}

		tags := strings.Join(ball.Tags, ";")

		row := []string{
			ball.ID,
			ball.WorkingDir,
			ball.Title,
			string(ball.Priority),
			string(ball.State),
			ball.BlockedReason,
			ball.StartedAt.Format("2006-01-02 15:04:05"),
			completedAt,
			ball.LastActivity.Format("2006-01-02 15:04:05"),
			tags,
			ball.CompletionNote,
		}

		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return []byte(buf.String()), nil
}

// exportRalph exports session data in Ralph agent format
// Format:
// <context>
// [session context]
// </context>
//
// <progress>
// [progress.txt content]
// </progress>
//
// <global-acceptance-criteria>
// [repo and session level ACs]
// </global-acceptance-criteria>
//
// <tasks>
// [balls with acceptance criteria]
// </tasks>
func exportRalph(projectDir, sessionID string, balls []*session.Ball) ([]byte, error) {
	var buf strings.Builder

	// Load session store to get context and progress
	sessionStore, err := session.NewSessionStore(projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	// Try to load the session
	juggleSession, err := sessionStore.LoadSession(sessionID)
	if err != nil {
		// Session might not exist yet, that's okay for Ralph export
		// We'll just have empty context
		juggleSession = &session.JuggleSession{
			ID:          sessionID,
			Description: "",
			Context:     "",
		}
	}

	// Load progress
	progress, _ := sessionStore.LoadProgress(sessionID) // Ignore error, empty progress is fine

	// Load repo-level acceptance criteria
	repoACs, _ := session.GetProjectAcceptanceCriteria(projectDir) // Ignore error

	// Write <context> section
	buf.WriteString("<context>\n")
	if juggleSession.Description != "" {
		buf.WriteString("# " + juggleSession.Description + "\n\n")
	}
	if juggleSession.Context != "" {
		buf.WriteString(juggleSession.Context)
		if !strings.HasSuffix(juggleSession.Context, "\n") {
			buf.WriteString("\n")
		}
	}
	buf.WriteString("</context>\n\n")

	// Write <progress> section
	buf.WriteString("<progress>\n")
	if progress != "" {
		buf.WriteString(progress)
		if !strings.HasSuffix(progress, "\n") {
			buf.WriteString("\n")
		}
	}
	buf.WriteString("</progress>\n\n")

	// Write <global-acceptance-criteria> section if any exist
	if len(repoACs) > 0 || len(juggleSession.AcceptanceCriteria) > 0 {
		buf.WriteString("<global-acceptance-criteria>\n")
		buf.WriteString("These criteria apply to ALL tasks in this session:\n\n")

		acIndex := 1
		if len(repoACs) > 0 {
			buf.WriteString("## Repository-Level Requirements\n")
			for _, ac := range repoACs {
				buf.WriteString(fmt.Sprintf("  %d. %s\n", acIndex, ac))
				acIndex++
			}
		}
		if len(juggleSession.AcceptanceCriteria) > 0 {
			if len(repoACs) > 0 {
				buf.WriteString("\n## Session-Level Requirements\n")
			} else {
				buf.WriteString("## Session-Level Requirements\n")
			}
			for _, ac := range juggleSession.AcceptanceCriteria {
				buf.WriteString(fmt.Sprintf("  %d. %s\n", acIndex, ac))
				acIndex++
			}
		}
		buf.WriteString("</global-acceptance-criteria>\n\n")
	}

	// Sort balls: in_progress first (implies unfinished work), then by priority
	sortBallsForAgent(balls)

	// Write <tasks> section
	buf.WriteString("<tasks>\n")
	for i, ball := range balls {
		if i > 0 {
			buf.WriteString("\n")
		}
		writeBallForRalph(&buf, ball)
	}
	buf.WriteString("</tasks>\n")

	return []byte(buf.String()), nil
}

// writeBallForRalph writes a single ball in Ralph format
func writeBallForRalph(buf *strings.Builder, ball *session.Ball) {
	// Task header with ID, state, and priority
	header := fmt.Sprintf("## %s [%s] (priority: %s)", ball.ID, ball.State, ball.Priority)
	if ball.ModelSize != "" {
		header += fmt.Sprintf(" (model: %s)", ball.ModelSize)
	}
	buf.WriteString(header + "\n")

	// Title
	buf.WriteString(fmt.Sprintf("Title: %s\n", ball.Title))

	// Context (full task description if different from title)
	if ball.Context != "" && ball.Context != ball.Title {
		buf.WriteString(fmt.Sprintf("Context: %s\n", ball.Context))
	}

	// Acceptance criteria (preferred over deprecated Description)
	if len(ball.AcceptanceCriteria) > 0 {
		buf.WriteString("Acceptance Criteria:\n")
		for i, ac := range ball.AcceptanceCriteria {
			buf.WriteString(fmt.Sprintf("  %d. %s\n", i+1, ac))
		}
	}

	// Dependencies
	if len(ball.DependsOn) > 0 {
		buf.WriteString(fmt.Sprintf("Depends On: %s\n", strings.Join(ball.DependsOn, ", ")))
	}

	// Blocked reason if blocked
	if ball.State == session.StateBlocked && ball.BlockedReason != "" {
		buf.WriteString(fmt.Sprintf("Blocked: %s\n", ball.BlockedReason))
	}

	// Tags
	if len(ball.Tags) > 0 {
		buf.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(ball.Tags, ", ")))
	}
}

// exportAgent exports session data in self-contained agent prompt format
// Format:
// <context>
// [session context]
// </context>
//
// <progress>
// [last 50 lines of progress.txt]
// </progress>
//
// <global-acceptance-criteria>
// [repo and session level ACs]
// </global-acceptance-criteria>
//
// <balls> or <task> (if singleBall)
// [balls with state and acceptance criteria]
// </balls> or </task>
//
// <instructions>
// [agent prompt template]
// [optional debug instructions]
// </instructions>
func exportAgent(projectDir, sessionID string, balls []*session.Ball, debug bool, singleBall bool) ([]byte, error) {
	var buf strings.Builder

	// Load session store to get context and progress
	sessionStore, err := session.NewSessionStore(projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	// Try to load the session
	juggleSession, err := sessionStore.LoadSession(sessionID)
	if err != nil {
		// Session might not exist yet, that's okay
		juggleSession = &session.JuggleSession{
			ID:          sessionID,
			Description: "",
			Context:     "",
		}
	}

	// Load progress and limit to last 50 lines
	progress, _ := sessionStore.LoadProgress(sessionID) // Ignore error, empty progress is fine
	progress = limitToLastLines(progress, 50)

	// Load repo-level acceptance criteria
	repoACs, _ := session.GetProjectAcceptanceCriteria(projectDir) // Ignore error

	// Write <context> section
	buf.WriteString("<context>\n")
	if juggleSession.Description != "" {
		buf.WriteString("# " + juggleSession.Description + "\n\n")
	}
	if juggleSession.Context != "" {
		buf.WriteString(juggleSession.Context)
		if !strings.HasSuffix(juggleSession.Context, "\n") {
			buf.WriteString("\n")
		}
	}
	buf.WriteString("</context>\n\n")

	// Write <session> section with the session ID
	// This tells the agent which session it's working on for progress commands
	buf.WriteString("<session>\n")
	buf.WriteString(sessionID)
	buf.WriteString("\n</session>\n\n")

	// Write <progress> section
	buf.WriteString("<progress>\n")
	if progress != "" {
		buf.WriteString(progress)
		if !strings.HasSuffix(progress, "\n") {
			buf.WriteString("\n")
		}
	}
	buf.WriteString("</progress>\n\n")

	// Write <global-acceptance-criteria> section if any exist
	if len(repoACs) > 0 || len(juggleSession.AcceptanceCriteria) > 0 {
		buf.WriteString("<global-acceptance-criteria>\n")
		buf.WriteString("These criteria apply to ALL tasks in this session:\n\n")

		acIndex := 1
		if len(repoACs) > 0 {
			buf.WriteString("## Repository-Level Requirements\n")
			for _, ac := range repoACs {
				buf.WriteString(fmt.Sprintf("  %d. %s\n", acIndex, ac))
				acIndex++
			}
		}
		if len(juggleSession.AcceptanceCriteria) > 0 {
			if len(repoACs) > 0 {
				buf.WriteString("\n## Session-Level Requirements\n")
			} else {
				buf.WriteString("## Session-Level Requirements\n")
			}
			for _, ac := range juggleSession.AcceptanceCriteria {
				buf.WriteString(fmt.Sprintf("  %d. %s\n", acIndex, ac))
				acIndex++
			}
		}
		buf.WriteString("</global-acceptance-criteria>\n\n")
	}

	// Sort balls: in_progress first (implies unfinished work), then by priority
	sortBallsForAgent(balls)

	// Write <balls> or <task> section
	if singleBall && len(balls) == 1 {
		// Single ball mode: focused task format
		buf.WriteString("<task>\n")
		buf.WriteString("This is your task:\n\n")
		writeBallForAgent(&buf, balls[0])
		buf.WriteString("</task>\n\n")
	} else {
		// Multi-ball session mode
		buf.WriteString("<balls>\n")
		for i, ball := range balls {
			if i > 0 {
				buf.WriteString("\n")
			}
			writeBallForAgent(&buf, ball)
		}
		buf.WriteString("</balls>\n\n")
	}

	// Write <instructions> section with agent prompt template
	buf.WriteString("<instructions>\n")

	if singleBall && len(balls) == 1 {
		// Single ball mode: task-focused instructions
		buf.WriteString("You are working on a single task. Complete the acceptance criteria above.\n\n")
		buf.WriteString("When done, output one of these signals:\n")
		buf.WriteString("- `<promise>COMPLETE</promise>` - Task is finished\n")
		buf.WriteString("- `<promise>BLOCKED: reason</promise>` - Task cannot proceed\n")
	} else {
		// Multi-ball session mode: full agent prompt
		buf.WriteString(agent.GetPromptTemplate())
		if !strings.HasSuffix(agent.GetPromptTemplate(), "\n") {
			buf.WriteString("\n")
		}
	}

	// Inject debug instructions if enabled
	if debug {
		buf.WriteString("\n## DEBUG MODE\n\n")
		buf.WriteString("Before outputting your completion signal, explain WHY you chose that signal.\n")
	}

	buf.WriteString("</instructions>\n")

	return []byte(buf.String()), nil
}

// limitToLastLines returns the last n lines of a string
func limitToLastLines(s string, n int) string {
	if s == "" {
		return ""
	}

	lines := strings.Split(s, "\n")

	// Remove trailing empty line if present (from trailing newline)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}

	return strings.Join(lines[len(lines)-n:], "\n")
}

// writeBallForAgent writes a single ball in agent format
func writeBallForAgent(buf *strings.Builder, ball *session.Ball) {
	// Ball header with ID, state, and priority
	header := fmt.Sprintf("## %s [%s] (priority: %s)", ball.ID, ball.State, ball.Priority)
	if ball.ModelSize != "" {
		header += fmt.Sprintf(" (model: %s)", ball.ModelSize)
	}
	buf.WriteString(header + "\n")

	// Title
	buf.WriteString(fmt.Sprintf("Title: %s\n", ball.Title))

	// Context (full task description if different from title)
	if ball.Context != "" && ball.Context != ball.Title {
		buf.WriteString(fmt.Sprintf("Context: %s\n", ball.Context))
	}

	// Acceptance criteria
	if len(ball.AcceptanceCriteria) > 0 {
		buf.WriteString("Acceptance Criteria:\n")
		for i, ac := range ball.AcceptanceCriteria {
			buf.WriteString(fmt.Sprintf("  %d. %s\n", i+1, ac))
		}
	}

	// Dependencies
	if len(ball.DependsOn) > 0 {
		buf.WriteString(fmt.Sprintf("Depends On: %s\n", strings.Join(ball.DependsOn, ", ")))
	}

	// Blocked reason if blocked
	if ball.State == session.StateBlocked && ball.BlockedReason != "" {
		buf.WriteString(fmt.Sprintf("Blocked: %s\n", ball.BlockedReason))
	}

	// Tags
	if len(ball.Tags) > 0 {
		buf.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(ball.Tags, ", ")))
	}
}

// SortBallsForAgentExport sorts balls so in_progress balls come first,
// followed by pending balls, then blocked balls.
// Complete balls should be filtered out before calling this.
// Within each state, balls are sorted by priority (urgent > high > medium > low).
// This is exported for testing.
func SortBallsForAgentExport(balls []*session.Ball) {
	sortBallsForAgent(balls)
}

// sortBallsForAgent sorts balls so in_progress balls come first,
// followed by pending balls, then blocked balls.
// Complete balls should be filtered out before calling this.
// Within each state, balls are sorted by:
// 1. Dependencies satisfied (balls with all deps complete come first)
// 2. Priority (urgent > high > medium > low)
func sortBallsForAgent(balls []*session.Ball) {
	// Build a map of ball states for dependency checking
	ballStates := make(map[string]session.BallState)
	for _, ball := range balls {
		ballStates[ball.ID] = ball.State
		// Also map by short ID for dependency resolution
		ballStates[ball.ShortID()] = ball.State
	}

	// Helper to check if all dependencies are satisfied (complete or researched)
	allDepsSatisfied := func(ball *session.Ball) bool {
		if len(ball.DependsOn) == 0 {
			return true
		}
		for _, depID := range ball.DependsOn {
			state, exists := ballStates[depID]
			if !exists {
				// Dependency not in current set - assume satisfied
				continue
			}
			if state != session.StateComplete && state != session.StateResearched {
				return false
			}
		}
		return true
	}

	// State priority: in_progress first, then pending, then blocked, then complete
	// blocked balls are filtered out before reaching this sort for agent exports
	stateOrder := map[session.BallState]int{
		session.StateInProgress: 0,
		session.StatePending:    1,
		session.StateBlocked:    2,
		session.StateComplete:   3,
		session.StateResearched: 4,
	}

	// Priority order: urgent > high > medium > low
	priorityOrder := map[session.Priority]int{
		session.PriorityUrgent: 0,
		session.PriorityHigh:   1,
		session.PriorityMedium: 2,
		session.PriorityLow:    3,
	}

	sort.SliceStable(balls, func(i, j int) bool {
		// First sort by state
		stateI := stateOrder[balls[i].State]
		stateJ := stateOrder[balls[j].State]
		if stateI != stateJ {
			return stateI < stateJ
		}

		// Then sort by dependency satisfaction (satisfied deps first)
		depsSatI := allDepsSatisfied(balls[i])
		depsSatJ := allDepsSatisfied(balls[j])
		if depsSatI != depsSatJ {
			return depsSatI // true (satisfied) comes before false (unsatisfied)
		}

		// Then sort by priority within each state
		priorityI := priorityOrder[balls[i].Priority]
		priorityJ := priorityOrder[balls[j].Priority]
		return priorityI < priorityJ
	})
}
