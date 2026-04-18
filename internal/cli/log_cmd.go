package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var logFlags struct {
	latest bool
	json   bool
}

var logCmd = &cobra.Command{
	Use:   "log [run-id]",
	Short: "View juggle session logs",
	Long: `Query and display past juggle session logs.

Without arguments, lists all recorded sessions (newest first).
With a run-id argument, shows a detailed timeline of that session.`,
	Example: `  # List all sessions
  juggle log

  # Show timeline for a specific session
  juggle log abc12345-dead-beef

  # Show the most recent session
  juggle log --latest

  # Raw JSON output for piping
  juggle log --json
  juggle log abc12345 --json`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeLogRunIDs,
	SilenceUsage:      true,
	SilenceErrors:     true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLogCmd(cmd, args)
	},
}

func init() {
	logCmd.Flags().BoolVar(&logFlags.latest, "latest", false, "show the most recent session")
	logCmd.Flags().BoolVar(&logFlags.json, "json", false, "output raw JSONL")
	logCmd.SetHelpFunc(groupedHelp)
	rootCmd.AddCommand(logCmd)
}

// logOutputFormat resolves the output format for the log command.
// --json is treated as --format json when --format is not explicitly set.
func logOutputFormat() OutputFormat {
	if logFlags.json && flags.format == "" {
		return FormatJSON
	}
	return outputFormat()
}

func runLogCmd(cmd *cobra.Command, args []string) error {
	logPath := DefaultLogPath()
	if logPath == "" {
		return fmt.Errorf("cannot determine log directory")
	}

	runs, err := parseLogFile(logPath)
	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}

	ofmt := logOutputFormat()

	if len(runs) == 0 {
		if ofmt == FormatToon {
			fmt.Fprintln(os.Stdout, "runs[0]: none found")
			return nil
		}
		fmt.Fprintln(os.Stderr, "No sessions found.")
		return nil
	}

	ids := sortedRunIDs(runs)

	switch {
	case logFlags.latest && len(ids) > 0:
		return showRun(runs[ids[0]], ofmt, os.Stdout)
	case len(args) == 1:
		run, ok := runs[args[0]]
		if !ok {
			if ofmt == FormatToon {
				ToonError(os.Stdout, "not_found", fmt.Sprintf("run %q not found", args[0]))
			}
			return fmt.Errorf("run %q not found", args[0])
		}
		return showRun(run, ofmt, os.Stdout)
	default:
		return listRuns(runs, ids, ofmt, os.Stdout)
	}
}

func listRuns(runs map[string]*logRun, ids []string, ofmt OutputFormat, w io.Writer) error {
	switch ofmt {
	case FormatToon:
		fields := []string{"id", "date", "label", "iter", "status"}
		rows := make([][]string, 0, len(ids))
		for _, id := range ids {
			run := runs[id]
			rows = append(rows, runToListRow(id, run))
		}
		ToonList(w, "runs", fields, rows, 0)
		ToonHelp(w, []string{
			"juggle log <id> for details",
		})
		return nil
	case FormatJSON:
		for _, id := range ids {
			run := runs[id]
			for _, evt := range run.Events {
				fmt.Fprintln(w, string(evt))
			}
		}
		return nil
	default:
		color := isColorEnabled(w)
		for _, id := range ids {
			printRunListLine(w, id, runs[id], color)
		}
		return nil
	}
}

// runToListRow extracts a TOON row for a run summary.
func runToListRow(id string, run *logRun) []string {
	shortID := id
	if len(id) > 8 {
		shortID = id[:8]
	}

	var ts time.Time
	var label string
	if run.Start != nil {
		ts = run.Start.Timestamp
		label = run.Start.Label
	}

	dateStr := "???"
	if !ts.IsZero() {
		dateStr = ts.Format("2006-01-02 15:04")
	}

	if label == "" {
		label = truncate(run.startPrompt(), 40)
	}

	var iters int
	var statusStr string
	if run.End != nil {
		iters = run.End.Iterations
		statusStr = "done"
	} else {
		iters = run.countIterEnds()
		statusStr = "running"
	}

	return []string{shortID, dateStr, label, fmt.Sprintf("%d", iters), statusStr}
}

