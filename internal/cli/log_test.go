package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ohare93/juggle/internal/agent"
)

func TestParseLastIteration_NonexistentFile(t *testing.T) {
	got, err := parseLastIteration("/nonexistent/path/to/log.jsonl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0 for missing file, got %d", got)
	}
}

func TestParseLastIteration_EmptyFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "log*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := parseLastIteration(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Errorf("expected 0 for empty file, got %d", got)
	}
}

func TestParseLastIteration_SingleEntry(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")
	if err := os.WriteFile(logFile, []byte(`{"iteration":5}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := parseLastIteration(logFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 5 {
		t.Errorf("expected 5, got %d", got)
	}
}

func TestParseLastIteration_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")
	content := `{"iteration":1}` + "\n" + `{"iteration":3}` + "\n" + `{"iteration":2}` + "\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := parseLastIteration(logFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 3 {
		t.Errorf("expected 3 (max), got %d", got)
	}
}

func TestParseLastIteration_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")
	content := `{"iteration":2}` + "\n" + `not valid json` + "\n" + `{"iteration":4}` + "\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := parseLastIteration(logFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 4 {
		t.Errorf("expected 4, got %d", got)
	}
}

func TestParseLastIteration_SkipsNonIterationLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")
	// Summary lines without "iteration" field (like the existing run summary)
	content := `{"iteration":2}` + "\n" + `{"type":"summary","iterations":2}` + "\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := parseLastIteration(logFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

// --- writeIterationLog tests ---

func TestWriteIterationLog_JSONFields(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	errStr := "agent exited with error"
	entry := iterationLogEntry{
		RunID:        "test-run-id-123",
		Timestamp:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Iteration:    2,
		Label:        "fix tests",
		DurationMs:   1500,
		InputTokens:  100,
		OutputTokens: 50,
		CacheTokens:  20,
		ExitCode:     1,
		RateLimited:  false,
		Error:        &errStr,
	}
	writeIterationLog(logFile, entry)

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)

	checks := []string{
		`"run_id":"test-run-id-123"`,
		`"timestamp"`,
		`"iteration":2`,
		`"label":"fix tests"`,
		`"duration_ms":1500`,
		`"input_tokens":100`,
		`"output_tokens":50`,
		`"cache_tokens":20`,
		`"exit_code":1`,
		`"rate_limited":false`,
		`"error":"agent exited with error"`,
	}
	for _, check := range checks {
		if !strings.Contains(line, check) {
			t.Errorf("expected %q in log line, got: %s", check, line)
		}
	}
}

func TestWriteIterationLog_IncludesRunID(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	writeIterationLog(logFile, iterationLogEntry{
		RunID:     "abc-123",
		Iteration: 1,
	})

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"run_id":"abc-123"`) {
		t.Errorf("expected run_id in log entry, got: %s", string(data))
	}
}

func TestWriteIterationLog_NullError(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	writeIterationLog(logFile, iterationLogEntry{
		Iteration: 1,
		Error:     nil,
	})

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"error":null`) {
		t.Errorf("expected null error field, got: %s", string(data))
	}
}

func TestWriteIterationLog_AppendsBehavior(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	writeIterationLog(logFile, iterationLogEntry{Iteration: 1})
	writeIterationLog(logFile, iterationLogEntry{Iteration: 2})

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %s", len(lines), string(data))
	}
}

func TestWriteIterationLog_IncludesLabel(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	writeIterationLog(logFile, iterationLogEntry{Iteration: 1, Label: "refactor auth"})

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"label":"refactor auth"`) {
		t.Errorf("expected label in log entry, got: %s", string(data))
	}
}

func TestWriteIterationLog_OmitsLabelWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	writeIterationLog(logFile, iterationLogEntry{Iteration: 1, Label: ""})

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "label") {
		t.Errorf("expected no label field when empty, got: %s", string(data))
	}
}

// --- writeSummaryLog tests ---

