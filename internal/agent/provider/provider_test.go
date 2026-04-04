package provider

import (
	"fmt"
	"testing"
	"time"
)

func TestClaudeProvider_MapModel(t *testing.T) {
	p := NewClaudeProvider()

	tests := []struct {
		input string
		want  string
	}{
		{"small", "haiku"},
		{"medium", "sonnet"},
		{"large", "opus"},
		{"haiku", "haiku"},
		{"sonnet", "sonnet"},
		{"opus", "opus"},
		{"custom-model", "custom-model"}, // Pass-through
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

func TestOpenCodeProvider_MapModel(t *testing.T) {
	p := NewOpenCodeProvider()

	tests := []struct {
		input string
		want  string
	}{
		{"small", "anthropic/claude-3-5-haiku-latest"},
		{"medium", "anthropic/claude-sonnet-4-5"},
		{"large", "anthropic/claude-opus-4-5"},
		{"haiku", "anthropic/claude-3-5-haiku-latest"},
		{"sonnet", "anthropic/claude-sonnet-4-5"},
		{"opus", "anthropic/claude-opus-4-5"},
		{"anthropic/custom", "anthropic/custom"}, // Pass-through
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

func TestClaudeProvider_MapPermission(t *testing.T) {
	p := NewClaudeProvider()

	tests := []struct {
		mode      PermissionMode
		wantFlag  string
		wantValue string
	}{
		{PermissionAcceptEdits, "--permission-mode", "acceptEdits"},
		{PermissionPlan, "--permission-mode", "plan"},
		{PermissionBypass, "--dangerously-skip-permissions", ""},
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

func TestOpenCodeProvider_MapPermission(t *testing.T) {
	p := NewOpenCodeProvider()

	tests := []struct {
		mode      PermissionMode
		wantFlag  string
		wantValue string
	}{
		{PermissionAcceptEdits, "--agent", "build"},
		{PermissionPlan, "--agent", "plan"},
		{PermissionBypass, "--agent", "build"},
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

func TestDetect(t *testing.T) {
	tests := []struct {
		name           string
		cliOverride    string
		projectProvider string
		globalProvider  string
		want           Type
	}{
		{"default to claude", "", "", "", TypeClaude},
		{"cli override wins", "opencode", "claude", "claude", TypeOpenCode},
		{"project config wins over global", "", "opencode", "claude", TypeOpenCode},
		{"global config used when no project", "", "", "opencode", TypeOpenCode},
		{"invalid cli override falls through", "invalid", "opencode", "", TypeOpenCode},
		{"invalid project falls through to global", "", "invalid", "opencode", TypeOpenCode},
		{"invalid global falls through to default", "", "", "invalid", TypeClaude},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Detect(tc.cliOverride, tc.projectProvider, tc.globalProvider)
			if got != tc.want {
				t.Errorf("Detect(%q, %q, %q) = %q, want %q",
					tc.cliOverride, tc.projectProvider, tc.globalProvider, got, tc.want)
			}
		})
	}
}

func TestType_IsValid(t *testing.T) {
	tests := []struct {
		t    Type
		want bool
	}{
		{TypeClaude, true},
		{TypeOpenCode, true},
		{Type("invalid"), false},
		{Type(""), false},
	}

	for _, tc := range tests {
		t.Run(string(tc.t), func(t *testing.T) {
			got := tc.t.IsValid()
			if got != tc.want {
				t.Errorf("Type(%q).IsValid() = %v, want %v", tc.t, got, tc.want)
			}
		})
	}
}

func TestApplyModelOverrides(t *testing.T) {
	claudeProvider := NewClaudeProvider()
	openCodeProvider := NewOpenCodeProvider()

	tests := []struct {
		name      string
		canonical string
		overrides ModelOverrides
		provider  Provider
		want      string
	}{
		{
			name:      "no overrides uses provider default",
			canonical: "opus",
			overrides: nil,
			provider:  claudeProvider,
			want:      "opus",
		},
		{
			name:      "override takes precedence",
			canonical: "opus",
			overrides: ModelOverrides{"opus": "custom-opus"},
			provider:  claudeProvider,
			want:      "custom-opus",
		},
		{
			name:      "non-matching override uses provider default",
			canonical: "sonnet",
			overrides: ModelOverrides{"opus": "custom-opus"},
			provider:  claudeProvider,
			want:      "sonnet",
		},
		{
			name:      "override with OpenCode provider",
			canonical: "opus",
			overrides: ModelOverrides{"opus": "anthropic/claude-opus-5"},
			provider:  openCodeProvider,
			want:      "anthropic/claude-opus-5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyModelOverrides(tc.canonical, tc.overrides, tc.provider)
			if got != tc.want {
				t.Errorf("ApplyModelOverrides(%q, %v, %T) = %q, want %q",
					tc.canonical, tc.overrides, tc.provider, got, tc.want)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   time.Duration
	}{
		{"30 seconds", "Rate limited. Try again in 30 seconds.", 30 * time.Second},
		{"2 minutes", "Please retry after 2 minutes", 2 * time.Minute},
		{"1 hour", "Wait 1 hour before retrying", 1 * time.Hour},
		{"5 minute singular", "Retry in 5 minute", 5 * time.Minute},
		{"no time", "Rate limit exceeded", 0},
		{"empty", "", 0},
		{"60 seconds", "Wait 60 seconds", 60 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRetryAfter(tc.output)
			if got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}

func TestGet(t *testing.T) {
	t.Run("returns ClaudeProvider for TypeClaude", func(t *testing.T) {
		p := Get(TypeClaude)
		if p.Type() != TypeClaude {
			t.Errorf("Get(TypeClaude).Type() = %v, want TypeClaude", p.Type())
		}
	})

	t.Run("returns OpenCodeProvider for TypeOpenCode", func(t *testing.T) {
		p := Get(TypeOpenCode)
		if p.Type() != TypeOpenCode {
			t.Errorf("Get(TypeOpenCode).Type() = %v, want TypeOpenCode", p.Type())
		}
	})

	t.Run("defaults to ClaudeProvider for unknown type", func(t *testing.T) {
		p := Get(Type("unknown"))
		if p.Type() != TypeClaude {
			t.Errorf("Get(unknown).Type() = %v, want TypeClaude", p.Type())
		}
	})
}

func TestValidProviders(t *testing.T) {
	providers := ValidProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	// Check both providers are present
	found := make(map[string]bool)
	for _, p := range providers {
		found[p] = true
	}

	if !found["claude"] {
		t.Error("expected 'claude' in valid providers")
	}
	if !found["opencode"] {
		t.Error("expected 'opencode' in valid providers")
	}
}

func TestOpenCodeProvider_ParseRateLimit(t *testing.T) {
	p := NewOpenCodeProvider()

	tests := []struct {
		name        string
		output      string
		wantLimited bool
	}{
		// Common rate limit patterns
		{"rate limit", "Error: rate limit exceeded", true},
		{"429 status", "HTTP 429 Too Many Requests", true},
		{"overloaded", "Server is overloaded, please try again", true},
		{"throttled", "Request was throttled", true},

		// OpenAI-specific patterns
		{"quota exceeded", "You exceeded your quota for the month", true},
		{"tpm limit", "TPM limit reached for model", true},
		{"rpm limit", "RPM limit exceeded", true},

		// Case insensitivity
		{"RATE LIMIT caps", "RATE LIMIT ERROR", true},
		{"Quota Exceeded caps", "Quota Exceeded", true},

		// Non-matching output
		{"normal output", "Task completed successfully", false},
		{"empty output", "", false},
		{"partial match no rate", "This is a limited feature", false},
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

func TestOpenCodeProvider_ParseRateLimitWithRetryAfter(t *testing.T) {
	p := NewOpenCodeProvider()

	result := &RunResult{Output: "Rate limit exceeded. Please retry in 30 seconds."}
	p.parseRateLimit(result)

	if !result.RateLimited {
		t.Error("expected RateLimited=true")
	}
	if result.RetryAfter != 30*time.Second {
		t.Errorf("expected RetryAfter=30s, got %v", result.RetryAfter)
	}
}

func TestBinaryName(t *testing.T) {
	tests := []struct {
		providerType Type
		want         string
	}{
		{TypeClaude, "claude"},
		{TypeOpenCode, "opencode"},
		{Type("invalid"), ""},
		{Type(""), ""},
	}

	for _, tc := range tests {
		t.Run(string(tc.providerType), func(t *testing.T) {
			got := BinaryName(tc.providerType)
			if got != tc.want {
				t.Errorf("BinaryName(%q) = %q, want %q", tc.providerType, got, tc.want)
			}
		})
	}
}

func TestGetWithDetection(t *testing.T) {
	tests := []struct {
		name            string
		cliOverride     string
		projectProvider string
		globalProvider  string
		wantType        Type
	}{
		{"all empty defaults to claude", "", "", "", TypeClaude},
		{"cli override wins", "opencode", "claude", "claude", TypeOpenCode},
		{"project wins over global", "", "opencode", "claude", TypeOpenCode},
		{"global used when no project", "", "", "opencode", TypeOpenCode},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := GetWithDetection(tc.cliOverride, tc.projectProvider, tc.globalProvider)
			if p.Type() != tc.wantType {
				t.Errorf("GetWithDetection() provider type = %v, want %v", p.Type(), tc.wantType)
			}
		})
	}
}

func TestClaudeProvider_Type(t *testing.T) {
	p := NewClaudeProvider()
	if p.Type() != TypeClaude {
		t.Errorf("ClaudeProvider.Type() = %v, want %v", p.Type(), TypeClaude)
	}
}

func TestOpenCodeProvider_Type(t *testing.T) {
	p := NewOpenCodeProvider()
	if p.Type() != TypeOpenCode {
		t.Errorf("OpenCodeProvider.Type() = %v, want %v", p.Type(), TypeOpenCode)
	}
}

func TestParseRateLimit_ClaudeProvider(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantLimited bool
	}{
		{"rate limit phrase", "Error: rate limit exceeded", true},
		{"rate_limit underscore", "Error: rate_limit_error", true},
		{"429 status code", "HTTP 429 Too Many Requests", true},
		{"overloaded", "API is overloaded", true},
		{"capacity", "Not enough capacity", true},
		{"try again", "Please try again later", true},
		{"throttled", "Request was throttled", true},
		{"normal output", "Task completed successfully", false},
		{"empty output", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &RunResult{Output: tc.output}
			parseRateLimit(result)

			if result.RateLimited != tc.wantLimited {
				t.Errorf("parseRateLimit(%q) RateLimited = %v, want %v",
					tc.output, result.RateLimited, tc.wantLimited)
			}
		})
	}
}

func TestParseOverloadExhausted(t *testing.T) {
	tests := []struct {
		name              string
		output            string
		exitCode          int
		hasError          bool
		wantExhausted     bool
	}{
		{"529 with error", "Error: 529 overloaded_error", 1, true, true},
		{"overloaded_error", "overloaded_error occurred", 1, true, true},
		{"api is overloaded", "api is overloaded, retries exhausted", 1, true, true},
		{"no error exit code 0", "529 overloaded_error", 0, false, false},
		{"normal exit with overloaded", "Server overloaded", 1, false, true},
		{"normal output", "Task completed", 0, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &RunResult{
				Output:   tc.output,
				ExitCode: tc.exitCode,
			}
			if tc.hasError {
				result.Error = fmt.Errorf("some error")
			}

			parseOverloadExhausted(result)

			if result.OverloadExhausted != tc.wantExhausted {
				t.Errorf("parseOverloadExhausted() OverloadExhausted = %v, want %v",
					result.OverloadExhausted, tc.wantExhausted)
			}
		})
	}
}

func TestOpenCodeProvider_ParseOverloadExhausted(t *testing.T) {
	p := NewOpenCodeProvider()

	tests := []struct {
		name          string
		output        string
		exitCode      int
		hasError      bool
		wantExhausted bool
	}{
		{"529 with error", "Error: 529", 1, true, true},
		{"quota exceeded", "quota exceeded for account", 1, true, true},
		{"overloaded with error", "server overloaded", 1, true, true},
		{"no error", "529", 0, false, false},
		{"normal output", "Task completed", 0, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := &RunResult{
				Output:   tc.output,
				ExitCode: tc.exitCode,
			}
			if tc.hasError {
				result.Error = fmt.Errorf("some error")
			}

			p.parseOverloadExhausted(result)

			if result.OverloadExhausted != tc.wantExhausted {
				t.Errorf("parseOverloadExhausted() OverloadExhausted = %v, want %v",
					result.OverloadExhausted, tc.wantExhausted)
			}
		})
	}
}

func TestParseRateLimitWithError(t *testing.T) {
	result := &RunResult{
		Output: "Normal output",
		Error:  fmt.Errorf("rate limit exceeded"),
	}

	parseRateLimit(result)

	if !result.RateLimited {
		t.Error("parseRateLimit should detect rate limit in error message")
	}
}

func TestClaudeProvider_MapPermission_Default(t *testing.T) {
	p := NewClaudeProvider()

	// Test default case (unknown permission mode)
	flag, value := p.MapPermission(PermissionMode("unknown"))
	if flag != "--permission-mode" {
		t.Errorf("flag = %q, want '--permission-mode'", flag)
	}
	if value != "acceptEdits" {
		t.Errorf("value = %q, want 'acceptEdits'", value)
	}
}

func TestOpenCodeProvider_MapPermission_Default(t *testing.T) {
	p := NewOpenCodeProvider()

	// Test default case (unknown permission mode)
	flag, value := p.MapPermission(PermissionMode("unknown"))
	if flag != "--agent" {
		t.Errorf("flag = %q, want '--agent'", flag)
	}
	if value != "build" {
		t.Errorf("value = %q, want 'build'", value)
	}
}
