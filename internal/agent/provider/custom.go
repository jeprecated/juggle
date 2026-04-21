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

// CustomProvider implements Provider for a user-defined agent CLI.
// The command is specified as a template string with optional variables:
//
//	{prompt}       — replaced with the prompt text
//	{prompt_file}  — replaced with the path to a temp file containing the prompt
//	{model}        — replaced with the model name
//	{timeout}      — replaced with the timeout in whole seconds (0 if none)
//	{workdir}      — replaced with the working directory
type CustomProvider struct {
	agentCmd string
}

// NewCustomProvider creates a new CustomProvider with the given command template.
func NewCustomProvider(agentCmd string) *CustomProvider {
	return &CustomProvider{agentCmd: agentCmd}
}

// Type returns TypeCustom.
func (c *CustomProvider) Type() Type {
	return TypeCustom
}

// MapModel passes the model name through unchanged.
// The user is responsible for model names in their --agent-cmd template.
func (c *CustomProvider) MapModel(canonical string) string {
	return canonical
}

// MapPermission returns empty strings; the user handles permissions in their template.
func (c *CustomProvider) MapPermission(_ PermissionMode) (flag, value string) {
	return "", ""
}

// Run executes the custom agent command.
func (c *CustomProvider) Run(opts RunOptions) (*RunResult, error) {
	if opts.Mode == ModeInteractive {
		return c.runInteractive(opts)
	}
	return c.runHeadless(opts)
}

// buildCustomCmd splits the template, substitutes variables, and returns
// (binary, args, cleanup, error). cleanup removes any temp files created.
func buildCustomCmd(template string, opts RunOptions) (binary string, args []string, cleanup func(), err error) {
	tokens := strings.Fields(template)
	if len(tokens) == 0 {
		return "", nil, nil, fmt.Errorf("agent-cmd is empty")
	}

	var tmpFile *os.File
	cleanupFn := func() {
		if tmpFile != nil {
			os.Remove(tmpFile.Name())
		}
	}

	timeoutSecs := int(opts.Timeout.Seconds())

	substituted := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		switch tok {
		case "{prompt}":
			substituted = append(substituted, opts.Prompt)
		case "{prompt_file}":
			if tmpFile == nil {
				tmpFile, err = os.CreateTemp("", "juggle-prompt-*")
				if err != nil {
					cleanupFn()
					return "", nil, nil, fmt.Errorf("creating prompt temp file: %w", err)
				}
				if _, err = tmpFile.WriteString(opts.Prompt); err != nil {
					cleanupFn()
					return "", nil, nil, fmt.Errorf("writing prompt temp file: %w", err)
				}
				tmpFile.Close()
			}
			substituted = append(substituted, tmpFile.Name())
		case "{model}":
			substituted = append(substituted, opts.Model)
		case "{timeout}":
			substituted = append(substituted, fmt.Sprintf("%d", timeoutSecs))
		case "{workdir}":
			substituted = append(substituted, opts.WorkingDir)
		default:
			substituted = append(substituted, tok)
		}
	}

	return substituted[0], substituted[1:], cleanupFn, nil
}

// runHeadless executes the custom command with captured stdout/stderr.
func (c *CustomProvider) runHeadless(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	binary, args, cleanup, err := buildCustomCmd(c.agentCmd, opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return nil, err
	}

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

	cmd := exec.Command(binary, args...)
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
		return nil, fmt.Errorf("failed to start custom agent: %w", err)
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
		result.Error = fmt.Errorf("custom agent exited with error: %w", err)
	}

	parseRateLimit(result)

	return result, nil
}

// runInteractive executes the custom command with inherited stdin/stdout/stderr.
func (c *CustomProvider) runInteractive(opts RunOptions) (*RunResult, error) {
	result := &RunResult{}

	binary, args, cleanup, err := buildCustomCmd(c.agentCmd, opts)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return nil, err
	}

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

	cmd := exec.Command(binary, args...)
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
		return nil, fmt.Errorf("failed to start custom agent: %w", err)
	}

	cmdDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killProcessGroup(cmd, 5*time.Second, cmdDone)
		case <-cmdDone:
		}
	}()

	err = cmd.Wait()
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
		result.Error = fmt.Errorf("custom agent exited with error: %w", err)
	}

	return result, nil
}
