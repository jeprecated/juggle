package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/pipeline"
)

func TestPipelineSubcommandIsRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pipeline"})
	if err != nil || cmd == nil {
		t.Fatal("pipeline subcommand not registered on rootCmd")
	}
	if cmd.Name() != "pipeline" {
		t.Errorf("expected command name %q, got %q", "pipeline", cmd.Name())
	}
}

func TestPipelineHelpContainsPipelineFocusedText(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pipeline"})
	if err != nil || cmd == nil {
		t.Skip("pipeline subcommand not found")
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	defer cmd.SetOut(os.Stdout)
	if err := cmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"pipeline", "agent", "cmd"} {
		if !strings.Contains(out, want) {
			t.Errorf("pipeline help output missing %q", want)
		}
	}
}

func TestPipelineFileAndInlineArgsMutuallyExclusive(t *testing.T) {
	err := runPipelineCmd(nil, []string{"agent", "foo", "do stuff"})
	// no --file set, this will attempt to parse inline args (may fail on validation, but not on the mutual exclusion check)
	// Now test with file set:
	old := pipelineFile
	pipelineFile = "some.toml"
	defer func() { pipelineFile = old }()
	err = runPipelineCmd(nil, []string{"agent", "foo", "do stuff"})
	if err == nil {
		t.Fatal("expected error when both --file and inline args provided")
	}
	if !strings.Contains(err.Error(), "cannot use --file with inline") {
		t.Errorf("expected mutual exclusion error, got: %v", err)
	}
}

func TestRootHelpListsPipelineSubcommand(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(os.Stdout)
	if err := rootCmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	if !strings.Contains(buf.String(), "pipeline") {
		t.Error("root help should list pipeline subcommand")
	}
}

func TestPipelineExecution_Success(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{Output: "done"})
	pipelineTestRunner = mock
	defer func() { pipelineTestRunner = nil }()

	err := runPipelineCmd(nil, []string{"agent", "Main", "do work"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(mock.Calls) != 1 {
		t.Errorf("expected 1 runner call, got %d", len(mock.Calls))
	}
}

func TestPipelineExecution_ExecutorFailure(t *testing.T) {
	mock := agent.NewMockRunner(&agent.RunResult{ExitCode: 1})
	pipelineTestRunner = mock
	defer func() { pipelineTestRunner = nil }()

	err := runPipelineCmd(nil, []string{"agent", "Main", "do work"})
	if err == nil {
		t.Fatal("expected error from non-zero exit code")
	}
}

func TestPipelineExecution_ShutdownInterrupts(t *testing.T) {
	shutdown := make(chan struct{})
	close(shutdown)

	p := &pipeline.Pipeline{
		Iterations: 5,
		Nodes: []pipeline.Node{
			{Name: "Main", Kind: pipeline.NodeKindAgent, Agent: &pipeline.AgentSpec{Prompt: "do work"}},
		},
	}
	if err := pipeline.Normalize(p); err != nil {
		t.Fatalf("normalize: %v", err)
	}

	mock := agent.NewMockRunner(&agent.RunResult{Output: "done"})
	err := executePipeline(p, pipeline.ExecutorConfig{
		Runner:   mock,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		Shutdown: shutdown,
		ForceCtx: context.Background(),
	})
	if err == nil {
		t.Fatal("expected error when shutdown channel is pre-closed")
	}
	if len(mock.Calls) != 0 {
		t.Errorf("expected 0 runner calls with pre-closed shutdown, got %d", len(mock.Calls))
	}
}
