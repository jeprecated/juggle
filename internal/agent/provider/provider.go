// Package provider defines the interface and implementations for AI agent backends.
// It supports multiple agent CLIs (Claude Code, OpenCode) through a common abstraction.
package provider

import (
	"context"
	"time"
)

// Type represents the agent provider type
type Type string

const (
	// TypeClaude is the Claude Code CLI provider (default)
	TypeClaude Type = "claude"
	// TypeOpenCode is the OpenCode CLI provider
	TypeOpenCode Type = "opencode"
	// TypeCodex is the OpenAI Codex CLI provider
	TypeCodex Type = "codex"
	// TypeCustom is a user-defined agent CLI provider
	TypeCustom Type = "custom"
	// TypeGemini is the Google Gemini CLI provider
	TypeGemini Type = "gemini"
)

// String returns the string representation
func (p Type) String() string {
	return string(p)
}

// IsValid returns true if the provider type is known
func (p Type) IsValid() bool {
	return p == TypeClaude || p == TypeOpenCode || p == TypeCodex || p == TypeCustom || p == TypeGemini
}

// RunMode defines how the agent should be executed
type RunMode string

const (
	// ModeHeadless runs with captured output, no terminal interaction
	ModeHeadless RunMode = "headless"
	// ModeInteractive runs with terminal TUI, inherits stdin/stdout/stderr
	ModeInteractive RunMode = "interactive"
)

// PermissionMode defines the agent's permission level
type PermissionMode string

const (
	// PermissionAcceptEdits allows file edits with confirmation
	PermissionAcceptEdits PermissionMode = "acceptEdits"
	// PermissionPlan starts in plan/read-only mode
	PermissionPlan PermissionMode = "plan"
	// PermissionBypass bypasses all permission checks (dangerous)
	PermissionBypass PermissionMode = "bypassPermissions"
)

// RunOptions configures how the agent is executed (provider-agnostic)
type RunOptions struct {
	Prompt       string          // The prompt to send to the agent
	Mode         RunMode         // headless vs interactive
	Permission   PermissionMode  // acceptEdits, plan, bypassPermissions
	Timeout      time.Duration   // timeout per invocation (0 = no timeout)
	SystemPrompt string          // optional additional system prompt
	Model        string          // canonical model name (e.g., "opus", "sonnet", "haiku")
	WorkingDir   string          // working directory for command execution
	ShowThinking bool            // Include thinking blocks in output
	Verbose      bool            // Show tool inputs alongside tool names
	Context      context.Context // optional external cancellation context (force-kill)
	Env               []string        // additional environment variables (KEY=VALUE) passed to the subprocess
	HooksSettingsFile string          // path to temp settings JSON for agent-internal hooks (Claude-specific)
	AllowedTools      []string        // restrict agent to these tools only (mutually exclusive with DisallowedTools)
	DisallowedTools   []string        // block specific tools (mutually exclusive with AllowedTools)
	MaxTurns          int             // cap tool-use turns per iteration (0 = provider default)
	MCPConfig         string          // path to MCP server config file (Claude-specific)
	PassthroughArgs   []string        // extra flags appended verbatim to the agent CLI command
}

// RunResult represents the outcome of a single agent run (provider-agnostic)
type RunResult struct {
	Output            string        // Full output from the agent
	ExitCode          int           // Process exit code
	TimedOut          bool          // Execution timed out
	RateLimited       bool          // Rate limit error detected
	RetryAfter        time.Duration // Suggested wait time from rate limit (0 if not specified)
	OverloadExhausted bool          // Agent exited after exhausting overload retries
	QuotaExhausted    bool          // Usage/quota limit hit (daily, monthly, billing cap)
	QuotaResetsAt     time.Time     // When the quota window resets (zero if unknown)
	Error             error         // Execution error (if any)

	// Stream-JSON metrics (populated when StreamJSON=true)
	InputTokens    int      // Input tokens consumed
	OutputTokens   int      // Output tokens generated
	CacheTokens    int      // Cached tokens used
	ThinkingBlocks []string // Thinking blocks (if ShowThinking=true)
}

// Provider defines the interface for AI agent backends
type Provider interface {
	// Type returns the provider type identifier
	Type() Type

	// Run executes the agent with options and returns the result
	Run(opts RunOptions) (*RunResult, error)

	// MapModel converts canonical model name to provider-specific format
	// Canonical names: "haiku", "sonnet", "opus" (or "small", "medium", "large")
	MapModel(canonical string) string

	// MapPermission converts PermissionMode to provider-specific flag/argument
	// Returns the flag name and value, or empty strings if not supported
	MapPermission(mode PermissionMode) (flag, value string)
}
