package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseHookFlag_Simple verifies basic EVENT:CMD parsing.
func TestParseHookFlag_Simple(t *testing.T) {
	spec, err := parseHookFlag("Stop:echo done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Event != "Stop" {
		t.Errorf("expected event Stop, got %q", spec.Event)
	}
	if spec.Command != "echo done" {
		t.Errorf("expected command 'echo done', got %q", spec.Command)
	}
}

// TestParseHookFlag_ColonInCommand verifies split on first colon only.
func TestParseHookFlag_ColonInCommand(t *testing.T) {
	spec, err := parseHookFlag("PostToolUse:Bash:echo ran")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Event != "PostToolUse" {
		t.Errorf("expected event PostToolUse, got %q", spec.Event)
	}
	if spec.Command != "Bash:echo ran" {
		t.Errorf("expected command 'Bash:echo ran', got %q", spec.Command)
	}
}

// TestParseHookFlag_NoColon verifies error when no colon present.
func TestParseHookFlag_NoColon(t *testing.T) {
	_, err := parseHookFlag("NoColonHere")
	if err == nil {
		t.Fatal("expected error for missing colon")
	}
}

// TestParseHookFlag_FileRef verifies @file resolution in hook command.
func TestParseHookFlag_FileRef(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "remind.sh")
	os.WriteFile(script, []byte("echo remember to commit\n"), 0644)

	spec, err := parseHookFlag("Stop:@" + script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Event != "Stop" {
		t.Errorf("expected event Stop, got %q", spec.Event)
	}
	if !strings.Contains(spec.Command, "echo remember to commit") {
		t.Errorf("expected command to contain file contents, got %q", spec.Command)
	}
}

// TestParseHookFlag_FileRefMissing verifies error for missing @file.
func TestParseHookFlag_FileRefMissing(t *testing.T) {
	_, err := parseHookFlag("Stop:@/nonexistent/file.sh")
	if err == nil {
		t.Fatal("expected error for missing @file")
	}
}

// TestBuildHooksJSON_SingleHook verifies JSON generation for one hook.
func TestBuildHooksJSON_SingleHook(t *testing.T) {
	specs := []hookSpec{{Event: "Stop", Command: "echo done"}}
	data, err := buildHooksJSON(specs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'hooks' object in settings")
	}
	stopEntries, ok := hooks["Stop"].([]interface{})
	if !ok || len(stopEntries) != 1 {
		t.Fatalf("expected 1 Stop entry, got %v", hooks["Stop"])
	}

	entry := stopEntries[0].(map[string]interface{})
	hookCmds, ok := entry["hooks"].([]interface{})
	if !ok || len(hookCmds) != 1 {
		t.Fatal("expected 1 hook command in entry")
	}
	cmd := hookCmds[0].(map[string]interface{})
	if cmd["type"] != "command" {
		t.Errorf("expected type 'command', got %v", cmd["type"])
	}
	if cmd["command"] != "echo done" {
		t.Errorf("expected command 'echo done', got %v", cmd["command"])
	}
}

