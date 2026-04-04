package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// OpenCodeProvider implements Provider for OpenCode CLI
type OpenCodeProvider struct{}

// NewOpenCodeProvider creates a new OpenCode provider
func NewOpenCodeProvider() *OpenCodeProvider {
	return &OpenCodeProvider{}
}

// Type returns TypeOpenCode
func (o *OpenCodeProvider) Type() Type {
	return TypeOpenCode
}

// MapModel converts canonical model name to OpenCode format
// OpenCode uses: provider/model_id format (e.g., "anthropic/claude-opus-4-5")
func (o *OpenCodeProvider) MapModel(canonical string) string {
	switch canonical {
	case "haiku", "small":
		return "anthropic/claude-3-5-haiku-latest"
	case "sonnet", "medium":
		return "anthropic/claude-sonnet-4-5"
	case "opus", "large":
		return "anthropic/claude-opus-4-5"
	default:
		// Assume it's already in provider/model format or pass through
		return canonical
	}
}

// MapPermission converts PermissionMode to OpenCode's --agent flag
// OpenCode uses "agents" instead of permission modes:
// - build = full access (like acceptEdits/bypassPermissions)
// - plan = read-only (like plan mode)
func (o *OpenCodeProvider) MapPermission(mode PermissionMode) (flag, value string) {
	switch mode {
	case PermissionPlan:
		return "--agent", "plan"
	case PermissionAcceptEdits, PermissionBypass:
		return "--agent", "build"
	default:
		return "--agent", "build"
	}
}

// Run executes OpenCode CLI with the given options
func (o *OpenCodeProvider) Run(opts RunOptions) (*RunResult, error) {
	if opts.Mode == ModeInteractive {
		return o.runInteractive(opts)
	}
	return o.runHeadless(opts)
}

// runHeadless executes OpenCode in headless mode (opencode run "prompt")
func (o *OpenCodeProvider) runHeadless(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	// OpenCode uses: opencode run "prompt"
	args := []string{"run"}

	// Set model if provided
	if opts.Model != "" {
		args = append(args, "--model", o.MapModel(opts.Model))
	}

	// Set agent (permission mode equivalent)
	flag, value := o.MapPermission(opts.Permission)
	args = append(args, flag, value)

	// OpenCode takes prompt as argument, not stdin
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

	cmd := exec.Command("opencode", args...)
	setProcessGroup(cmd)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	var outputBuf strings.Builder

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
		return nil, fmt.Errorf("failed to start opencode: %w", err)
	}

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
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamOutput(stdout, &outputBuf, os.Stdout)
	}()
	go func() {
		defer wg.Done()
		streamOutput(stderr, &outputBuf, os.Stderr)
	}()

	// Wait for command to complete
	err = cmd.Wait()
	close(cmdDone)
	wg.Wait()
	result.Output = outputBuf.String()

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
		result.Error = fmt.Errorf("opencode exited with error: %w", err)
	}

	// Parse rate limits with OpenCode-specific patterns
	o.parseRateLimit(result)

	return result, nil
}

// runInteractive executes OpenCode in interactive mode (terminal TUI)
func (o *OpenCodeProvider) runInteractive(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	// OpenCode interactive mode - no "run" subcommand
	args := []string{}

	// Set model if provided
	if opts.Model != "" {
		args = append(args, "--model", o.MapModel(opts.Model))
	}

	// Set agent (permission mode equivalent)
	flag, value := o.MapPermission(opts.Permission)
	args = append(args, flag, value)

	// Pass prompt via --prompt flag
	if opts.Prompt != "" {
		args = append(args, "--prompt", opts.Prompt)
	}

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

	cmd := exec.Command("opencode", args...)
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
		return nil, fmt.Errorf("failed to start opencode: %w", err)
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
		result.Error = fmt.Errorf("opencode exited with error: %w", err)
	}

	return result, nil
}

// parseRateLimit detects rate limit errors with OpenCode/OpenAI-specific patterns
func (o *OpenCodeProvider) parseRateLimit(result *RunResult) {
	output := strings.ToLower(result.Output)

	// OpenAI-specific quota patterns (in addition to shared claude quotaPatterns)
	openaiQuotaPatterns := []string{
		"exceeded your quota",
		"insufficient_quota",
		"quota_exceeded",
		"usage limit",
		"usage quota",
		"daily limit",
		"daily quota",
		"monthly limit",
	}

	// Check quota exhaustion first (distinct from transient rate limits)
	for _, pattern := range quotaPatterns {
		if strings.Contains(output, pattern) {
			result.QuotaExhausted = true
			result.RateLimited = true
			break
		}
	}
	if !result.QuotaExhausted {
		for _, pattern := range openaiQuotaPatterns {
			if strings.Contains(output, pattern) {
				result.QuotaExhausted = true
				result.RateLimited = true
				break
			}
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

	// Rate limit patterns - includes both Anthropic and OpenAI patterns
	// since OpenCode supports multiple providers
	rateLimitPatterns := []string{
		"rate limit",
		"rate_limit",
		"too many requests",
		"429",
		"overloaded",
		"capacity",
		"try again",
		"throttl",
		"tpm limit", // Tokens per minute
		"rpm limit", // Requests per minute
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

	// Check for overload exhaustion
	o.parseOverloadExhausted(result)
}

// parseOverloadExhausted detects when the agent has exited after exhausting retries
func (o *OpenCodeProvider) parseOverloadExhausted(result *RunResult) {
	output := strings.ToLower(result.Output)

	exhaustionPatterns := []string{
		"529",
		"overloaded_error",
		"api is overloaded",
		"exhausted.*retry",
		"maximum.*retries",
		"quota exceeded",
	}

	if result.Error == nil && result.ExitCode == 0 {
		return
	}

	for _, pattern := range exhaustionPatterns {
		if strings.Contains(output, pattern) {
			result.OverloadExhausted = true
			return
		}
	}

	if result.ExitCode != 0 && strings.Contains(output, "overloaded") {
		result.OverloadExhausted = true
	}
}
