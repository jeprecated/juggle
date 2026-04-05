package provider

import (
	"fmt"
	"testing"
	"time"
)

func TestCopilotProvider_Type(t *testing.T) {
	p := NewCopilotProvider()
	if p.Type() != TypeCopilot {
		t.Errorf("CopilotProvider.Type() = %v, want %v", p.Type(), TypeCopilot)
	}
}

func TestCopilotProvider_MapModel(t *testing.T) {
	p := NewCopilotProvider()

	tests := []struct {
		input string
		want  string
	}{
		{"small", "claude-haiku-4-5"},
		{"haiku", "claude-haiku-4-5"},
		{"medium", "claude-sonnet-4-5"},
		{"sonnet", "claude-sonnet-4-5"},
		{"large", "claude-opus-4-5"},
		{"opus", "claude-opus-4-5"},
		{"gpt-4o", "gpt-4o"},                     // pass-through
		{"gemini-2.0-flash", "gemini-2.0-flash"}, // pass-through
		{"custom-model", "custom-model"},          // pass-through
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

func TestCopilotProvider_MapPermission(t *testing.T) {
	p := NewCopilotProvider()

	tests := []struct {
		mode      PermissionMode
		wantFlag  string
		wantValue string
	}{
		{PermissionBypass, "--yolo", ""},
		{PermissionAcceptEdits, "--allow-all-tools", ""},
		{PermissionPlan, "", ""},
		{PermissionMode("unknown"), "", ""},
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

func TestCopilotHeadlessArgs_AlwaysIncludesAutopilot(t *testing.T) {
	opts := RunOptions{Prompt: "do the thing"}
	args := copilotHeadlessArgs(opts)

	for _, a := range args {
		if a == "--autopilot" {
			return
		}
	}
	t.Errorf("expected --autopilot in headless args, got: %v", args)
}

func TestCopilotHeadlessArgs_AlwaysIncludesSilent(t *testing.T) {
	opts := RunOptions{Prompt: "do the thing"}
	args := copilotHeadlessArgs(opts)

	for _, a := range args {
		if a == "-s" {
			return
		}
	}
	t.Errorf("expected -s in headless args, got: %v", args)
}

func TestCopilotHeadlessArgs_Basic(t *testing.T) {
	opts := RunOptions{Prompt: "do the thing"}
	args := copilotHeadlessArgs(opts)

	// Must end with -p and the prompt
	n := len(args)
	if n < 2 {
		t.Fatalf("expected at least 2 args, got %d: %v", n, args)
	}
	if args[n-2] != "-p" || args[n-1] != "do the thing" {
		t.Errorf("expected args to end with [-p, prompt], got: %v", args)
	}
}

func TestCopilotHeadlessArgs_Model(t *testing.T) {
	opts := RunOptions{Prompt: "task", Model: "large"}
	args := copilotHeadlessArgs(opts)

	found := false
	for _, a := range args {
		if a == "--model=claude-opus-4-5" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --model=claude-opus-4-5 in args, got: %v", args)
	}
}

func TestCopilotHeadlessArgs_NoModel_NoModelFlag(t *testing.T) {
	opts := RunOptions{Prompt: "task"}
	args := copilotHeadlessArgs(opts)

	for _, a := range args {
		if len(a) > 7 && a[:8] == "--model=" {
			t.Errorf("unexpected --model= flag when Model is empty: %v", args)
		}
	}
}

func TestCopilotHeadlessArgs_Permission_Bypass(t *testing.T) {
	opts := RunOptions{Prompt: "task", Permission: PermissionBypass}
	args := copilotHeadlessArgs(opts)

	found := false
	for _, a := range args {
		if a == "--yolo" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --yolo in args for PermissionBypass, got: %v", args)
	}
}

func TestCopilotHeadlessArgs_Permission_AcceptEdits(t *testing.T) {
	opts := RunOptions{Prompt: "task", Permission: PermissionAcceptEdits}
	args := copilotHeadlessArgs(opts)

	found := false
	for _, a := range args {
		if a == "--allow-all-tools" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --allow-all-tools in args for PermissionAcceptEdits, got: %v", args)
	}
}

func TestCopilotHeadlessArgs_Permission_Plan_NoAutonomousFlags(t *testing.T) {
	opts := RunOptions{Prompt: "task", Permission: PermissionPlan}
	args := copilotHeadlessArgs(opts)

	for _, a := range args {
		if a == "--yolo" || a == "--allow-all-tools" {
			t.Errorf("unexpected autonomous flag %q for PermissionPlan: %v", a, args)
		}
	}
}

func TestCopilotHeadlessArgs_MaxTurns(t *testing.T) {
	opts := RunOptions{Prompt: "task", MaxTurns: 10}
	args := copilotHeadlessArgs(opts)

	found := false
	for i, a := range args {
		if a == "--max-autopilot-continues" && i+1 < len(args) && args[i+1] == "10" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --max-autopilot-continues 10 in args, got: %v", args)
	}
}

func TestCopilotHeadlessArgs_MaxTurns_Zero(t *testing.T) {
	opts := RunOptions{Prompt: "task", MaxTurns: 0}
	args := copilotHeadlessArgs(opts)

	for _, a := range args {
		if a == "--max-autopilot-continues" {
			t.Errorf("unexpected --max-autopilot-continues flag when MaxTurns=0: %v", args)
		}
	}
}

func TestCopilotHeadlessArgs_AllowedTools(t *testing.T) {
	opts := RunOptions{Prompt: "task", AllowedTools: []string{"Bash", "Read"}}
	args := copilotHeadlessArgs(opts)

	bashFound, readFound := false, false
	for i, a := range args {
		if a == "--allow-tool" && i+1 < len(args) {
			switch args[i+1] {
			case "Bash":
				bashFound = true
			case "Read":
				readFound = true
			}
		}
	}
	if !bashFound {
		t.Errorf("expected --allow-tool Bash in args, got: %v", args)
	}
	if !readFound {
		t.Errorf("expected --allow-tool Read in args, got: %v", args)
	}
}

func TestCopilotHeadlessArgs_DisallowedTools(t *testing.T) {
	opts := RunOptions{Prompt: "task", DisallowedTools: []string{"Write", "Edit"}}
	args := copilotHeadlessArgs(opts)

	writeFound, editFound := false, false
	for i, a := range args {
		if a == "--deny-tool" && i+1 < len(args) {
			switch args[i+1] {
			case "Write":
				writeFound = true
			case "Edit":
				editFound = true
			}
		}
	}
	if !writeFound {
		t.Errorf("expected --deny-tool Write in args, got: %v", args)
	}
	if !editFound {
		t.Errorf("expected --deny-tool Edit in args, got: %v", args)
	}
}

func TestCopilotHeadlessArgs_PassthroughArgs(t *testing.T) {
	opts := RunOptions{Prompt: "task", PassthroughArgs: []string{"--extra", "val"}}
	args := copilotHeadlessArgs(opts)

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

func TestTypeCopilot_IsValid(t *testing.T) {
	if !TypeCopilot.IsValid() {
		t.Errorf("TypeCopilot.IsValid() = false, want true")
	}
}

func TestBinaryName_Copilot(t *testing.T) {
	got := BinaryName(TypeCopilot)
	if got != "copilot" {
		t.Errorf("BinaryName(TypeCopilot) = %q, want %q", got, "copilot")
	}
}

func TestGet_Copilot(t *testing.T) {
	p := Get(TypeCopilot)
	if p.Type() != TypeCopilot {
		t.Errorf("Get(TypeCopilot).Type() = %v, want TypeCopilot", p.Type())
	}
}

func TestValidProviders_IncludesCopilot(t *testing.T) {
	providers := ValidProviders()
	for _, p := range providers {
		if p == "copilot" {
			return
		}
	}
	t.Error("expected 'copilot' in valid providers")
}

func TestDetect_Copilot(t *testing.T) {
	got := Detect("copilot", "", "")
	if got != TypeCopilot {
		t.Errorf("Detect(\"copilot\", ...) = %v, want TypeCopilot", got)
	}
}

func TestCopilotProvider_ParseRateLimit_RateLimit(t *testing.T) {
	p := NewCopilotProvider()

	tests := []struct {
		name        string
		output      string
		wantLimited bool
	}{
		{"rate limit exceeded", "Error: rate limit exceeded", true},
		{"429 status", "HTTP 429 Too Many Requests", true},
		{"too many requests", "too many requests for this model", true},
		{"copilot specific", "GitHub Copilot rate limit exceeded for your plan", true},
		{"normal output", "Task completed successfully", false},
		{"empty output", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &RunResult{Output: tc.output}
			p.parseRateLimit(result)
			if result.RateLimited != tc.wantLimited {
				t.Errorf("parseRateLimit(%q) RateLimited = %v, want %v",
					tc.output, result.RateLimited, tc.wantLimited)
			}
		})
	}
}

func TestCopilotProvider_ParseRateLimit_RetryAfter(t *testing.T) {
	p := NewCopilotProvider()

	result := &RunResult{Output: "Rate limit exceeded. Please retry in 30 seconds."}
	p.parseRateLimit(result)

	if !result.RateLimited {
		t.Error("expected RateLimited=true")
	}
	if result.RetryAfter != 30*time.Second {
		t.Errorf("expected RetryAfter=30s, got %v", result.RetryAfter)
	}
}

func TestCopilotProvider_ParseRateLimit_FromError(t *testing.T) {
	p := NewCopilotProvider()
	result := &RunResult{
		Output: "Normal output",
		Error:  fmt.Errorf("rate limit exceeded"),
	}
	p.parseRateLimit(result)
	if !result.RateLimited {
		t.Error("expected RateLimited=true when error contains rate limit message")
	}
}
