package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	content := `{"iteration":2}` + "\n" + `{"summary":"Run summary"}` + "\n"
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

func TestWriteIterationLog_IncludesLabel(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "run.jsonl")

	writeIterationLog(logFile, 1, "refactor auth")

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

	writeIterationLog(logFile, 1, "")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "label") {
		t.Errorf("expected no label field when empty, got: %s", string(data))
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
