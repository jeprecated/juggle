package provider

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// quotaPatterns matches usage/quota exhaustion errors (distinct from transient rate limits).
var quotaPatterns = []string{
	"daily limit",
	"daily quota",
	"usage limit",
	"usage quota",
	"monthly limit",
	"monthly quota",
	"billing limit",
	"insufficient_quota",
	"quota_exceeded",
	"quota exceeded",
	"usage_limit_reached",
}

// ClaudeProvider implements Provider for Claude Code CLI
type ClaudeProvider struct{}

// NewClaudeProvider creates a new Claude Code provider
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{}
}

// Type returns TypeClaude
func (c *ClaudeProvider) Type() Type {
	return TypeClaude
}

// MapModel converts canonical model name to Claude format
// Claude uses: haiku, sonnet, opus
func (c *ClaudeProvider) MapModel(canonical string) string {
	switch canonical {
	case "small":
		return "haiku"
	case "medium":
		return "sonnet"
	case "large":
		return "opus"
	default:
		// Already in Claude format or custom model
		return canonical
	}
}

// MapPermission converts PermissionMode to Claude CLI flags
func (c *ClaudeProvider) MapPermission(mode PermissionMode) (flag, value string) {
	switch mode {
	case PermissionBypass:
		return "--dangerously-skip-permissions", ""
	case PermissionPlan:
		return "--permission-mode", "plan"
	case PermissionAcceptEdits:
		return "--permission-mode", "acceptEdits"
	default:
		return "--permission-mode", "acceptEdits"
	}
}

// Run executes Claude CLI with the given options
func (c *ClaudeProvider) Run(opts RunOptions) (*RunResult, error) {
	if opts.Mode == ModeInteractive {
		return c.runInteractive(opts)
	}
	return c.runHeadless(opts)
}

// claudeHeadlessArgs builds the CLI argument list for a headless Claude invocation.
// Extracted for testability.
func claudeHeadlessArgs(opts RunOptions) []string {
	args := []string{
		"--disable-slash-commands",
	}

	if opts.SystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.SystemPrompt)
	}

	if opts.Model != "" {
		p := NewClaudeProvider()
		args = append(args, "--model", p.MapModel(opts.Model))
	}

	p := NewClaudeProvider()
	flag, value := p.MapPermission(opts.Permission)
	if value != "" {
		args = append(args, flag, value)
	} else {
		args = append(args, flag)
	}

	if opts.HooksSettingsFile != "" {
		args = append(args, "--settings", opts.HooksSettingsFile)
	}

	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}

	if len(opts.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(opts.DisallowedTools, ","))
	}

	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}

	if opts.MCPConfig != "" {
		args = append(args, "--mcp-config", opts.MCPConfig)
	}

	args = append(args, "--output-format", "stream-json", "--verbose")
	args = append(args, "-p", "-")

	return args
}

// runHeadless executes Claude in headless mode (-p flag, captured output)
func (c *ClaudeProvider) runHeadless(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	args := claudeHeadlessArgs(opts)

	// Build cancellation context (external context + optional timeout)
	baseCtx := opts.Context
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(baseCtx, opts.Timeout)
		defer cancel()
	} else {
		ctx = baseCtx
	}

	cmd := exec.Command("claude", args...)
	setProcessGroup(cmd)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	var outputBuf strings.Builder

	// Pipe prompt through stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	// Write prompt to stdin
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, opts.Prompt)
	}()

	// cmdDone is closed when cmd.Wait() returns; lets killProcessGroup skip SIGKILL
	// if the process exits cleanly after SIGTERM.
	cmdDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killProcessGroup(cmd, 5*time.Second, cmdDone)
		case <-cmdDone:
		}
	}()

	// Stream output to console and capture
	var wg sync.WaitGroup
	accumulator := NewStreamAccumulator()
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamJSONOutput(stdout, &outputBuf, os.Stdout, accumulator, opts.ShowThinking, opts.Verbose)
	}()
	go func() {
		defer wg.Done()
		streamOutput(stderr, &outputBuf, os.Stderr)
	}()

	// Wait for command to complete
	err = cmd.Wait()
	close(cmdDone)
	// Wait for output streaming to finish before reading buffer
	wg.Wait()
	result.Output = outputBuf.String()

	// Populate stream-JSON metrics
	result.InputTokens = accumulator.InputTokens
	result.OutputTokens = accumulator.OutputTokens
	result.CacheTokens = accumulator.CacheTokens
	result.ThinkingBlocks = accumulator.ThinkingBlocks

	if err != nil {
		// Check if this was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.Error = fmt.Errorf("iteration timed out after %v", opts.Timeout)
			return result, nil
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Error = fmt.Errorf("claude exited with error: %w", err)
	}

	// Parse rate limit indicators from output
	parseRateLimit(result)

	return result, nil
}