func printRunListLine(w io.Writer, id string, run *logRun, color bool) {
	shortID := id
	if len(id) > 8 {
		shortID = id[:8]
	}

	var ts time.Time
	var label string
	var provider string
	var model string
	if run.Start != nil {
		ts = run.Start.Timestamp
		label = run.Start.Label
		provider = run.Start.Provider
		model = run.Start.Model
	}

	dateStr := "???"
	if !ts.IsZero() {
		dateStr = ts.Format("2006-01-02 15:04")
	}

	if label == "" {
		label = truncate(run.startPrompt(), 40)
	}

	var statusStr string
	var iters int
	var cost float64
	var durMs int64
	if run.End != nil {
		iters = run.End.Iterations
		cost = run.End.EstimatedCost
		durMs = run.End.DurationMs
		statusStr = "✓"
		if color {
			statusStr = ansiGreen + "✓" + ansiReset
		}
	} else {
		iters = run.countIterEnds()
		statusStr = "…"
		if color {
			statusStr = ansiYellow + "…" + ansiReset
		}
	}

	durStr := formatDurationMs(durMs)
	costStr := fmt.Sprintf("$%.2f", cost)

	parts := []string{shortID, dateStr}
	if label != "" {
		parts = append(parts, fmt.Sprintf("%q", label))
	}
	parts = append(parts, fmt.Sprintf("%d iter", iters))
	if provider != "" {
		parts = append(parts, provider)
	}
	if model != "" {
		parts = append(parts, model)
	}
	parts = append(parts, costStr, durStr, statusStr)

	fmt.Fprintln(w, strings.Join(parts, "  "))
}

func showRun(run *logRun, ofmt OutputFormat, w io.Writer) error {
	switch ofmt {
	case FormatToon:
		return showRunToon(run, w)
	case FormatJSON:
		for _, evt := range run.Events {
			fmt.Fprintln(w, string(evt))
		}
		return nil
	default:
		return showRunText(run, w)
	}
}

func showRunToon(run *logRun, w io.Writer) error {
	// Run header
	runFields := []string{"run_id", "provider", "model", "status"}
	runValues := make([]string, 4)
	if run.Start != nil {
		runValues[0] = shortRunID(run.Start.RunID)
		runValues[1] = run.Start.Provider
		runValues[2] = run.Start.Model
		if run.Start.Label != "" {
			runFields = append(runFields, "label")
			runValues = append(runValues, run.Start.Label)
		}
		if run.Start.Prompt != "" {
			runFields = append(runFields, "prompt")
			runValues = append(runValues, truncate(run.Start.Prompt, 200))
		}
	}
	status := "running"
	if run.End != nil {
		status = "done"
	}
	runValues[3] = status
	ToonObject(w, runFields, runValues)

	// Iterations
	iterFields := []string{"iter", "duration", "tokens_in", "tokens_out", "exit"}
	var iterRows [][]string
	for _, raw := range run.Events {
		var typed rawLogEntry
		if json.Unmarshal(raw, &typed) != nil {
			continue
		}
		if typed.Type == "iter_end" || typed.Type == "" {
			var e iterationLogEntry
			if json.Unmarshal(raw, &e) != nil {
				continue
			}
			iterRows = append(iterRows, []string{
				fmt.Sprintf("%d", e.Iteration),
				formatDurationMs(e.DurationMs),
				fmt.Sprintf("%d", e.InputTokens),
				fmt.Sprintf("%d", e.OutputTokens),
				fmt.Sprintf("%d", e.ExitCode),
			})
		}
	}
	if len(iterRows) > 0 {
		ToonList(w, "iterations", iterFields, iterRows, 0)
	}

	// Summary
	if run.End != nil {
		e := run.End
		ToonObject(w,
			[]string{"iterations", "tokens_in", "tokens_out", "cost", "duration"},
			[]string{
				fmt.Sprintf("%d", e.Iterations),
				fmt.Sprintf("%d", e.InputTokens),
				fmt.Sprintf("%d", e.OutputTokens),
				fmt.Sprintf("$%.4f", e.EstimatedCost),
				formatDurationMs(e.DurationMs),
			},
		)
	}

	return nil
}

