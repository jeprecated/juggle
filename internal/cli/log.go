package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// iterationLogEntry is the per-iteration JSONL record written to the log file.
type iterationLogEntry struct {
	Iteration int `json:"iteration"`
}

// writeIterationLog appends a JSONL entry for a completed iteration to the log file.
// Errors are silently ignored (best-effort logging).
func writeIterationLog(logFile string, iteration int) {
	if logFile == "" {
		return
	}
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(iterationLogEntry{Iteration: iteration})
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