func TestWriteSummaryLog_JSONFields(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	stats := runStats{
		runID:        "summary-run-id",
		iterations:   3,
		inputTokens:  300,
		outputTokens: 150,
		cacheTokens:  50,
		start:        time.Now().Add(-5 * time.Second),
		model:        "sonnet",
	}
	writeSummaryLog(logFile, stats)

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(bytes.TrimRight(data, "\n"), &entry); err != nil {
		t.Fatalf("failed to parse summary JSON: %v\nraw: %s", err, data)
	}

	if entry["type"] != "summary" {
		t.Errorf("expected type=summary, got: %v", entry["type"])
	}
	if v, ok := entry["iterations"].(float64); !ok || int(v) != 3 {
		t.Errorf("expected iterations=3, got: %v", entry["iterations"])
	}
	if v, ok := entry["input_tokens"].(float64); !ok || int(v) != 300 {
		t.Errorf("expected input_tokens=300, got: %v", entry["input_tokens"])
	}
	if _, ok := entry["timestamp"]; !ok {
		t.Error("expected timestamp field")
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Error("expected duration_ms field")
	}
	if _, ok := entry["estimated_cost"]; !ok {
		t.Error("expected estimated_cost field")
	}
	if v, ok := entry["run_id"].(string); !ok || v != "summary-run-id" {
		t.Errorf("expected run_id=summary-run-id, got: %v", entry["run_id"])
	}
}

