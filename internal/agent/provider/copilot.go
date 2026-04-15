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

// CopilotProvider implements Provider for GitHub Copilot CLI
type CopilotProvider struct{}

// NewCopilotProvider creates a new GitHub Copilot CLI provider
func NewCopilotProvider() *CopilotProvider {
	return &CopilotProvider{}
}

// Type returns TypeCopilot
func (c *CopilotProvider) Type() Type {
	return TypeCopilot
}

// MapModel converts canonical model name to Copilot format.
// Copilot supports claude, gpt, and gemini model families.
func (c *CopilotProvider) MapModel(canonical string) string {
	switch canonical {
	case "small", "haiku":
		return "claude-haiku-4-5"
	case "medium", "sonnet":
		return "claude-sonnet-4-5"
	case "large", "opus":
		return "claude-opus-4-5"
	default:
		return canonical
	}
}

// MapPermission converts PermissionMode to Copilot CLI flags.
// bypassPermissions → --yolo, acceptEdits → --allow-all-tools, plan → no autonomous flags.
func (c *CopilotProvider) MapPermission(mode PermissionMode) (flag, value string) {
	switch mode {
	case PermissionBypass:
		return "--yolo", ""
	case PermissionAcceptEdits:
		return "--allow-all-tools", ""
	default:
		return "", ""
	}
}

// Run executes Copilot CLI with the given options
func (c *CopilotProvider) Run(opts RunOptions) (*RunResult, error) {
	if opts.Mode == ModeInteractive {
		return c.runInteractive(opts)
	}
	return c.runHeadless(opts)
}

// copilotHeadlessArgs builds the CLI argument list for a headless Copilot invocation.
// Extracted for testability.
func copilotHeadlessArgs(opts RunOptions) []string {
	args := []string{"--autopilot"}

	p := NewCopilotProvider()
	flag, _ := p.MapPermission(opts.Permission)
	if flag != "" {
		args = append(args, flag)
	}

	// Silent mode for captured output
	args = append(args, "-s")

	if opts.Model != "" {
		args = append(args, fmt.Sprintf("--model=%s", p.MapModel(opts.Model)))
	}

	if opts.MaxTurns > 0 {
		args = append(args, "--max-autopilot-continues", fmt.Sprintf("%d", opts.MaxTurns))
	}

	for _, tool := range opts.AllowedTools {
		args = append(args, "--allow-tool", tool)
	}

	for _, tool := range opts.DisallowedTools {
		args = append(args, "--deny-tool", tool)
	}

	args = append(args, opts.PassthroughArgs...)

	// Prompt via -p flag
	args = append(args, "-p", opts.Prompt)

	return args
}

func copilotHeadlessSpec(opts RunOptions) commandSpec {
	return commandSpec{
		Binary:          "copilot",
		Args:            copilotHeadlessArgs(opts),
		Prompt:          opts.Prompt,
		CommandOverride: opts.CommandOverride,
	}
}

func copilotInteractiveSpec(opts RunOptions) commandSpec {
	args := []string{}

	if opts.Model != "" {
		args = append(args, fmt.Sprintf("--model=%s", NewCopilotProvider().MapModel(opts.Model)))
	}

	flag, value := NewCopilotProvider().MapPermission(opts.Permission)
	args = appendFlag(args, flag, value)

	args = append(args, opts.PassthroughArgs...)

	if opts.Prompt != "" {
		args = append(args, "-p", opts.Prompt)
	}

	return commandSpec{
		Binary:          "copilot",
		Args:            args,
		Prompt:          opts.Prompt,
		CommandOverride: opts.CommandOverride,
	}
}

// runHeadless executes Copilot in headless mode (-p flag, captured output)
func (c *CopilotProvider) runHeadless(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	if opts.HooksSettingsFile != "" {
		fmt.Fprintf(os.Stderr, "warning: --settings (hooks) is not supported by the copilot provider and will be ignored\n")
	}
	if opts.MCPConfig != "" {
		fmt.Fprintf(os.Stderr, "warning: --mcp-config is not supported by the copilot provider and will be ignored\n")
	}

	spec := copilotHeadlessSpec(opts)

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

	cmd := commandForSpec(spec)
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

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start copilot: %w", err)
	}

	cmdDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killProcessGroup(cmd, 5*time.Second, cmdDone)
		case <-cmdDone:
		}
	}()

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

	wg.Wait()
	err = cmd.Wait()
	close(cmdDone)
	result.Output = outputBuf.String()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.Error = fmt.Errorf("iteration timed out after %v", opts.Timeout)
			return result, nil
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Error = fmt.Errorf("copilot exited with error: %w", err)
	}

	c.parseRateLimit(result)

	return result, nil
}

// runInteractive executes Copilot in interactive mode (terminal TUI)
func (c *CopilotProvider) runInteractive(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	spec := copilotInteractiveSpec(opts)

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

	cmd := commandForSpec(spec)
	setProcessGroup(cmd)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start copilot: %w", err)
	}

	cmdDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killProcessGroup(cmd, 5*time.Second, cmdDone)
		case <-cmdDone:
		}
	}()

	err := cmd.Wait()
	close(cmdDone)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.TimedOut = true
			result.Error = fmt.Errorf("session timed out after %v", opts.Timeout)
			return result, nil
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Error = fmt.Errorf("copilot exited with error: %w", err)
	}

	return result, nil
}

// parseRateLimit detects rate limit errors with Copilot/GitHub-specific patterns
func (c *CopilotProvider) parseRateLimit(result *RunResult) {
	output := strings.ToLower(result.Output)

	for _, pattern := range quotaPatterns {
		if strings.Contains(output, pattern) {
			result.QuotaExhausted = true
			result.RateLimited = true
			break
		}
	}

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

	rateLimitPatterns := []string{
		"rate limit",
		"rate_limit",
		"too many requests",
		"429",
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

	if !result.RateLimited && result.Error != nil {
		errStr := strings.ToLower(result.Error.Error())
		for _, pattern := range rateLimitPatterns {
			if strings.Contains(errStr, pattern) {
				result.RateLimited = true
				break
			}
		}
	}

	if result.RateLimited {
		result.RetryAfter = parseRetryAfter(result.Output)
	}

	if result.QuotaExhausted {
		result.QuotaResetsAt = parseQuotaResetTime(result.Output)
	}
}