// runInteractive executes Claude in interactive mode (terminal TUI)
func (c *ClaudeProvider) runInteractive(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	// Build command arguments
	args := []string{
		"--disable-slash-commands",
	}

	// Append system prompt if provided
	if opts.SystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.SystemPrompt)
	}

	// Set model if provided
	if opts.Model != "" {
		args = append(args, "--model", c.MapModel(opts.Model))
	}

	// Set permission mode
	flag, value := c.MapPermission(opts.Permission)
	if value != "" {
		args = append(args, flag, value)
	} else {
		args = append(args, flag)
	}

	// Pass hooks settings file if configured
	if opts.HooksSettingsFile != "" {
		args = append(args, "--settings", opts.HooksSettingsFile)
	}

	// Interactive mode: pass prompt as argument
	args = append(args, opts.Prompt)

	// Build cancellation context (external context + optional timeout)
	baseCtx := opts.Context
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(baseCtx, opts.Timeout)
		defer cancel()
	} else {
		ctx = baseCtx
	}

	cmd := exec.Command("claude", args...)
	setProcessGroup(cmd)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	// Inherit terminal for full TUI
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	cmdDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killProcessGroup(cmd, 5*time.Second, cmdDone)
		case <-cmdDone:
		}
	}()

	// Wait for command to complete
	err := cmd.Wait()
	close(cmdDone)

	if err != nil {
		// Check if this was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.Error = fmt.Errorf("session timed out after %v", opts.Timeout)
			return result, nil
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Error = fmt.Errorf("claude exited with error: %w", err)
	}

	return result, nil
}

// parseRateLimit detects rate limit errors and extracts retry-after time if available
func parseRateLimit(result *RunResult) {
	output := strings.ToLower(result.Output)

	// Check quota exhaustion first (distinct from transient rate limits)
	for _, pattern := range quotaPatterns {
		if strings.Contains(output, pattern) {
			result.QuotaExhausted = true
			result.RateLimited = true
			break
		}
	}

	// Check error message for quota patterns
	if !result.QuotaExhausted && result.Error != nil {
		errStr := strings.ToLower(result.Error.Error())
		for _, pattern := range quotaPatterns {
			if strings.Contains(errStr, pattern) {
				result.QuotaExhausted = true
				result.RateLimited = true
				break
			}
		}
	}

	// Common transient rate limit patterns from Claude API
	rateLimitPatterns := []string{
		"rate limit",
		"rate_limit",
		"too many requests",
		"429",
		"overloaded",
		"capacity",
		"try again",
		"throttl",
	}

	if !result.RateLimited {
		for _, pattern := range rateLimitPatterns {
			if strings.Contains(output, pattern) {
				result.RateLimited = true
				break
			}
		}
	}

	// Also check error message if present
	if !result.RateLimited && result.Error != nil {
		errStr := strings.ToLower(result.Error.Error())
		for _, pattern := range rateLimitPatterns {
			if strings.Contains(errStr, pattern) {
				result.RateLimited = true
				break
			}
		}
	}

	// Extract retry-after time if specified
	if result.RateLimited {
		result.RetryAfter = parseRetryAfter(result.Output)
	}

	// Extract quota reset time if quota was hit
	if result.QuotaExhausted {
		result.QuotaResetsAt = parseQuotaResetTime(result.Output)
	}

	// Check for 529 overload exhaustion
	parseOverloadExhausted(result)
}