func TestWriteSummaryLog_IncludesRunID(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	stats := runStats{
		runID: "xyz-456",
		start: time.Now(),
		model: "sonnet",
	}
	writeSummaryLog(logFile, stats)

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"run_id":"xyz-456"`) {
		t.Errorf("expected run_id in summary entry, got: %s", string(data))
	}
}

// --- Resume integration tests ---

func TestRunLoop_Resume_RequiresLog(t *testing.T) {
	cfg := Config{
		Content:    "do work",
		Iterations: 3,
		Resume:     true,
		// Log is intentionally not set
		Runner: agent.NewMockRunner(&agent.RunResult{}),
		Stderr: &bytes.Buffer{},
	}
	err := RunLoop(cfg)
	if err == nil {
		t.Fatal("expected error when --resume used without --log")
	}
	if !strings.Contains(err.Error(), "--log") {
		t.Errorf("error should mention --log, got: %v", err)
	}
}

func TestRunLoop_Resume_LogNotExist_StartsFromOne(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "nonexistent.jsonl")

	var capturedIterations []int
	runner := &funcRunner{run: func(opts agent.RunOptions) (*agent.RunResult, error) {
		// Extract iteration from prompt
		for i := 1; i <= 5; i++ {
			if strings.Contains(opts.Prompt, "iteration "+string(rune('0'+i))) {
				capturedIterations = append(capturedIterations, i)
			}
		}
		return &agent.RunResult{}, nil
	}}

	mock := agent.NewMockRunner(
		&agent.RunResult{},
		&agent.RunResult{},
	)
	cfg := Config{
		Content:    "do work",
		Iterations: 2,
		Resume:     true,
		Log:        logFile,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = runner
	if len(mock.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(mock.Calls))
	}
	// Should start from iteration 1 (no log exists)
	if !strings.Contains(mock.Calls[0].Prompt, "iteration 1 of 2") {
		t.Errorf("expected first call to be iteration 1, got prompt: %s", mock.Calls[0].Prompt)
	}
}

func TestRunLoop_Resume_StartsFromLastPlusOne(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")
	// Simulate that iterations 1 and 2 already completed
	content := `{"iteration":1}` + "\n" + `{"iteration":2}` + "\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	mock := agent.NewMockRunner(
		&agent.RunResult{},
		&agent.RunResult{},
		&agent.RunResult{},
	)
	cfg := Config{
		Content:    "do work",
		Iterations: 4, // would run 4 total; resume from 3
		Resume:     true,
		Log:        logFile,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have run iterations 3 and 4 only (2 calls)
	if len(mock.Calls) != 2 {
		t.Fatalf("expected 2 calls (iterations 3 and 4), got %d", len(mock.Calls))
	}
	if !strings.Contains(mock.Calls[0].Prompt, "iteration 3 of 4") {
		t.Errorf("expected first resumed call to be iteration 3 of 4, got: %s", mock.Calls[0].Prompt)
	}
	if !strings.Contains(mock.Calls[1].Prompt, "iteration 4 of 4") {
		t.Errorf("expected second resumed call to be iteration 4 of 4, got: %s", mock.Calls[1].Prompt)
	}
}

func TestRunLoop_Resume_LogsResumeMessage(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")
	if err := os.WriteFile(logFile, []byte(`{"iteration":2}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	mock := agent.NewMockRunner(&agent.RunResult{}, &agent.RunResult{})
	cfg := Config{
		Content:    "do work",
		Iterations: 4,
		Resume:     true,
		Log:        logFile,
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := stderr.String()
	if !strings.Contains(output, "resuming from iteration 3") {
		t.Errorf("expected 'resuming from iteration 3' in stderr, got: %s", output)
	}
}

func TestRunLoop_WritesIterationLogEntries(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	mock := agent.NewMockRunner(
		&agent.RunResult{},
		&agent.RunResult{},
		&agent.RunResult{},
	)
	cfg := Config{
		Content:    "do work",
		Iterations: 3,
		Log:        logFile,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After 3 iterations, log should have entries for iterations 1, 2, 3
	last, err := parseLastIteration(logFile)
	if err != nil {
		t.Fatalf("unexpected error reading log: %v", err)
	}
	if last != 3 {
		t.Errorf("expected last iteration 3 in log, got %d", last)
	}
}

func TestRunLoop_LogsTokensAndExitCode(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	mock := agent.NewMockRunner(&agent.RunResult{
		InputTokens:  100,
		OutputTokens: 50,
		CacheTokens:  20,
		ExitCode:     0,
	})
	cfg := Config{
		Content:    "do work",
		Iterations: 1,
		Log:        logFile,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	// First line is the iteration entry
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least one log line")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("failed to parse log line: %v", err)
	}

	requiredFields := []string{"run_id", "timestamp", "iteration", "duration_ms", "input_tokens", "output_tokens", "exit_code", "rate_limited", "error"}
	for _, f := range requiredFields {
		if _, ok := entry[f]; !ok {
			t.Errorf("missing field %q in log entry", f)
		}
	}

	if v, ok := entry["input_tokens"].(float64); !ok || int(v) != 100 {
		t.Errorf("expected input_tokens=100, got: %v", entry["input_tokens"])
	}
	if v, ok := entry["output_tokens"].(float64); !ok || int(v) != 50 {
		t.Errorf("expected output_tokens=50, got: %v", entry["output_tokens"])
	}
	if v, ok := entry["exit_code"].(float64); !ok || int(v) != 0 {
		t.Errorf("expected exit_code=0, got: %v", entry["exit_code"])
	}
	if v, ok := entry["error"]; !ok || v != nil {
		t.Errorf("expected error=null, got: %v", entry["error"])
	}
}

func TestRunLoop_LogsSummaryLine(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	mock := agent.NewMockRunner(
		&agent.RunResult{InputTokens: 100, OutputTokens: 50},
		&agent.RunResult{InputTokens: 200, OutputTokens: 80},
	)
	cfg := Config{
		Content:    "do work",
		Iterations: 2,
		Log:        logFile,
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	// Should have 2 iteration lines + 1 summary line
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (2 iterations + 1 summary), got %d:\n%s", len(lines), string(data))
	}

	var summary map[string]interface{}
	if err := json.Unmarshal([]byte(lines[2]), &summary); err != nil {
		t.Fatalf("failed to parse summary line: %v", err)
	}

	if summary["type"] != "summary" {
		t.Errorf("expected type=summary, got: %v", summary["type"])
	}
	if v, ok := summary["iterations"].(float64); !ok || int(v) != 2 {
		t.Errorf("expected iterations=2, got: %v", summary["iterations"])
	}
	if v, ok := summary["input_tokens"].(float64); !ok || int(v) != 300 {
		t.Errorf("expected input_tokens=300 (100+200), got: %v", summary["input_tokens"])
	}
}

func TestRunLoop_LabelInHeader(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{})
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "do work",
		Iterations: 1,
		Label:      "my label",
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "my label") {
		t.Errorf("expected label in header output, got: %s", stderr.String())
	}
}

func TestRunLoop_AutoLabelInHeader(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{})
	var stderr bytes.Buffer
	cfg := Config{
		Content:    "fix the failing tests and make sure everything passes",
		Iterations: 1,
		Runner:     mock,
		Stderr:     &stderr,
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatal(err)
	}
	// Auto-label is first ~50 chars of prompt
	if !strings.Contains(stderr.String(), "fix the failing tests") {
		t.Errorf("expected auto-label in header, got: %s", stderr.String())
	}
}