// TestBuildHooksJSON_MultipleHooksSameEvent verifies multiple hooks for same event.
func TestBuildHooksJSON_MultipleHooksSameEvent(t *testing.T) {
	specs := []hookSpec{
		{Event: "Stop", Command: "echo first"},
		{Event: "Stop", Command: "echo second"},
	}
	data, err := buildHooksJSON(specs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var settings map[string]interface{}
	json.Unmarshal(data, &settings)
	hooks := settings["hooks"].(map[string]interface{})
	stopEntries := hooks["Stop"].([]interface{})
	if len(stopEntries) != 2 {
		t.Errorf("expected 2 Stop entries, got %d", len(stopEntries))
	}
}

// TestBuildHooksJSON_MultipleEvents verifies hooks for different events.
func TestBuildHooksJSON_MultipleEvents(t *testing.T) {
	specs := []hookSpec{
		{Event: "Stop", Command: "echo stop"},
		{Event: "SessionStart", Command: "echo start"},
	}
	data, err := buildHooksJSON(specs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var settings map[string]interface{}
	json.Unmarshal(data, &settings)
	hooks := settings["hooks"].(map[string]interface{})
	if _, ok := hooks["Stop"]; !ok {
		t.Error("expected Stop key in hooks")
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Error("expected SessionStart key in hooks")
	}
}

// TestBuildHooksJSON_EmptySpecs verifies empty specs produces valid JSON.
func TestBuildHooksJSON_EmptySpecs(t *testing.T) {
	data, err := buildHooksJSON(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

// TestBuildHooksJSON_MergesWithBaseJSON verifies --hook specs are appended to --hooks-file content.
func TestBuildHooksJSON_MergesWithBaseJSON(t *testing.T) {
	base := []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"existing"}]}]}}`)
	specs := []hookSpec{{Event: "Stop", Command: "new cmd"}}
	data, err := buildHooksJSON(specs, base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var settings map[string]interface{}
	json.Unmarshal(data, &settings)
	hooks := settings["hooks"].(map[string]interface{})
	stopEntries := hooks["Stop"].([]interface{})
	if len(stopEntries) != 2 {
		t.Errorf("expected 2 Stop entries (1 existing + 1 new), got %d", len(stopEntries))
	}
}

// TestBuildHooksJSON_PreservesBaseJSONFields verifies non-hooks fields from base are preserved.
func TestBuildHooksJSON_PreservesBaseJSONFields(t *testing.T) {
	base := []byte(`{"someOtherField":"value","hooks":{}}`)
	specs := []hookSpec{{Event: "Stop", Command: "echo done"}}
	data, err := buildHooksJSON(specs, base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var settings map[string]interface{}
	json.Unmarshal(data, &settings)
	if settings["someOtherField"] != "value" {
		t.Error("expected non-hooks fields preserved from base JSON")
	}
}

// TestBuildHooksSettingsFile_NoHooks verifies empty string and no-op cleanup when no hooks configured.
func TestBuildHooksSettingsFile_NoHooks(t *testing.T) {
	path, cleanup, err := buildHooksSettingsFile(nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path when no hooks, got %q", path)
	}
	cleanup() // should not panic
}

// TestBuildHooksSettingsFile_WithHookFlag verifies temp file is created with hook content.
func TestBuildHooksSettingsFile_WithHookFlag(t *testing.T) {
	path, cleanup, err := buildHooksSettingsFile([]string{"Stop:echo done"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if path == "" {
		t.Fatal("expected non-empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("temp file contains invalid JSON: %v", err)
	}
	hooks := settings["hooks"].(map[string]interface{})
	if _, ok := hooks["Stop"]; !ok {
		t.Error("expected Stop in hooks")
	}
}

// TestBuildHooksSettingsFile_WithHooksFile verifies temp file is created from --hooks-file.
func TestBuildHooksSettingsFile_WithHooksFile(t *testing.T) {
	dir := t.TempDir()
	hooksFile := filepath.Join(dir, "hooks.json")
	os.WriteFile(hooksFile, []byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo from file"}]}]}}`), 0644)

	path, cleanup, err := buildHooksSettingsFile(nil, hooksFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if path == "" {
		t.Fatal("expected non-empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if !strings.Contains(string(data), "echo from file") {
		t.Errorf("expected hooks-file content in temp file, got %q", string(data))
	}
}

// TestBuildHooksSettingsFile_CleanupRemovesFile verifies cleanup deletes the file.
func TestBuildHooksSettingsFile_CleanupRemovesFile(t *testing.T) {
	path, cleanup, err := buildHooksSettingsFile([]string{"Stop:echo done"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected temp file to be removed after cleanup")
	}
}

// TestBuildHooksSettingsFile_MissingHooksFile verifies error for missing --hooks-file.
func TestBuildHooksSettingsFile_MissingHooksFile(t *testing.T) {
	_, cleanup, err := buildHooksSettingsFile(nil, "/nonexistent/hooks.json")
	cleanup()
	if err == nil {
		t.Fatal("expected error for missing hooks file")
	}
}

// TestBuildRunOptions_HooksSettingsFilePassedThrough verifies HooksSettingsFile flows into RunOptions.
func TestBuildRunOptions_HooksSettingsFilePassedThrough(t *testing.T) {
	cfg := Config{
		HooksSettingsFile: "/tmp/test-hooks.json",
	}
	opts := buildRunOptions(cfg, "test prompt")
	if opts.HooksSettingsFile != "/tmp/test-hooks.json" {
		t.Errorf("expected HooksSettingsFile to pass through, got %q", opts.HooksSettingsFile)
	}
}
