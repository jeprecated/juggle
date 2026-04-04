package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
)

func TestBuildJuggleEnv_BasicVars(t *testing.T) {
	env := buildJuggleEnv("run-id-123", 2, 5, "", "sonnet", "claude", "", -1)
	for _, want := range []string{
		"JUGGLE_ITERATION=2",
		"JUGGLE_MAX_ITERATIONS=5",
		"JUGGLE_RUN_ID=run-id-123",
		"JUGGLE_MODEL=sonnet",
		"JUGGLE_PROVIDER=claude",
	} {
		if !hasEnv(env, want) {
			t.Errorf("env missing %q; have: %v", want, env)
		}
	}
}

func TestBuildJuggleEnv_LabelSetWhenNonEmpty(t *testing.T) {
	env := buildJuggleEnv("id", 1, 3, "my-run", "sonnet", "claude", "", -1)
	if !hasEnv(env, "JUGGLE_LABEL=my-run") {
		t.Errorf("expected JUGGLE_LABEL=my-run; have: %v", env)
	}
}

func TestBuildJuggleEnv_LabelOmittedWhenEmpty(t *testing.T) {
	env := buildJuggleEnv("id", 1, 3, "", "sonnet", "claude", "", -1)
	for _, e := range env {
		if strings.HasPrefix(e, "JUGGLE_LABEL=") {
			t.Errorf("expected JUGGLE_LABEL omitted when empty; have: %v", env)
		}
	}
}

func TestBuildJuggleEnv_TaskFileSetWhenNonEmpty(t *testing.T) {
	env := buildJuggleEnv("id", 1, 5, "", "sonnet", "claude", "/tasks/foo.md", -1)
	if !hasEnv(env, "JUGGLE_TASK_FILE=/tasks/foo.md") {
		t.Errorf("expected JUGGLE_TASK_FILE=/tasks/foo.md; have: %v", env)
	}
}

func TestBuildJuggleEnv_TaskFileOmittedWhenEmpty(t *testing.T) {
	env := buildJuggleEnv("id", 1, 5, "", "sonnet", "claude", "", -1)
	for _, e := range env {
		if strings.HasPrefix(e, "JUGGLE_TASK_FILE=") {
			t.Errorf("expected JUGGLE_TASK_FILE omitted when empty; have: %v", env)
		}
	}
}

func TestBuildJuggleEnv_WorkerIDSetWhenNonNegative(t *testing.T) {
	env := buildJuggleEnv("id", 1, 5, "", "sonnet", "claude", "", 2)
	if !hasEnv(env, "JUGGLE_WORKER_ID=2") {
		t.Errorf("expected JUGGLE_WORKER_ID=2; have: %v", env)
	}
}

func TestBuildJuggleEnv_WorkerIDZeroIsSet(t *testing.T) {
	env := buildJuggleEnv("id", 1, 5, "", "sonnet", "claude", "", 0)
	if !hasEnv(env, "JUGGLE_WORKER_ID=0") {
		t.Errorf("expected JUGGLE_WORKER_ID=0; have: %v", env)
	}
}

func TestBuildJuggleEnv_WorkerIDOmittedWhenNegative(t *testing.T) {
	env := buildJuggleEnv("id", 1, 5, "", "sonnet", "claude", "", -1)
	for _, e := range env {
		if strings.HasPrefix(e, "JUGGLE_WORKER_ID=") {
			t.Errorf("expected JUGGLE_WORKER_ID omitted for non-worker runs; have: %v", env)
		}
	}
}

func TestBuildJuggleEnv_UnlimitedIterationsIsZero(t *testing.T) {
	env := buildJuggleEnv("id", 1, 0, "", "sonnet", "claude", "", -1)
	if !hasEnv(env, "JUGGLE_MAX_ITERATIONS=0") {
		t.Errorf("expected JUGGLE_MAX_ITERATIONS=0 for unlimited; have: %v", env)
	}
}

func TestGenerateRunID_IsNonEmpty(t *testing.T) {
	id := generateRunID()
	if id == "" {
		t.Error("expected non-empty run ID")
	}
}

func TestGenerateRunID_IsUnique(t *testing.T) {
	a := generateRunID()
	b := generateRunID()
	if a == b {
		t.Errorf("expected unique run IDs, got %q twice", a)
	}
}

