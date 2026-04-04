package provider

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestCustomProvider_Type(t *testing.T) {
	p := NewCustomProvider("echo hello")
	if p.Type() != TypeCustom {
		t.Errorf("CustomProvider.Type() = %v, want TypeCustom", p.Type())
	}
}

func TestCustomProvider_MapModel_PassThrough(t *testing.T) {
	p := NewCustomProvider("my-agent")
	tests := []string{"sonnet", "opus", "haiku", "small", "large", "gpt-4.1", ""}
	for _, model := range tests {
		if got := p.MapModel(model); got != model {
			t.Errorf("MapModel(%q) = %q, want %q (pass-through)", model, got, model)
		}
	}
}

func TestCustomProvider_MapPermission_ReturnsEmpty(t *testing.T) {
	p := NewCustomProvider("my-agent")
	flag, value := p.MapPermission(PermissionAcceptEdits)
	if flag != "" || value != "" {
		t.Errorf("MapPermission() = (%q, %q), want empty strings", flag, value)
	}
}

func TestTypeCustom_IsValid(t *testing.T) {
	if !TypeCustom.IsValid() {
		t.Error("TypeCustom.IsValid() = false, want true")
	}
}

func TestValidProviders_IncludesCustom(t *testing.T) {
	providers := ValidProviders()
	for _, p := range providers {
		if p == "custom" {
			return
		}
	}
	t.Error("expected 'custom' in valid providers")
}

func TestGet_Custom(t *testing.T) {
	p := Get(TypeCustom)
	if p.Type() != TypeCustom {
		t.Errorf("Get(TypeCustom).Type() = %v, want TypeCustom", p.Type())
	}
}

func TestBinaryName_Custom(t *testing.T) {
	// Custom provider has no fixed binary name
	got := BinaryName(TypeCustom)
	if got != "" {
		t.Errorf("BinaryName(TypeCustom) = %q, want empty", got)
	}
}

// --- Template substitution ---

func TestCustomCmdArgs_PromptSubstitution(t *testing.T) {
	opts := RunOptions{Prompt: "do the thing"}
	cmd, args, cleanup, err := buildCustomCmd("my-agent --prompt {prompt}", opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("buildCustomCmd() error = %v", err)
	}
	if cmd != "my-agent" {
		t.Errorf("cmd = %q, want %q", cmd, "my-agent")
	}
	found := false
	for _, a := range args {
		if a == "do the thing" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected prompt 'do the thing' in args, got: %v", args)
	}
}

func TestCustomCmdArgs_ModelSubstitution(t *testing.T) {
	opts := RunOptions{Prompt: "task", Model: "sonnet"}
	_, args, cleanup, err := buildCustomCmd("my-agent --model {model} --prompt {prompt}", opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("buildCustomCmd() error = %v", err)
	}
	found := false
	for i, a := range args {
		if a == "--model" && i+1 < len(args) && args[i+1] == "sonnet" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --model sonnet in args, got: %v", args)
	}
}

func TestCustomCmdArgs_TimeoutSubstitution(t *testing.T) {
	opts := RunOptions{Prompt: "task", Timeout: 30 * time.Second}
	_, args, cleanup, err := buildCustomCmd("my-agent --timeout {timeout} {prompt}", opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("buildCustomCmd() error = %v", err)
	}
	found := false
	for i, a := range args {
		if a == "--timeout" && i+1 < len(args) && args[i+1] == "30" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --timeout 30 in args, got: %v", args)
	}
}

func TestCustomCmdArgs_WorkdirSubstitution(t *testing.T) {
	opts := RunOptions{Prompt: "task", WorkingDir: "/my/project"}
	_, args, cleanup, err := buildCustomCmd("my-agent --workdir {workdir} {prompt}", opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("buildCustomCmd() error = %v", err)
	}
	found := false
	for i, a := range args {
		if a == "--workdir" && i+1 < len(args) && args[i+1] == "/my/project" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --workdir /my/project in args, got: %v", args)
	}
}

