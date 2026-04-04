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

// runHeadless executes Claude in headless mode (-p flag, captured output)
func (c *ClaudeProvider) runHeadless(opts RunOptions) (*RunResult, error) {
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

	// Always use stream-json output format (requires --verbose with -p stdin mode)
	args = append(args, "--output-format", "stream-json", "--verbose")

	// Headless mode: read prompt from stdin
	args = append(args, "-p", "-")

	// Create context with timeout if specified
	var ctx context.Context
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
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

	// Stream output to console and capture
	var wg sync.WaitGroup
	accumulator := NewStreamAccumulator()
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamJSONOutput(stdout, &outputBuf, os.Stdout, accumulator, opts.ShowThinking)
	}()
	go func() {
		defer wg.Done()
		streamOutput(stderr, &outputBuf, os.Stderr)
	}()

	// Wait for command to complete
	err = cmd.Wait()
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

	// Interactive mode: pass prompt as argument
	args = append(args, opts.Prompt)

	// Create context with timeout if specified
	var ctx context.Context
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	// Inherit terminal for full TUI
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	// Wait for command to complete
	err := cmd.Wait()

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

	// Common rate limit patterns from Claude API
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

	for _, pattern := range rateLimitPatterns {
		if strings.Contains(output, pattern) {
			result.RateLimited = true
			break
		}
	}

	// Also check error message if present
	if result.Error != nil {
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

	// Check for 529 overload exhaustion
	parseOverloadExhausted(result)
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
