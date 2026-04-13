package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type iterationLogEntry struct {
	Type         string    `json:"type"`
	RunID        string    `json:"run_id"`
	Timestamp    time.Time `json:"timestamp"`
	Iteration    int       `json:"iteration"`
	Label        string    `json:"label,omitempty"`
	WorkerID     int       `json:"worker_id,omitempty"`
	DurationMs   int64     `json:"duration_ms"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CacheTokens  int       `json:"cache_tokens,omitempty"`
	ExitCode     int       `json:"exit_code"`
	RateLimited  bool      `json:"rate_limited"`
	Error        *string   `json:"error"`
}

type runStartLogEntry struct {
	Type      string    `json:"type"`
	RunID     string    `json:"run_id"`
	Timestamp time.Time `json:"timestamp"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	Label     string    `json:"label,omitempty"`
	Prompt    string    `json:"prompt,omitempty"`
	Workers   int       `json:"workers,omitempty"`
	Watch     []string  `json:"watch,omitempty"`
}

type iterStartLogEntry struct {
	Type      string    `json:"type"`
	RunID     string    `json:"run_id"`
	Timestamp time.Time `json:"timestamp"`
	Iteration int       `json:"iteration"`
	WorkerID  int       `json:"worker_id,omitempty"`
	TaskFile  string    `json:"task_file,omitempty"`
}

type summaryLogEntry struct {
	Type          string    `json:"type"`
	RunID         string    `json:"run_id"`
	Timestamp     time.Time `json:"timestamp"`
	Iterations    int       `json:"iterations"`
	InputTokens   int       `json:"input_tokens"`
	OutputTokens  int       `json:"output_tokens"`
	CacheTokens   int       `json:"cache_tokens,omitempty"`
	DurationMs    int64     `json:"duration_ms"`
	EstimatedCost float64   `json:"estimated_cost"`
}

type rawLogEntry struct {
	Type      string    `json:"type"`
	RunID     string    `json:"run_id"`
	Timestamp time.Time `json:"timestamp"`
}

func writeIterationLog(logFile string, entry iterationLogEntry) {
	if logFile == "" {
		return
	}
	if entry.Type == "" {
		entry.Type = "iter_end"
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(entry)
}

func writeRunStartLog(logFile string, entry runStartLogEntry) {
	if logFile == "" {
		return
	}
	entry.Type = "run_start"
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(entry)
}

func writeIterStartLog(logFile string, entry iterStartLogEntry) {
	if logFile == "" {
		return
	}
	entry.Type = "iter_start"
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(entry)
}

func writeSummaryLog(logFile string, stats runStats) {
	if logFile == "" {
		return
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	entry := summaryLogEntry{
		Type:          "run_end",
		RunID:         stats.runID,
		Timestamp:     time.Now().UTC(),
		Iterations:    stats.iterations,
		InputTokens:   stats.inputTokens,
		OutputTokens:  stats.outputTokens,
		CacheTokens:   stats.cacheTokens,
		DurationMs:    time.Since(stats.start).Milliseconds(),
		EstimatedCost: estimateCost(stats.inputTokens, stats.outputTokens, stats.model),
	}
	enc := json.NewEncoder(f)
	_ = enc.Encode(entry)
}

func parseLastIteration(logFile string) (int, error) {
	f, err := os.Open(logFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	max := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		var entry iterationLogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type != "" && entry.Type != "iter_end" {
			continue
		}
		if entry.Iteration > max {
			max = entry.Iteration
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	return max, nil
}

// DefaultLogDir returns the default log directory path.
func DefaultLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "juggle")
}

// DefaultLogPath returns the default log file path.
func DefaultLogPath() string {
	dir := DefaultLogDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "log.jsonl")
}

// EnsureLogDir creates the log directory if it doesn't exist.
func EnsureLogDir(logPath string) error {
	dir := filepath.Dir(logPath)
	return os.MkdirAll(dir, 0755)
}

// logRun represents a parsed run with its start info and events.
type logRun struct {
	Start   *runStartLogEntry
	End     *summaryLogEntry
	Events  []json.RawMessage
}

// parseLogFile reads all entries from the log file and groups them by run_id.
func parseLogFile(path string) (map[string]*logRun, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	runs := make(map[string]*logRun)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw rawLogEntry
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		run, ok := runs[raw.RunID]
		if !ok {
			run = &logRun{}
			runs[raw.RunID] = run
		}
		run.Events = append(run.Events, line)

		switch raw.Type {
		case "run_start":
			var s runStartLogEntry
			if json.Unmarshal(line, &s) == nil {
				run.Start = &s
			}
		case "run_end":
			var s summaryLogEntry
			if json.Unmarshal(line, &s) == nil {
				run.End = &s
			}
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return runs, nil
}

// sortedRunIDs returns run IDs sorted by their start timestamp (newest first).
func sortedRunIDs(runs map[string]*logRun) []string {
	type idTime struct {
		id string
		t  time.Time
	}
	var items []idTime
	for id, run := range runs {
		t := time.Time{}
		if run.Start != nil {
			t = run.Start.Timestamp
		}
		items = append(items, idTime{id: id, t: t})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].t.After(items[j].t)
	})
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.id
	}
	return ids
}