func TestCustomCmdArgs_PromptFileSubstitution(t *testing.T) {
	opts := RunOptions{Prompt: "hello world prompt"}
	_, args, cleanup, err := buildCustomCmd("my-agent --file {prompt_file}", opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("buildCustomCmd() error = %v", err)
	}
	// One of the args should be a file path that exists and contains the prompt
	var filePath string
	for i, a := range args {
		if a == "--file" && i+1 < len(args) {
			filePath = args[i+1]
		}
	}
	if filePath == "" {
		t.Fatalf("expected --file <path> in args, got: %v", args)
	}
	content, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("prompt file not readable: %v", readErr)
	}
	if string(content) != "hello world prompt" {
		t.Errorf("prompt file content = %q, want %q", string(content), "hello world prompt")
	}
}

func TestCustomCmdArgs_NoVariables(t *testing.T) {
	opts := RunOptions{Prompt: "task"}
	cmd, args, cleanup, err := buildCustomCmd("my-agent --flag val", opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("buildCustomCmd() error = %v", err)
	}
	if cmd != "my-agent" {
		t.Errorf("cmd = %q, want my-agent", cmd)
	}
	if len(args) != 2 || args[0] != "--flag" || args[1] != "val" {
		t.Errorf("args = %v, want [--flag val]", args)
	}
}

func TestCustomCmdArgs_EmptyModel(t *testing.T) {
	opts := RunOptions{Prompt: "task", Model: ""}
	_, args, cleanup, err := buildCustomCmd("my-agent {model} {prompt}", opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("buildCustomCmd() error = %v", err)
	}
	// {model} with empty model → replaced with ""
	for _, a := range args {
		if a == "{model}" {
			t.Errorf("expected {model} to be substituted, but found literal in args: %v", args)
		}
	}
}

func TestCustomCmdArgs_TimeoutZero(t *testing.T) {
	opts := RunOptions{Prompt: "task", Timeout: 0}
	_, args, cleanup, err := buildCustomCmd("my-agent --timeout {timeout} {prompt}", opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatalf("buildCustomCmd() error = %v", err)
	}
	for i, a := range args {
		if a == "--timeout" && i+1 < len(args) && args[i+1] == "0" {
			return // found correctly substituted
		}
	}
	t.Errorf("expected --timeout 0 in args, got: %v", args)
}

// --- Execution tests ---

func TestCustomProvider_Run_HeadlessCapturesOutput(t *testing.T) {
	p := NewCustomProvider("echo juggle-custom-test")
	result, err := p.Run(RunOptions{Mode: ModeHeadless})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Output, "juggle-custom-test") {
		t.Errorf("expected output to contain 'juggle-custom-test', got: %q", result.Output)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestCustomProvider_Run_ExitCodeCaptured(t *testing.T) {
	p := NewCustomProvider("sh -c \"exit 2\"")
	result, err := p.Run(RunOptions{Mode: ModeHeadless})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 2 {
		t.Errorf("expected exit code 2, got %d", result.ExitCode)
	}
	if result.Error == nil {
		t.Error("expected non-nil Error for non-zero exit")
	}
}

func TestCustomProvider_Run_TokenCountsZero(t *testing.T) {
	p := NewCustomProvider("echo hi")
	result, err := p.Run(RunOptions{Mode: ModeHeadless})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.InputTokens != 0 || result.OutputTokens != 0 || result.CacheTokens != 0 {
		t.Errorf("expected zero token counts, got in=%d out=%d cache=%d",
			result.InputTokens, result.OutputTokens, result.CacheTokens)
	}
}

func TestCustomProvider_Run_PromptSubstituted(t *testing.T) {
	p := NewCustomProvider("echo {prompt}")
	result, err := p.Run(RunOptions{Mode: ModeHeadless, Prompt: "hello-from-juggle"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(result.Output, "hello-from-juggle") {
		t.Errorf("expected output to contain prompt, got: %q", result.Output)
	}
}

func TestCustomProvider_Run_Timeout(t *testing.T) {
	p := NewCustomProvider("sleep 60")
	result, err := p.Run(RunOptions{
		Mode:    ModeHeadless,
		Timeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.TimedOut {
		t.Error("expected TimedOut=true")
	}
}

func TestDetect_Custom(t *testing.T) {
	got := Detect("custom", "", "")
	if got != TypeCustom {
		t.Errorf("Detect(\"custom\", ...) = %v, want TypeCustom", got)
	}
}
