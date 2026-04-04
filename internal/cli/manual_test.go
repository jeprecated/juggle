package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveContexts_InlineString(t *testing.T) {
	result, err := resolveContexts([]string{"do the thing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "do the thing" {
		t.Errorf("expected [\"do the thing\"], got %v", result)
	}
}

func TestResolveContexts_FileReference(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ctx.md")
	os.WriteFile(f, []byte("file contents here"), 0644)

	result, err := resolveContexts([]string{"@" + f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "file contents here" {
		t.Errorf("expected [\"file contents here\"], got %v", result)
	}
}

func TestResolveContexts_FileNotFound(t *testing.T) {
	_, err := resolveContexts([]string{"@/nonexistent/file.md"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestResolveContexts_Mixed(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ctx.md")
	os.WriteFile(f, []byte("from file"), 0644)

	result, err := resolveContexts([]string{"inline string", "@" + f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0] != "inline string" {
		t.Errorf("expected first = \"inline string\", got %q", result[0])
	}
	if result[1] != "from file" {
		t.Errorf("expected second = \"from file\", got %q", result[1])
	}
}

func TestResolveContexts_Empty(t *testing.T) {
	result, err := resolveContexts([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestManualSessionID_Deterministic(t *testing.T) {
	id1 := manualSessionID([]string{"context a", "context b"})
	id2 := manualSessionID([]string{"context a", "context b"})
	if id1 != id2 {
		t.Errorf("expected deterministic IDs, got %q and %q", id1, id2)
	}
}

func TestManualSessionID_DifferentInputs(t *testing.T) {
	id1 := manualSessionID([]string{"context a"})
	id2 := manualSessionID([]string{"context b"})
	if id1 == id2 {
		t.Error("expected different IDs for different inputs")
	}
}

func TestManualSessionID_Prefix(t *testing.T) {
	id := manualSessionID([]string{"anything"})
	if !strings.HasPrefix(id, "manual-") {
		t.Errorf("expected prefix \"manual-\", got %q", id)
	}
	// Should be "manual-" + 6 hex chars = 13 chars total
	if len(id) != 13 {
		t.Errorf("expected length 13, got %d (%q)", len(id), id)
	}
}

func TestManualSessionID_OrderIndependent(t *testing.T) {
	id1 := manualSessionID([]string{"a", "b"})
	id2 := manualSessionID([]string{"b", "a"})
	if id1 != id2 {
		t.Errorf("expected order-independent IDs, got %q and %q", id1, id2)
	}
}

func TestManualSessionID_EmptyContexts(t *testing.T) {
	id := manualSessionID([]string{})
	if !strings.HasPrefix(id, "manual-") {
		t.Errorf("expected prefix \"manual-\", got %q", id)
	}
}

func TestWatchSessionID(t *testing.T) {
	id := watchSessionID("/some/path/queue/ready")
	if !strings.HasPrefix(id, "watch-") {
		t.Errorf("expected prefix \"watch-\", got %q", id)
	}
	if len(id) != 12 {
		t.Errorf("expected length 12, got %d (%q)", len(id), id)
	}
}

func TestGenerateManualPrompt_Basic(t *testing.T) {
	prompt, err := generateManualPrompt([]string{"do the thing"}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "do the thing") {
		t.Error("expected prompt to contain context")
	}
	if !strings.Contains(prompt, "<context>") {
		t.Error("expected prompt to contain <context> tag")
	}
	if !strings.Contains(prompt, "iteration 1") {
		t.Error("expected prompt to contain iteration number")
	}
	if !strings.Contains(prompt, "<promise>") {
		t.Error("expected prompt to contain promise signal instructions")
	}
}

func TestGenerateManualPrompt_MultipleContexts(t *testing.T) {
	prompt, err := generateManualPrompt([]string{"objective", "instructions"}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "objective") {
		t.Error("expected prompt to contain first context")
	}
	if !strings.Contains(prompt, "instructions") {
		t.Error("expected prompt to contain second context")
	}
	if !strings.Contains(prompt, "iteration 3") {
		t.Error("expected prompt to contain iteration 3")
	}
}

func TestGenerateManualPrompt_NoContexts(t *testing.T) {
	prompt, err := generateManualPrompt([]string{}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "<instructions>") {
		t.Error("expected prompt to contain instructions even with no contexts")
	}
	if strings.Contains(prompt, "<context>") {
		t.Error("expected no <context> tags with empty contexts")
	}
}

func TestGenerateWatchPrompt_Basic(t *testing.T) {
	prompt, err := generateWatchPrompt("task file contents", []string{"worker instructions"}, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompt, "task file contents") {
		t.Error("expected prompt to contain task contents")
	}
	if !strings.Contains(prompt, "<task>") {
		t.Error("expected prompt to contain <task> tag")
	}
	if !strings.Contains(prompt, "worker instructions") {
		t.Error("expected prompt to contain context")
	}
	if !strings.Contains(prompt, "iteration 2") {
		t.Error("expected prompt to contain iteration 2")
	}
}
