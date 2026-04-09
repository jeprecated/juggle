package provider

import (
	"testing"
)

func TestGeminiProvider_Type(t *testing.T) {
	p := NewGeminiProvider()
	if p.Type() != TypeGemini {
		t.Errorf("GeminiProvider.Type() = %v, want %v", p.Type(), TypeGemini)
	}
}

func TestGeminiProvider_MapModel(t *testing.T) {
	p := NewGeminiProvider()

	tests := []struct {
		input string
		want  string
	}{
		{"small", "gemini-2.5-flash"},
		{"haiku", "gemini-2.5-flash"},
		{"medium", "gemini-2.5-pro"},
		{"sonnet", "gemini-2.5-pro"},
		{"large", "gemini-2.5-pro"},
		{"opus", "gemini-2.5-pro"},
		{"gemini-2.0-flash", "gemini-2.0-flash"}, // pass-through
		{"custom-model", "custom-model"},         // pass-through
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := p.MapModel(tc.input)
			if got != tc.want {
				t.Errorf("MapModel(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestGeminiProvider_MapPermission(t *testing.T) {
	p := NewGeminiProvider()

	tests := []struct {
		mode      PermissionMode
		wantFlag  string
		wantValue string
	}{
		{PermissionAcceptEdits, "--approval-mode", "auto_edit"},
		{PermissionPlan, "--approval-mode", "plan"},
		{PermissionBypass, "--approval-mode", "yolo"},
		{PermissionMode("unknown"), "--approval-mode", "auto_edit"},
	}

	for _, tc := range tests {
		t.Run(string(tc.mode), func(t *testing.T) {
			flag, value := p.MapPermission(tc.mode)
			if flag != tc.wantFlag {
				t.Errorf("MapPermission(%q) flag = %q, want %q", tc.mode, flag, tc.wantFlag)
			}
			if value != tc.wantValue {
				t.Errorf("MapPermission(%q) value = %q, want %q", tc.mode, value, tc.wantValue)
			}
		})
	}
}

func TestGeminiHeadlessArgs_Basic(t *testing.T) {
	opts := RunOptions{Prompt: "do the thing"}
	args := geminiHeadlessArgs(opts)

	// Must end with -p and the prompt
	n := len(args)
	if n < 2 {
		t.Fatalf("expected at least 2 args, got %d: %v", n, args)
	}
	if args[n-2] != "-p" || args[n-1] != "do the thing" {
		t.Errorf("expected args to end with [-p, prompt], got: %v", args)
	}
}

func TestGeminiHeadlessArgs_Model(t *testing.T) {
	opts := RunOptions{Prompt: "task", Model: "large"}
	args := geminiHeadlessArgs(opts)

	found := false
	for i, a := range args {
		if a == "--model" && i+1 < len(args) && args[i+1] == "gemini-2.5-pro" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --model gemini-2.5-pro in args, got: %v", args)
	}
}

func TestGeminiHeadlessArgs_Permission_Bypass(t *testing.T) {
	opts := RunOptions{Prompt: "task", Permission: PermissionBypass}
	args := geminiHeadlessArgs(opts)

	found := false
	for i, a := range args {
		if a == "--approval-mode" && i+1 < len(args) && args[i+1] == "yolo" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --approval-mode yolo in args for PermissionBypass, got: %v", args)
	}
}

func TestGeminiHeadlessArgs_Permission_Plan(t *testing.T) {
	opts := RunOptions{Prompt: "task", Permission: PermissionPlan}
	args := geminiHeadlessArgs(opts)

	found := false
	for i, a := range args {
		if a == "--approval-mode" && i+1 < len(args) && args[i+1] == "plan" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --approval-mode plan in args for PermissionPlan, got: %v", args)
	}
}

func TestGeminiHeadlessArgs_Permission_AcceptEdits(t *testing.T) {
	opts := RunOptions{Prompt: "task", Permission: PermissionAcceptEdits}
	args := geminiHeadlessArgs(opts)

	found := false
	for i, a := range args {
		if a == "--approval-mode" && i+1 < len(args) && args[i+1] == "auto_edit" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --approval-mode auto_edit in args for PermissionAcceptEdits, got: %v", args)
	}
}

func TestGeminiHeadlessArgs_PassthroughArgs(t *testing.T) {
	opts := RunOptions{Prompt: "task", PassthroughArgs: []string{"--extra", "val"}}
	args := geminiHeadlessArgs(opts)

	// PassthroughArgs must appear before -p <prompt>
	n := len(args)
	if n < 4 {
		t.Fatalf("expected at least 4 args, got %d: %v", n, args)
	}
	// -p prompt at end
	if args[n-2] != "-p" || args[n-1] != "task" {
		t.Errorf("expected args to end with [-p, prompt], got: %v", args)
	}
	found := false
	for i, a := range args[:n-2] {
		if a == "--extra" && i+1 < n-2 && args[i+1] == "val" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --extra val in args before -p, got: %v", args)
	}
}

func TestGeminiHeadlessArgs_NoModel_NoModelFlag(t *testing.T) {
	opts := RunOptions{Prompt: "task"}
	args := geminiHeadlessArgs(opts)

	for _, a := range args {
		if a == "--model" {
			t.Errorf("unexpected --model flag when Model is empty: %v", args)
		}
	}
}

func TestTypeGemini_IsValid(t *testing.T) {
	if !TypeGemini.IsValid() {
		t.Errorf("TypeGemini.IsValid() = false, want true")
	}
}

func TestBinaryName_Gemini(t *testing.T) {
	got := BinaryName(TypeGemini)
	if got != "gemini" {
		t.Errorf("BinaryName(TypeGemini) = %q, want %q", got, "gemini")
	}
}

func TestGet_Gemini(t *testing.T) {
	p := Get(TypeGemini)
	if p.Type() != TypeGemini {
		t.Errorf("Get(TypeGemini).Type() = %v, want TypeGemini", p.Type())
	}
}

func TestValidProviders_IncludesGemini(t *testing.T) {
	providers := ValidProviders()
	for _, p := range providers {
		if p == "gemini" {
			return
		}
	}
	t.Error("expected 'gemini' in valid providers")
}

func TestDetect_Gemini(t *testing.T) {
	got := Detect("gemini", "", "")
	if got != TypeGemini {
		t.Errorf("Detect(\"gemini\", ...) = %v, want TypeGemini", got)
	}
}

func TestGeminiProvider_UnsupportedOptions_Warnings(t *testing.T) {
	// Verifies that unsupported opts don't cause a panic or error in headless args building.
	// Actual warning output goes to stderr; we just ensure no crash.
	opts := RunOptions{
		Prompt:            "task",
		HooksSettingsFile: "/some/hooks.json",
		AllowedTools:      []string{"Bash"},
		DisallowedTools:   []string{"Write"},
		MaxTurns:          10,
		MCPConfig:         "/some/mcp.json",
	}
	args := geminiHeadlessArgs(opts)

	// Should still produce valid args ending with -p <prompt>
	n := len(args)
	if n < 2 || args[n-2] != "-p" || args[n-1] != "task" {
		t.Errorf("expected args to end with [-p, task], got: %v", args)
	}
}
