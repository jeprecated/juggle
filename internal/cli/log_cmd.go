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

func runLogCmd(cmd *cobra.Command, args []string) error {
	logPath := DefaultLogPath()
	if logPath == "" {
		return fmt.Errorf("cannot determine log directory")
	}

	runs, err := parseLogFile(logPath)
	if err != nil {
		return fmt.Errorf("reading log: %w", err)
	}
	if len(runs) == 0 {
		fmt.Fprintln(os.Stderr, "No sessions found.")
		return nil
	}

	ids := sortedRunIDs(runs)

	switch {
	case logFlags.latest && len(ids) > 0:
		return showRun(runs[ids[0]], logFlags.json, os.Stdout)
	case len(args) == 1:
		run, ok := runs[args[0]]
		if !ok {
			return fmt.Errorf("run %q not found", args[0])
		}
		return showRun(run, logFlags.json, os.Stdout)
	default:
		return listRuns(runs, ids, logFlags.json, os.Stdout)
	}
}

func listRuns(runs map[string]*logRun, ids []string, jsonOut bool, w io.Writer) error {
	color := isColorEnabled(w)
	for _, id := range ids {
		run := runs[id]
		if jsonOut {
			for _, evt := range run.Events {
				fmt.Fprintln(w, string(evt))
			}
			continue
		}
		printRunListLine(w, id, run, color)
	}
	return nil
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

func showRun(run *logRun, jsonOut bool, w io.Writer) error {
	color := isColorEnabled(w)

	if jsonOut {
		for _, evt := range run.Events {
			fmt.Fprintln(w, string(evt))
		}
		return nil
	}

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
