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

// CodexProvider implements Provider for OpenAI Codex CLI
type CodexProvider struct{}

// NewCodexProvider creates a new Codex provider
func NewCodexProvider() *CodexProvider {
	return &CodexProvider{}
}

// Type returns TypeCodex
func (c *CodexProvider) Type() Type {
	return TypeCodex
}

// MapModel converts explicit OpenAI model names for Codex.
// Generic juggle aliases do not map cleanly across Codex account types, so they
// intentionally fall back to the CLI default by returning "".
func (c *CodexProvider) MapModel(canonical string) string {
	switch canonical {
	case "small", "haiku", "medium", "sonnet":
		return ""
	case "large", "opus":
		return ""
	default:
		return canonical
	}
}

// MapPermission converts PermissionMode to Codex's --approval-mode flag.
func (c *CodexProvider) MapPermission(mode PermissionMode) (flag, value string) {
	switch mode {
	case PermissionPlan:
		return "--sandbox", "read-only"
	case PermissionBypass:
		return "--dangerously-bypass-approvals-and-sandbox", ""
	case PermissionAcceptEdits:
		return "--full-auto", ""
	default:
		return "--full-auto", ""
	}
}

// Run executes Codex CLI with the given options
func (c *CodexProvider) Run(opts RunOptions) (*RunResult, error) {
	if opts.Mode == ModeInteractive {
		return c.runInteractive(opts)
	}
	return c.runHeadless(opts)
}

// codexHeadlessArgs builds the CLI argument list for a headless Codex invocation.
// Extracted for testability.
func codexHeadlessArgs(opts RunOptions) []string {
	args := []string{"exec"}

	if mappedModel := NewCodexProvider().MapModel(opts.Model); mappedModel != "" {
		p := NewCodexProvider()
		args = append(args, "--model", p.MapModel(opts.Model))
	}

	p := NewCodexProvider()
	flag, value := p.MapPermission(opts.Permission)
	if value != "" {
		args = append(args, flag, value)
	} else if flag != "" {
		args = append(args, flag)
	}

	args = append(args, opts.PassthroughArgs...)

	// Prompt is a positional argument (last)
	args = append(args, opts.Prompt)

	return args
}

func codexHeadlessSpec(opts RunOptions) commandSpec {
	return commandSpec{
		Binary:          "codex",
		Args:            codexHeadlessArgs(opts),
		Prompt:          opts.Prompt,
		CommandOverride: opts.CommandOverride,
	}
}

func codexInteractiveSpec(opts RunOptions) commandSpec {
	args := []string{}

	if mappedModel := NewCodexProvider().MapModel(opts.Model); mappedModel != "" {
		args = append(args, "--model", mappedModel)
	}

	flag, value := NewCodexProvider().MapPermission(opts.Permission)
	args = appendFlag(args, flag, value)

	args = append(args, opts.PassthroughArgs...)

	if opts.Prompt != "" {
		args = append(args, opts.Prompt)
	}

	return commandSpec{
		Binary:          "codex",
		Args:            args,
		Prompt:          opts.Prompt,
		CommandOverride: opts.CommandOverride,
	}
}

// runHeadless executes Codex in headless mode via `codex exec`.
func (c *CodexProvider) runHeadless(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	if opts.HooksSettingsFile != "" {
		fmt.Fprintf(os.Stderr, "warning: --settings (hooks) is not supported by the codex provider and will be ignored\n")
	}
	if len(opts.AllowedTools) > 0 {
		fmt.Fprintf(os.Stderr, "warning: --allowed-tools is not supported by the codex provider and will be ignored\n")
	}
	if len(opts.DisallowedTools) > 0 {
		fmt.Fprintf(os.Stderr, "warning: --disallowed-tools is not supported by the codex provider and will be ignored\n")
	}
	if opts.MaxTurns > 0 {
		fmt.Fprintf(os.Stderr, "warning: --max-turns is not supported by the codex provider and will be ignored\n")
	}
	if opts.MCPConfig != "" {
		fmt.Fprintf(os.Stderr, "warning: --mcp-config is not supported by the codex provider and will be ignored\n")
	}

	spec := codexHeadlessSpec(opts)

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
		return nil, fmt.Errorf("failed to start codex: %w", err)
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

	err = cmd.Wait()
	close(cmdDone)
	wg.Wait()
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
		result.Error = fmt.Errorf("codex exited with error: %w", err)
	}

	c.parseRateLimit(result)

	return result, nil
}

// runInteractive executes Codex in interactive mode (terminal TUI)
func (c *CodexProvider) runInteractive(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	spec := codexInteractiveSpec(opts)

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
		return nil, fmt.Errorf("failed to start codex: %w", err)
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
		result.Error = fmt.Errorf("codex exited with error: %w", err)
	}

	return result, nil
}

// parseRateLimit detects rate limit errors with Codex/OpenAI-specific patterns
func (c *CodexProvider) parseRateLimit(result *RunResult) {
	output := strings.ToLower(result.Output)

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
		"overloaded",
		"capacity",
		"try again",
		"throttl",
		"tpm limit",
		"rpm limit",
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
