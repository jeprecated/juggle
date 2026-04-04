package integration_test

import (
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/cli"
)

func TestManualMode_CompleteSignal(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "manual-aabbcc"
	env.CreateSession(t, sessionID, "manual mode test")
	sessionStore := env.GetSessionStore(t)

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

	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 5,
		IterDelay:     0,
		Manual:        true,
		Contexts:      []string{"do the thing"},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected result.Complete=true")
	}
	if mock.NextIndex != 1 {
		t.Errorf("Expected 1 call to runner, got %d", mock.NextIndex)
	}

	prompt := mock.Calls[0].Prompt
	if !strings.Contains(prompt, "do the thing") {
		t.Error("Expected prompt to contain context text")
	}
	if strings.Contains(prompt, "<balls>") {
		t.Error("Expected prompt to NOT contain <balls> tag in manual mode")
	}
}

func TestManualMode_ContinueThenComplete(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "manual-112233"
	env.CreateSession(t, sessionID, "manual mode continue test")
	sessionStore := env.GetSessionStore(t)

	mock := agent.NewMockRunner(
		&agent.RunResult{
			Output:   "Working...\n<promise>CONTINUE</promise>",
			Continue: true,
		},
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

	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 5,
		IterDelay:     0,
		Manual:        true,
		Contexts:      []string{"build a feature"},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected result.Complete=true")
	}
	if mock.NextIndex != 2 {
		t.Errorf("Expected 2 calls to runner, got %d", mock.NextIndex)
	}
}

func TestManualMode_NoContexts(t *testing.T) {
	skipIfNoClaudeCLI(t)
	env := SetupTestEnv(t)
	defer CleanupTestEnv(t, env)

	sessionID := "manual-000000"
	env.CreateSession(t, sessionID, "bare manual mode")
	sessionStore := env.GetSessionStore(t)

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

	config := cli.AgentLoopConfig{
		SessionID:     sessionID,
		ProjectDir:    env.ProjectDir,
		MaxIterations: 1,
		IterDelay:     0,
		Manual:        true,
		Contexts:      []string{},
	}

	result, err := cli.RunAgentLoop(config)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	if !result.Complete {
		t.Error("Expected result.Complete=true for bare manual mode")
	}

	prompt := mock.Calls[0].Prompt
	if !strings.Contains(prompt, "<instructions>") {
		t.Error("Expected prompt to contain <instructions>")
	}
}
