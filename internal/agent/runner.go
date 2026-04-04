// Package agent provides the agent prompt template and runner interface
// for running AI agents with juggle.
package agent

import (
	"github.com/ohare93/juggle/internal/agent/provider"
)

// RunMode defines how the agent should be executed
type RunMode = provider.RunMode

const (
	// ModeHeadless runs with captured output, no terminal interaction
	ModeHeadless = provider.ModeHeadless
	// ModeInteractive runs with terminal TUI, inherits stdin/stdout/stderr
	ModeInteractive = provider.ModeInteractive
)

// PermissionMode defines the agent's permission level
type PermissionMode = provider.PermissionMode

const (
	// PermissionAcceptEdits allows file edits with confirmation
	PermissionAcceptEdits = provider.PermissionAcceptEdits
	// PermissionPlan starts in plan/read-only mode
	PermissionPlan = provider.PermissionPlan
	// PermissionBypass bypasses all permission checks (dangerous)
	PermissionBypass = provider.PermissionBypass
)

// RunOptions configures how the agent is executed
type RunOptions = provider.RunOptions

// RunResult represents the outcome of a single agent run
type RunResult = provider.RunResult

// Runner defines the interface for running AI agents.
// Implementations must execute an agent with options and return the result.
type Runner interface {
	// Run executes the agent with the given options and returns the result.
	Run(opts RunOptions) (*RunResult, error)
}

// ProviderRunner wraps a provider.Provider to implement Runner
type ProviderRunner struct {
	Provider       provider.Provider
	ModelOverrides provider.ModelOverrides
}

// Run executes the agent using the configured provider
func (r *ProviderRunner) Run(opts RunOptions) (*RunResult, error) {
	p := r.Provider
	if p == nil {
		// Default to Claude provider if not configured
		p = provider.NewClaudeProvider()
	}

	// Apply model overrides if configured
	if opts.Model != "" && r.ModelOverrides != nil {
		opts.Model = provider.ApplyModelOverrides(opts.Model, r.ModelOverrides, p)
	}

	return p.Run(opts)
}

// MockRunner is a test implementation of Runner
type MockRunner struct {
	// Responses is a queue of results to return (FIFO)
	Responses []*RunResult
	// Calls records all calls made to Run
	Calls []RunOptions
	// NextIndex tracks which response to return next
	NextIndex int
}

// NewMockRunner creates a new MockRunner with the given responses
func NewMockRunner(responses ...*RunResult) *MockRunner {
	return &MockRunner{
		Responses: responses,
		Calls:     make([]RunOptions, 0),
	}
}

// Run records the call and returns the next queued response
func (m *MockRunner) Run(opts RunOptions) (*RunResult, error) {
	m.Calls = append(m.Calls, opts)

	if m.NextIndex >= len(m.Responses) {
		// Return a default error result if no more responses queued
		return &RunResult{
			Output:   "No more mock responses",
			ExitCode: 1,
		}, nil
	}

	result := m.Responses[m.NextIndex]
	m.NextIndex++
	return result, nil
}

// Reset clears call history and resets response index
func (m *MockRunner) Reset() {
	m.Calls = make([]RunOptions, 0)
	m.NextIndex = 0
}

// SetResponses replaces the response queue
func (m *MockRunner) SetResponses(responses ...*RunResult) {
	m.Responses = responses
	m.NextIndex = 0
}
