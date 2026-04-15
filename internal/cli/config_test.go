package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_NoFile(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer
	cfg, path, err := LoadConfig(false, dir, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config when no file found")
	}
	if path != "" {
		t.Errorf("expected empty path, got %q", path)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected no stderr output, got %q", stderr.String())
	}
}

func TestLoadConfig_LocalFile(t *testing.T) {
	dir := t.TempDir()
	tomlContent := "iterations = 5\nmodel = \"opus\"\n"
	err := os.WriteFile(filepath.Join(dir, "juggle.toml"), []byte(tomlContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	cfg, path, err := LoadConfig(false, dir, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Iterations == nil || *cfg.Iterations != 5 {
		t.Errorf("expected iterations=5, got %v", cfg.Iterations)
	}
	if cfg.Model == nil || *cfg.Model != "opus" {
		t.Errorf("expected model=opus, got %v", cfg.Model)
	}
	if !strings.Contains(path, "juggle.toml") {
		t.Errorf("expected path to contain juggle.toml, got %q", path)
	}
	out := stderr.String()
	if !strings.Contains(out, "using config:") {
		t.Errorf("expected 'using config:' in stderr, got %q", out)
	}
	if !strings.Contains(out, "juggle.toml") {
		t.Errorf("expected juggle.toml in stderr, got %q", out)
	}
}

func TestLoadConfig_PrintsRelativePath(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "juggle.toml"), []byte(""), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	_, _, err = LoadConfig(false, dir, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stderr.String()
	// Should print with ./ prefix for local file
	if !strings.Contains(out, "./juggle.toml") {
		t.Errorf("expected './juggle.toml' in stderr, got %q", out)
	}
}

func TestLoadConfig_NoConfig(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "juggle.toml"), []byte("iterations = 5\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	cfg, _, err := LoadConfig(true, dir, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config when noConfig=true")
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "juggle.toml"), []byte("not valid toml = [[["), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	_, _, err = LoadConfig(false, dir, &stderr)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestApplyFileConfig_OverridesUnchangedFlags(t *testing.T) {
	origIterations := flags.iterations
	origModel := flags.model
	t.Cleanup(func() {
		flags.iterations = origIterations
		flags.model = origModel
	})

	flags.iterations = 0
	flags.model = "sonnet"

	five := 5
	opus := "opus"
	cfg := &FileConfig{
		Iterations: &five,
		Model:      &opus,
	}

	changed := func(string) bool { return false }
	var stderr bytes.Buffer
	ApplyFileConfig(cfg, changed, false, &stderr, "loop")

	if flags.iterations != 5 {
		t.Errorf("expected iterations=5, got %d", flags.iterations)
	}
	if flags.model != "opus" {
		t.Errorf("expected model=opus, got %s", flags.model)
	}
}

func TestApplyFileConfig_DoesNotOverrideChangedFlags(t *testing.T) {
	origIterations := flags.iterations
	t.Cleanup(func() {
		flags.iterations = origIterations
	})

	flags.iterations = 7

	five := 5
	cfg := &FileConfig{Iterations: &five}

	changed := func(name string) bool { return name == "iterations" }
	var stderr bytes.Buffer
	ApplyFileConfig(cfg, changed, false, &stderr, "loop")

	if flags.iterations != 7 {
		t.Errorf("expected iterations=7 (flag value preserved), got %d", flags.iterations)
	}
}

func TestApplyFileConfig_VerboseOutput(t *testing.T) {
	origModel := flags.model
	t.Cleanup(func() {
		flags.model = origModel
	})

	flags.model = "sonnet"

	opus := "opus"
	cfg := &FileConfig{Model: &opus}

	changed := func(string) bool { return false }
	var stderr bytes.Buffer
	ApplyFileConfig(cfg, changed, true, &stderr, "loop")

	out := stderr.String()
	if !strings.Contains(out, "model") {
		t.Errorf("expected 'model' in verbose output, got %q", out)
	}
}

func TestApplyFileConfig_NilConfig(t *testing.T) {
	var stderr bytes.Buffer
	ApplyFileConfig(nil, func(string) bool { return false }, false, &stderr, "loop")
}

func TestApplyFileConfig_LoopOnlyFields(t *testing.T) {
	origIterations := flags.iterations
	origDelay := flags.delay
	origTrust := flags.trust
	t.Cleanup(func() {
		flags.iterations = origIterations
		flags.delay = origDelay
		flags.trust = origTrust
	})

	n := 3
	d := 5
	tr := true
	cfg := &FileConfig{
		Iterations: &n,
		Delay:      &d,
		Trust:      &tr,
	}

	changed := func(string) bool { return false }
	var stderr bytes.Buffer
	ApplyFileConfig(cfg, changed, false, &stderr, "loop")

	if flags.iterations != 3 {
		t.Errorf("iterations: expected 3, got %d", flags.iterations)
	}
	if flags.delay != 5 {
		t.Errorf("delay: expected 5, got %d", flags.delay)
	}
	if !flags.trust {
		t.Error("trust: expected true")
	}
}

func TestApplyFileConfig_QueueOnlyFields(t *testing.T) {
	origWorkers := queueFlags.workers
	origOnTouch := queueFlags.onTouch
	origDashboard := queueFlags.dashboard
	t.Cleanup(func() {
		queueFlags.workers = origWorkers
		queueFlags.onTouch = origOnTouch
		queueFlags.dashboard = origDashboard
	})

	w := 4
	ot := true
	db := true
	cfg := &FileConfig{
		Workers:   &w,
		OnTouch:   &ot,
		Dashboard: &db,
	}

	changed := func(string) bool { return false }
	var stderr bytes.Buffer
	ApplyFileConfig(cfg, changed, false, &stderr, "queue")

	if queueFlags.workers != 4 {
		t.Errorf("workers: expected 4, got %d", queueFlags.workers)
	}
	if !queueFlags.onTouch {
		t.Error("on-touch: expected true")
	}
	if !queueFlags.dashboard {
		t.Error("dashboard: expected true")
	}
}

func TestApplyFileConfig_LoopIgnoresQueueKeys(t *testing.T) {
	origWorkers := queueFlags.workers
	t.Cleanup(func() {
		queueFlags.workers = origWorkers
	})

	queueFlags.workers = 1
	w := 8
	cfg := &FileConfig{Workers: &w}

	changed := func(string) bool { return false }
	var stderr bytes.Buffer
	ApplyFileConfig(cfg, changed, false, &stderr, "loop")

	if queueFlags.workers != 1 {
		t.Errorf("workers should not change in loop mode: expected 1, got %d", queueFlags.workers)
	}
}

func TestApplyFileConfig_QueueIgnoresLoopKeys(t *testing.T) {
	origIterations := flags.iterations
	origDelay := flags.delay
	t.Cleanup(func() {
		flags.iterations = origIterations
		flags.delay = origDelay
	})

	flags.iterations = 0
	flags.delay = 0
	n := 10
	d := 30
	cfg := &FileConfig{Iterations: &n, Delay: &d}

	changed := func(string) bool { return false }
	var stderr bytes.Buffer
	ApplyFileConfig(cfg, changed, false, &stderr, "queue")

	if flags.iterations != 0 {
		t.Errorf("iterations should not change in queue mode: expected 0, got %d", flags.iterations)
	}
	if flags.delay != 0 {
		t.Errorf("delay should not change in queue mode: expected 0, got %d", flags.delay)
	}
}

func TestLoadConfig_WatchAsString(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "juggle.toml"), []byte(`watch = "./tasks"`), 0644)

	var stderr bytes.Buffer
	cfg, _, err := LoadConfig(false, dir, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || cfg.Watch == nil {
		t.Fatal("expected non-nil config with Watch set")
	}
	if len(*cfg.Watch) != 1 || (*cfg.Watch)[0] != "./tasks" {
		t.Errorf("expected Watch=[./tasks], got %v", *cfg.Watch)
	}
}

func TestLoadConfig_WatchAsList(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "juggle.toml"), []byte("watch = [\"./tasks1\", \"./tasks2\"]"), 0644)

	var stderr bytes.Buffer
	cfg, _, err := LoadConfig(false, dir, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || cfg.Watch == nil {
		t.Fatal("expected non-nil config with Watch set")
	}
	if len(*cfg.Watch) != 2 {
		t.Errorf("expected 2 watch dirs, got %d: %v", len(*cfg.Watch), *cfg.Watch)
	}
	if (*cfg.Watch)[0] != "./tasks1" || (*cfg.Watch)[1] != "./tasks2" {
		t.Errorf("unexpected watch values: %v", *cfg.Watch)
	}
}

func TestApplyFileConfig_WatchSetsFlags(t *testing.T) {
	origWatch := queueFlags.watch
	t.Cleanup(func() { queueFlags.watch = origWatch })

	queueFlags.watch = nil
	w := tomlStringOrList([]string{"./tasks1", "./tasks2"})
	cfg := &FileConfig{Watch: &w}

	changed := func(string) bool { return false }
	var stderr bytes.Buffer
	ApplyFileConfig(cfg, changed, false, &stderr, "queue")

	if len(queueFlags.watch) != 2 {
		t.Fatalf("expected 2 watch dirs, got %d: %v", len(queueFlags.watch), queueFlags.watch)
	}
	if queueFlags.watch[0] != "./tasks1" || queueFlags.watch[1] != "./tasks2" {
		t.Errorf("unexpected watch flags: %v", queueFlags.watch)
	}
}
