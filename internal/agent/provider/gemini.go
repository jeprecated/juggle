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

// GeminiProvider implements Provider for Google Gemini CLI
type GeminiProvider struct{}

// NewGeminiProvider creates a new Gemini CLI provider
func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{}
}

// Type returns TypeGemini
func (g *GeminiProvider) Type() Type {
	return TypeGemini
}

// MapModel converts canonical model name to Gemini format.
// Gemini uses: gemini-2.5-flash (fast), gemini-2.5-pro (powerful).
func (g *GeminiProvider) MapModel(canonical string) string {
	switch canonical {
	case "small", "haiku":
		return "gemini-2.5-flash"
	case "medium", "sonnet", "large", "opus":
		return "gemini-2.5-pro"
	default:
		return canonical
	}
}

// MapPermission converts PermissionMode to Gemini CLI flags.
// Gemini exposes approval policy directly via --approval-mode.
func (g *GeminiProvider) MapPermission(mode PermissionMode) (flag, value string) {
	switch mode {
	case PermissionBypass:
		return "--approval-mode", "yolo"
	case PermissionPlan:
		return "--approval-mode", "plan"
	case PermissionAcceptEdits:
		return "--approval-mode", "auto_edit"
	default:
		return "--approval-mode", "auto_edit"
	}
}

// Run executes Gemini CLI with the given options
func (g *GeminiProvider) Run(opts RunOptions) (*RunResult, error) {
	if opts.Mode == ModeInteractive {
		return g.runInteractive(opts)
	}
	return g.runHeadless(opts)
}

// geminiHeadlessArgs builds the CLI argument list for a headless Gemini invocation.
// Extracted for testability.
func geminiHeadlessArgs(opts RunOptions) []string {
	args := []string{}

	if opts.Model != "" {
		p := NewGeminiProvider()
		args = append(args, "--model", p.MapModel(opts.Model))
	}

	p := NewGeminiProvider()
	flag, value := p.MapPermission(opts.Permission)
	if flag != "" {
		if value != "" {
			args = append(args, flag, value)
		} else {
			args = append(args, flag)
		}
	}

	args = append(args, opts.PassthroughArgs...)

	// Prompt via -p flag (non-interactive mode)
	args = append(args, "-p", opts.Prompt)

	return args
}

func geminiHeadlessSpec(opts RunOptions) commandSpec {
	return commandSpec{
		Binary:          "gemini",
		Args:            geminiHeadlessArgs(opts),
		Prompt:          opts.Prompt,
		CommandOverride: opts.CommandOverride,
	}
}

func geminiInteractiveSpec(opts RunOptions) commandSpec {
	args := []string{}

	if opts.Model != "" {
		args = append(args, "--model", NewGeminiProvider().MapModel(opts.Model))
	}

	flag, value := NewGeminiProvider().MapPermission(opts.Permission)
	args = appendFlag(args, flag, value)

	args = append(args, opts.PassthroughArgs...)

	if opts.Prompt != "" {
		args = append(args, opts.Prompt)
	}

	return commandSpec{
		Binary:          "gemini",
		Args:            args,
		Prompt:          opts.Prompt,
		CommandOverride: opts.CommandOverride,
	}
}

// runHeadless executes Gemini in headless mode (-p flag, captured output)
func (g *GeminiProvider) runHeadless(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	if opts.HooksSettingsFile != "" {
		fmt.Fprintf(os.Stderr, "warning: --settings (hooks) is not supported by the gemini provider and will be ignored\n")
	}
	if len(opts.AllowedTools) > 0 {
		fmt.Fprintf(os.Stderr, "warning: --allowed-tools is not supported by the gemini provider and will be ignored\n")
	}
	if len(opts.DisallowedTools) > 0 {
		fmt.Fprintf(os.Stderr, "warning: --disallowed-tools is not supported by the gemini provider and will be ignored\n")
	}
	if opts.MaxTurns > 0 {
		fmt.Fprintf(os.Stderr, "warning: --max-turns is not supported by the gemini provider and will be ignored\n")
	}
	if opts.MCPConfig != "" {
		fmt.Fprintf(os.Stderr, "warning: --mcp-config is not supported by the gemini provider and will be ignored\n")
	}

	spec := geminiHeadlessSpec(opts)

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
		return nil, fmt.Errorf("failed to start gemini: %w", err)
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
		result.Error = fmt.Errorf("gemini exited with error: %w", err)
	}

	g.parseRateLimit(result)

	return result, nil
}

// runInteractive executes Gemini in interactive mode (terminal TUI)
func (g *GeminiProvider) runInteractive(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	spec := geminiInteractiveSpec(opts)

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
		return nil, fmt.Errorf("failed to start gemini: %w", err)
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
		result.Error = fmt.Errorf("gemini exited with error: %w", err)
	}

	return result, nil
}

// parseRateLimit detects rate limit errors with Gemini/Google-specific patterns
func (g *GeminiProvider) parseRateLimit(result *RunResult) {
	output := strings.ToLower(result.Output)

	googleQuotaPatterns := []string{
		"quota exceeded",
		"daily limit",
		"daily quota",
		"resource_exhausted",
		"billing limit",
		"free tier limit",
	}

	for _, pattern := range quotaPatterns {
		if strings.Contains(output, pattern) {
			result.QuotaExhausted = true
			result.RateLimited = true
			break
		}
	}
	if !result.QuotaExhausted {
		for _, pattern := range googleQuotaPatterns {
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