// parseQuotaResetTime extracts the quota window reset time from error messages.
// Returns zero time if no reset time can be determined.
func parseQuotaResetTime(output string) time.Time {
	lower := strings.ToLower(output)

	// Relative patterns: "resets in X hours/minutes"
	relPhrases := []string{"resets in", "reset in", "available in", "try again in"}
	for _, phrase := range relPhrases {
		idx := strings.Index(lower, phrase)
		if idx < 0 {
			continue
		}
		rest := lower[idx+len(phrase):]
		rest = strings.TrimSpace(rest)

		var num int
		if n, _ := fmt.Sscanf(rest, "%d", &num); n == 1 && num > 0 {
			if strings.Contains(rest, "hour") {
				return time.Now().Add(time.Duration(num) * time.Hour)
			}
			if strings.Contains(rest, "minute") {
				return time.Now().Add(time.Duration(num) * time.Minute)
			}
		}
	}

	// Absolute patterns: "resets at HH:MM", "try again at HH:MM", "available at HH:MM"
	absPhrases := []string{"resets at", "reset at", "try again at", "available at"}
	for _, phrase := range absPhrases {
		idx := strings.Index(lower, phrase)
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(lower[idx+len(phrase):])

		// Parse HH:MM or HH:MM:SS optionally followed by timezone
		var h, m, s int
		n, _ := fmt.Sscanf(rest, "%d:%d:%d", &h, &m, &s)
		if n < 2 {
			n, _ = fmt.Sscanf(rest, "%d:%d", &h, &m)
			s = 0
		}
		if n >= 2 && h >= 0 && h < 24 && m >= 0 && m < 60 {
			now := time.Now().UTC()
			candidate := time.Date(now.Year(), now.Month(), now.Day(), h, m, s, 0, time.UTC)
			if candidate.Before(now) {
				// Time already passed today; use tomorrow
				candidate = candidate.Add(24 * time.Hour)
			}
			return candidate
		}
	}

	return time.Time{}
}

// parseOverloadExhausted detects when the agent has exited after exhausting overload retries
func parseOverloadExhausted(result *RunResult) {
	output := strings.ToLower(result.Output)

	// Patterns that indicate 529/overload exhaustion
	exhaustionPatterns := []string{
		"529",
		"overloaded_error",
		"api is overloaded",
		"exhausted.*retry",
		"maximum.*retries.*overload",
	}

	// Only flag as exhausted if the process exited with an error
	if result.Error == nil && result.ExitCode == 0 {
		return
	}

	for _, pattern := range exhaustionPatterns {
		if strings.Contains(output, pattern) {
			result.OverloadExhausted = true
			return
		}
	}

	// Also check for exit code != 0 combined with overload indicators
	if result.ExitCode != 0 && strings.Contains(output, "overloaded") {
		result.OverloadExhausted = true
	}
}

// parseRetryAfter extracts wait time from rate limit messages
func parseRetryAfter(output string) time.Duration {
	output = strings.ToLower(output)

	// Pattern: "X seconds" or "X minutes" or "X hours"
	patterns := []struct {
		unit       string
		multiplier time.Duration
	}{
		{"second", time.Second},
		{"minute", time.Minute},
		{"hour", time.Hour},
	}

	for _, p := range patterns {
		// Look for number followed by unit
		idx := strings.Index(output, p.unit)
		if idx > 0 {
			// Search backwards for a number
			numStr := ""
			for i := idx - 1; i >= 0 && i >= idx-5; i-- {
				c := output[i]
				if c >= '0' && c <= '9' {
					numStr = string(c) + numStr
				} else if len(numStr) > 0 {
					break
				}
			}
			if len(numStr) > 0 {
				var num int
				fmt.Sscanf(numStr, "%d", &num)
				if num > 0 {
					return time.Duration(num) * p.multiplier
				}
			}
		}
	}

	return 0
}
