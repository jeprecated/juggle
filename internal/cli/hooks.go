package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// hookEnv holds iteration and run-level metadata passed as environment variables to hook commands.
type hookEnv struct {
	iteration     int
	maxIterations int
	exitCode      int
	inputTokens   int
	outputTokens  int
	runID         string
	label         string
	model         string
	provider      string
}

// envSlice returns the hook environment as a slice of KEY=VALUE strings.
func (h hookEnv) envSlice() []string {
	env := []string{
		"JUGGLE_ITERATION=" + strconv.Itoa(h.iteration),
		"JUGGLE_MAX_ITERATIONS=" + strconv.Itoa(h.maxIterations),
		"JUGGLE_EXIT_CODE=" + strconv.Itoa(h.exitCode),
		"JUGGLE_INPUT_TOKENS=" + strconv.Itoa(h.inputTokens),
		"JUGGLE_OUTPUT_TOKENS=" + strconv.Itoa(h.outputTokens),
		"JUGGLE_RUN_ID=" + h.runID,
		"JUGGLE_MODEL=" + h.model,
		"JUGGLE_PROVIDER=" + h.provider,
	}
	if h.label != "" {
		env = append(env, "JUGGLE_LABEL="+h.label)
	}
	return env
}

// runHook executes a hook command (inline shell or @file reference).
// Empty cmd is a no-op. Returns an error if the command exits non-zero.
// Hook output (stdout+stderr) is forwarded to w.
func runHook(cmd string, env hookEnv, w io.Writer) error {
	if cmd == "" {
		return nil
	}

	var c *exec.Cmd
	if strings.HasPrefix(cmd, "@") {
		path, err := resolveHookPath(cmd[1:])
		if err != nil {
			return fmt.Errorf("resolving hook file: %w", err)
		}
		c = exec.Command(path)
	} else {
		c = exec.Command("sh", "-c", cmd)
	}

	c.Env = append(os.Environ(), env.envSlice()...)
	c.Stdout = w
	c.Stderr = w

	return c.Run()
}

// resolveHookPath finds the filesystem path for a hook file reference,
// using the same JUGGLE_PROMPTS → cwd fallback chain as resolveFile.
func resolveHookPath(name string) (string, error) {
	if _, err := os.Stat(name); err == nil {
		return name, nil
	}

	if strings.Contains(name, "/") {
		return "", fmt.Errorf("hook file not found: %s", name)
	}

	promptsDir := os.Getenv("JUGGLE_PROMPTS")
	if promptsDir == "" {
		return "", fmt.Errorf("hook file not found: %s", name)
	}

	candidate := filepath.Join(promptsDir, name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	if filepath.Ext(name) == "" {
		candidate = filepath.Join(promptsDir, name+".md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("hook file not found: %s (also tried %s)", name, promptsDir)
}
