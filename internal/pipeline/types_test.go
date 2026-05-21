package pipeline_test

import (
	"testing"

	"github.com/jeprecated/juggle/internal/pipeline"
)

// Event.Valid()

func TestEventValid_knownEventsAreValid(t *testing.T) {
	known := []pipeline.Event{
		pipeline.EventRunStart,
		pipeline.EventLoopStart,
		pipeline.EventLoopBody,
		pipeline.EventLoopEnd,
		pipeline.EventRunEnd,
		pipeline.EventFailure,
	}
	for _, e := range known {
		if !e.Valid() {
			t.Errorf("expected event %q to be valid", e)
		}
	}
}

func TestEventValid_unknownEventIsInvalid(t *testing.T) {
	e := pipeline.Event("unknown-event")
	if e.Valid() {
		t.Errorf("expected event %q to be invalid", e)
	}
}

// NodeKind.Valid()

func TestNodeKindValid_agentAndCmdAreValid(t *testing.T) {
	for _, k := range []pipeline.NodeKind{pipeline.NodeKindAgent, pipeline.NodeKindCmd} {
		if !k.Valid() {
			t.Errorf("expected node kind %q to be valid", k)
		}
	}
}

func TestNodeKindValid_unknownKindIsInvalid(t *testing.T) {
	k := pipeline.NodeKind("task")
	if k.Valid() {
		t.Errorf("expected node kind %q to be invalid", k)
	}
}

// FailurePolicy.Valid()

func TestFailurePolicyValid_knownPoliciesAreValid(t *testing.T) {
	known := []pipeline.FailurePolicy{
		pipeline.FailurePolicyStop,
		pipeline.FailurePolicyContinue,
		pipeline.FailurePolicyRetry,
	}
	for _, p := range known {
		if !p.Valid() {
			t.Errorf("expected failure policy %q to be valid", p)
		}
	}
}

func TestFailurePolicyValid_unknownPolicyIsInvalid(t *testing.T) {
	p := pipeline.FailurePolicy("ignore")
	if p.Valid() {
		t.Errorf("expected failure policy %q to be invalid", p)
	}
}

// Node.EffectiveFailurePolicy()

func TestNodeEffectiveFailurePolicy_emptyDefaultsToStop(t *testing.T) {
	n := pipeline.Node{}
	if got := n.EffectiveFailurePolicy(); got != pipeline.FailurePolicyStop {
		t.Errorf("expected default failure policy %q, got %q", pipeline.FailurePolicyStop, got)
	}
}

func TestNodeEffectiveFailurePolicy_explicitPolicyIsPreserved(t *testing.T) {
	n := pipeline.Node{OnFailure: pipeline.FailurePolicyContinue}
	if got := n.EffectiveFailurePolicy(); got != pipeline.FailurePolicyContinue {
		t.Errorf("expected failure policy %q, got %q", pipeline.FailurePolicyContinue, got)
	}
}

// Struct fields — compile-time shape checks

func TestAgentSpecFields(t *testing.T) {
	_ = pipeline.AgentSpec{
		Prompt:          "do something",
		Provider:        "claude",
		Model:           "sonnet",
		Plan:            false,
		Trust:           true,
		SystemPrompt:    "@system.md",
		AllowedTools:    []string{"Read"},
		DisallowedTools: []string{"Bash"},
		MaxTurns:        10,
		MCPConfig:       "mcp.json",
		Passthrough:     []string{"--flag"},
	}
}

func TestCmdSpecFields(t *testing.T) {
	_ = pipeline.CmdSpec{
		Command: "echo hello",
		Shell:   "bash",
		Env:     []string{"KEY=value"},
	}
}

func TestNodeFields(t *testing.T) {
	_ = pipeline.Node{
		Name:      "Setup",
		Kind:      pipeline.NodeKindAgent,
		Event:     pipeline.EventRunStart,
		After:     []string{"other"},
		Parallel:  false,
		When:      "iteration==1",
		OnFailure: pipeline.FailurePolicyStop,
		Retries:   3,
		WorkDir:   "/tmp",
		Agent:     &pipeline.AgentSpec{Prompt: "go"},
	}
}

func TestPipelineFields(t *testing.T) {
	_ = pipeline.Pipeline{
		Iterations:       5,
		MaxParallelSteps: 2,
		Defaults: pipeline.Defaults{
			Provider: "claude",
			Model:    "sonnet",
		},
		Nodes: []pipeline.Node{},
	}
}