func showRunText(run *logRun, w io.Writer) error {
	color := isColorEnabled(w)

	if run.Start != nil {
		s := run.Start
		var label string
		if s.Label != "" {
			label = fmt.Sprintf("  %s", s.Label)
		}
		fmt.Fprintf(w, "%s  RUN START  %s  %s/%s%s\n",
			s.Timestamp.Format("15:04:05"),
			shortRunID(s.RunID),
			s.Provider,
			s.Model,
			label,
		)
		if s.Prompt != "" {
			prompt := truncate(s.Prompt, 80)
			fmt.Fprintf(w, "  prompt: %s\n", prompt)
		}
		if len(s.Watch) > 0 {
			fmt.Fprintf(w, "  watch: %s\n", strings.Join(s.Watch, ", "))
		}
		if s.Workers > 0 {
			fmt.Fprintf(w, "  workers: %d\n", s.Workers)
		}
	}

	for _, raw := range run.Events {
		var typed rawLogEntry
		if json.Unmarshal(raw, &typed) != nil {
			continue
		}

		switch typed.Type {
		case "iter_start":
			var e iterStartLogEntry
			if json.Unmarshal(raw, &e) != nil {
				continue
			}
			workerStr := ""
			if e.WorkerID > 0 {
				workerStr = fmt.Sprintf("  worker=%d", e.WorkerID)
			}
			taskStr := ""
			if e.TaskFile != "" {
				taskStr = fmt.Sprintf("  task=%s", filepath.Base(e.TaskFile))
			}
			fmt.Fprintf(w, "%s  ITER %-4d  started%s%s\n",
				e.Timestamp.Format("15:04:05"),
				e.Iteration,
				workerStr,
				taskStr,
			)

		case "iter_end", "":
			var e iterationLogEntry
			if json.Unmarshal(raw, &e) != nil {
				continue
			}
			durStr := formatDurationMs(e.DurationMs)
			tokStr := formatTokens(e.InputTokens, e.OutputTokens, e.CacheTokens)
			workerStr := ""
			if e.WorkerID > 0 {
				workerStr = fmt.Sprintf("  worker=%d", e.WorkerID)
			}

			if e.ExitCode == 0 {
				check := "✓"
				if color {
					check = ansiGreen + "✓" + ansiReset
				}
				fmt.Fprintf(w, "%s  ITER %-4d  %s  %s  %s%s\n",
					e.Timestamp.Format("15:04:05"),
					e.Iteration,
					check,
					durStr,
					tokStr,
					workerStr,
				)
			} else {
				check := "✗"
				errStr := ""
				if color {
					check = ansiRed + "✗" + ansiReset
				}
				if e.Error != nil && *e.Error != "" {
					errStr = fmt.Sprintf("  %s", truncate(*e.Error, 60))
				}
				fmt.Fprintf(w, "%s  ITER %-4d  %s  %s  exit %d%s%s\n",
					e.Timestamp.Format("15:04:05"),
					e.Iteration,
					check,
					durStr,
					e.ExitCode,
					errStr,
					workerStr,
				)
			}

		case "run_start", "run_end":
			// handled above/below
		}
	}

	if run.End != nil {
		e := run.End
		durStr := formatDurationMs(e.DurationMs)
		tokStr := formatTokens(e.InputTokens, e.OutputTokens, e.CacheTokens)
		costStr := fmt.Sprintf("~$%.4f", e.EstimatedCost)
		fmt.Fprintf(w, "%s  RUN END    %d iters  %s  %s  %s\n",
			e.Timestamp.Format("15:04:05"),
			e.Iterations,
			tokStr,
			costStr,
			durStr,
		)
	}

	return nil
}

func shortRunID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

func formatDurationMs(ms int64) string {
	if ms <= 0 {
		return "0s"
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}

func formatTokens(in, out, cache int) string {
	s := fmt.Sprintf("%s in / %s out", humanizeNum(in), humanizeNum(out))
	if cache > 0 {
		s += fmt.Sprintf(" (%s cached)", humanizeNum(cache))
	}
	return s
}

func humanizeNum(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func (r *logRun) startPrompt() string {
	if r.Start != nil && r.Start.Prompt != "" {
		return r.Start.Prompt
	}
	return ""
}

func (r *logRun) countIterEnds() int {
	n := 0
	for _, raw := range r.Events {
		var typed rawLogEntry
		if json.Unmarshal(raw, &typed) != nil {
			continue
		}
		if typed.Type == "iter_end" || typed.Type == "" {
			n++
		}
	}
	return n
}
