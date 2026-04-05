package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

// iterationLogEntry is the per-iteration JSONL record written to the log file.
type iterationLogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Iteration    int       `json:"iteration"`
	Label        string    `json:"label,omitempty"`
	DurationMs   int64     `json:"duration_ms"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CacheTokens  int       `json:"cache_tokens,omitempty"`
	ExitCode     int       `json:"exit_code"`
	RateLimited  bool      `json:"rate_limited"`
	Error        *string   `json:"error"`
}

// summaryLogEntry is the final JSONL record appended after all iterations complete.
type summaryLogEntry struct {
	Type          string    `json:"type"`
	Timestamp     time.Time `json:"timestamp"`
	Iterations    int       `json:"iterations"`
	InputTokens   int       `json:"input_tokens"`
	OutputTokens  int       `json:"output_tokens"`
	CacheTokens   int       `json:"cache_tokens,omitempty"`
	DurationMs    int64     `json:"duration_ms"`
	EstimatedCost float64   `json:"estimated_cost"`
}

// writeIterationLog appends a JSONL entry for a completed iteration to the log file.
// Errors are silently ignored (best-effort logging).
func writeIterationLog(logFile string, entry iterationLogEntry) {
	if logFile == "" {
		return
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(entry)
}

// writeSummaryLog appends a JSON summary entry to the log file.
// Errors are silently ignored (best-effort logging).
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
		Type:          "summary",
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

// parseLastIteration reads a JSONL log file and returns the highest iteration
// number found. Returns 0 if the file doesn't exist, is empty, or has no
// valid iteration entries.
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
		if entry.Iteration > max {
			max = entry.Iteration
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	return max, nil
}