func TestGenerateRunID_LooksLikeUUID(t *testing.T) {
	id := generateRunID()
	// UUID format: 8-4-4-4-12 hex chars separated by dashes = 36 chars total
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d: %q", len(id), id)
	}
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Errorf("expected UUID dashes at positions 8,13,18,23: %q", id)
	}
}

func TestRunLoop_SetsJuggleEnvVars(t *testing.T) {
	mock := agent.NewMockRunner(
		&agent.RunResult{Output: "ok"},
		&agent.RunResult{Output: "ok"},
	)
	cfg := Config{
		Content:    "test",
		Iterations: 2,
		Model:      "sonnet",
		Provider:   "claude",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatal(err)
	}
	if len(mock.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(mock.Calls))
	}

	// Iteration numbers increment
	if !hasEnv(mock.Calls[0].Env, "JUGGLE_ITERATION=1") {
		t.Errorf("call 0: missing JUGGLE_ITERATION=1; have: %v", mock.Calls[0].Env)
	}
	if !hasEnv(mock.Calls[1].Env, "JUGGLE_ITERATION=2") {
		t.Errorf("call 1: missing JUGGLE_ITERATION=2; have: %v", mock.Calls[1].Env)
	}

	// Max iterations
	if !hasEnv(mock.Calls[0].Env, "JUGGLE_MAX_ITERATIONS=2") {
		t.Errorf("missing JUGGLE_MAX_ITERATIONS=2; have: %v", mock.Calls[0].Env)
	}

	// Model and provider
	if !hasEnv(mock.Calls[0].Env, "JUGGLE_MODEL=sonnet") {
		t.Errorf("missing JUGGLE_MODEL=sonnet; have: %v", mock.Calls[0].Env)
	}
	if !hasEnv(mock.Calls[0].Env, "JUGGLE_PROVIDER=claude") {
		t.Errorf("missing JUGGLE_PROVIDER=claude; have: %v", mock.Calls[0].Env)
	}

	// RUN_ID is non-empty and stable across iterations
	id0 := envValue(mock.Calls[0].Env, "JUGGLE_RUN_ID")
	id1 := envValue(mock.Calls[1].Env, "JUGGLE_RUN_ID")
	if id0 == "" {
		t.Error("JUGGLE_RUN_ID not set on first call")
	}
	if id0 != id1 {
		t.Errorf("JUGGLE_RUN_ID not stable across iterations: %q vs %q", id0, id1)
	}
}

func TestRunLoop_SetsLabelEnvVar(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:    "test",
		Iterations: 1,
		Model:      "sonnet",
		Provider:   "claude",
		Label:      "my-label",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	if err := RunLoop(cfg); err != nil {
		t.Fatal(err)
	}
	if !hasEnv(mock.Calls[0].Env, "JUGGLE_LABEL=my-label") {
		t.Errorf("missing JUGGLE_LABEL=my-label; have: %v", mock.Calls[0].Env)
	}
}

func TestRunWatchTask_SetsTaskFileEnvVar(t *testing.T) {
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "task.md")
	if err := os.WriteFile(taskFile, []byte("do work"), 0644); err != nil {
		t.Fatal(err)
	}

	mock := agent.NewMockRunner(&agent.RunResult{Output: "ok"})
	cfg := Config{
		Content:    "rules",
		Iterations: 1,
		Model:      "sonnet",
		Provider:   "claude",
		Runner:     mock,
		Stderr:     &bytes.Buffer{},
	}
	stats := &runStats{}
	if err := runWatchTask(cfg, taskFile, "task.md", stats); err != nil {
		t.Fatal(err)
	}
	if !hasEnv(mock.Calls[0].Env, "JUGGLE_TASK_FILE="+taskFile) {
		t.Errorf("missing JUGGLE_TASK_FILE; have: %v", mock.Calls[0].Env)
	}
}

// hasEnv returns true if the exact string s appears in env.
func hasEnv(env []string, s string) bool {
	for _, e := range env {
		if e == s {
			return true
		}
	}
	return false
}

// envValue returns the value for key in the env slice ("KEY=value" → "value"), or "".
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}
