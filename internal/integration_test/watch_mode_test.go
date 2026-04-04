package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/cli"
)

func TestWatchMode_ProcessesTaskFile(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "watch-aabbcc"
	env.CreateSession(t, sessionID, "watch mode test")
	sessionStore := env.GetSessionStore(t)

	taskDir := filepath.Join(env.ProjectDir, "queue", "ready")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("Failed to create task dir: %v", err)
	}
	taskFile := filepath.Join(taskDir, "01-test-task.md")
	if err := os.WriteFile(taskFile, []byte("# Test Task\nDo the thing."), 0644); err != nil {
		t.Fatalf("Failed to write task file: %v", err)
	}

	mock := agent.NewMockRunner(
		&agent.RunResult{
			Output:   "Done.\n<promise>COMPLETE</promise>",
			Complete: true,
		},
	)
	agent.SetRunner(&progressUpdatingMockRunner{
		mock:         mock,
		sessionStore: sessionStore,
		sessionID:    sessionID,
	})
	defer agent.ResetRunner()

	// Test the sub-loop directly (not runWatchLoop which loops forever)
	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 5,
		IterDelay:     0,
		Manual:        true,
		WatchTaskFile: taskFile,
		Contexts:      []string{"worker instructions here"},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected result.Complete=true")
	}

	prompt := mock.Calls[0].Prompt
	if !strings.Contains(prompt, "Test Task") {
		t.Error("Expected prompt to contain task file contents")
	}
	if !strings.Contains(prompt, "<task>") {
		t.Error("Expected prompt to contain <task> tag")
	}
	if !strings.Contains(prompt, "worker instructions here") {
		t.Error("Expected prompt to contain context")
	}
}

func TestWatchMode_ReReadsTaskFile(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "watch-112233"
	env.CreateSession(t, sessionID, "watch re-read test")
	sessionStore := env.GetSessionStore(t)

	taskDir := filepath.Join(env.ProjectDir, "queue")
	os.MkdirAll(taskDir, 0755)
	taskFile := filepath.Join(taskDir, "task.md")
	os.WriteFile(taskFile, []byte("# Task\nOriginal content"), 0644)

	callCount := 0
	customRunner := &funcRunner{
		fn: func(opts agent.RunOptions) (*agent.RunResult, error) {
			callCount++
			_ = sessionStore.AppendProgress(sessionID, "[progress]\n")

			if callCount == 1 {
				// Simulate agent appending progress to task file
				f, _ := os.OpenFile(taskFile, os.O_APPEND|os.O_WRONLY, 0644)
				f.WriteString("\n## Progress\n- Did step 1\n")
				f.Close()

				return &agent.RunResult{
					Output:   "<promise>CONTINUE</promise>",
					Continue: true,
				}, nil
			}
			return &agent.RunResult{
				Output:   "<promise>COMPLETE</promise>",
				Complete: true,
			}, nil
		},
	}
	agent.SetRunner(customRunner)
	defer agent.ResetRunner()

	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 5,
		IterDelay:     0,
		Manual:        true,
		WatchTaskFile: taskFile,
		Contexts:      []string{},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected complete")
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

// funcRunner is a simple Runner implementation that calls a function.
type funcRunner struct {
	fn func(opts agent.RunOptions) (*agent.RunResult, error)
}

func (f *funcRunner) Run(opts agent.RunOptions) (*agent.RunResult, error) {
	return f.fn(opts)
}
