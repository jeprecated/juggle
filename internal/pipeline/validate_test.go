package pipeline_test

import (
	"testing"

	"github.com/ohare93/juggle/internal/pipeline"
)

// --- helpers ---

// agentNode returns a minimal valid agent node with event=loop-body.
func agentNode(name string) pipeline.Node {
	return pipeline.Node{
		Name:  name,
		Kind:  pipeline.NodeKindAgent,
		Event: pipeline.EventLoopBody,
		Agent: &pipeline.AgentSpec{Prompt: "do the thing"},
	}
}

// cmdNode returns a minimal valid cmd node with event=loop-end.
func cmdNode(name string) pipeline.Node {
	return pipeline.Node{
		Name:  name,
		Kind:  pipeline.NodeKindCmd,
		Event: pipeline.EventLoopEnd,
		Cmd:   &pipeline.CmdSpec{Command: "echo done"},
	}
}

// minimalPipeline returns a valid single-node pipeline.
func minimalPipeline() *pipeline.Pipeline {
	return &pipeline.Pipeline{
		Nodes: []pipeline.Node{agentNode("main")},
	}
}

// --- Implicit dependency insertion ---

func TestNormalize_singleNode_noAfterAdded(t *testing.T) {
	p := minimalPipeline()
	if err := pipeline.Normalize(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes[0].After) != 0 {
		t.Errorf("expected no After for single node; got %v", p.Nodes[0].After)
	}
}

func TestNormalize_twoNodes_secondImplicitlyDependsOnFirst(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "setup", Kind: pipeline.NodeKindCmd, Event: pipeline.EventRunStart, Cmd: &pipeline.CmdSpec{Command: "echo setup"}},
			agentNode("main"),
		},
	}
	if err := pipeline.Normalize(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Nodes[1].After
	if len(got) != 1 || got[0] != "setup" {
		t.Errorf("expected After=[setup]; got %v", got)
	}
}

func TestNormalize_parallelNode_suppressesImplicitDep(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "setup", Kind: pipeline.NodeKindCmd, Event: pipeline.EventRunStart, Cmd: &pipeline.CmdSpec{Command: "echo setup"}},
			{Name: "main", Kind: pipeline.NodeKindAgent, Event: pipeline.EventLoopBody, Parallel: true, Agent: &pipeline.AgentSpec{Prompt: "do"}},
		},
	}
	if err := pipeline.Normalize(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes[1].After) != 0 {
		t.Errorf("expected no After for parallel node; got %v", p.Nodes[1].After)
	}
}

func TestNormalize_explicitAfter_notReplaced(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "a", Kind: pipeline.NodeKindCmd, Event: pipeline.EventRunStart, Cmd: &pipeline.CmdSpec{Command: "echo a"}},
			{Name: "b", Kind: pipeline.NodeKindCmd, Event: pipeline.EventRunStart, Cmd: &pipeline.CmdSpec{Command: "echo b"}},
			{Name: "main", Kind: pipeline.NodeKindAgent, Event: pipeline.EventLoopBody, After: []string{"a"}, Agent: &pipeline.AgentSpec{Prompt: "do"}},
		},
	}
	if err := pipeline.Normalize(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := p.Nodes[2].After
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("expected After=[a] unchanged; got %v", got)
	}
}

// --- Unique node names ---

func TestNormalize_duplicateNames_returnsError(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			agentNode("main"),
			{Name: "main", Kind: pipeline.NodeKindCmd, Event: pipeline.EventLoopEnd, Cmd: &pipeline.CmdSpec{Command: "echo"}},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error for duplicate node name")
	}
}

// --- Valid kind payloads ---

func TestNormalize_agentNodeWithoutAgentSpec_returnsError(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "main", Kind: pipeline.NodeKindAgent, Event: pipeline.EventLoopBody},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error for agent node without AgentSpec")
	}
}

func TestNormalize_cmdNodeWithoutCmdSpec_returnsError(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			agentNode("main"),
			{Name: "cleanup", Kind: pipeline.NodeKindCmd, Event: pipeline.EventLoopEnd},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error for cmd node without CmdSpec")
	}
}

// --- Valid event names ---

func TestNormalize_invalidEventName_returnsError(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "main", Kind: pipeline.NodeKindAgent, Event: "not-an-event", Agent: &pipeline.AgentSpec{Prompt: "do"}},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error for invalid event name")
	}
}

// --- After target validation ---

func TestNormalize_afterTargetNotFound_returnsError(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "main", Kind: pipeline.NodeKindAgent, Event: pipeline.EventLoopBody, After: []string{"ghost"}, Agent: &pipeline.AgentSpec{Prompt: "do"}},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error for unknown After target")
	}
}

func TestNormalize_afterForwardRef_returnsError(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "a", Kind: pipeline.NodeKindAgent, Event: pipeline.EventLoopBody, After: []string{"b"}, Agent: &pipeline.AgentSpec{Prompt: "do"}},
			{Name: "b", Kind: pipeline.NodeKindCmd, Event: pipeline.EventLoopEnd, Cmd: &pipeline.CmdSpec{Command: "echo"}},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error for forward dependency")
	}
}

// --- Cycle detection ---

func TestNormalize_dependencyCycle_returnsError(t *testing.T) {
	// A depends on B (forward ref), B depends on A — forms a cycle.
	// The forward-ref check catches this; the important thing is an error is returned.
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "a", Kind: pipeline.NodeKindAgent, Event: pipeline.EventLoopBody, After: []string{"b"}, Agent: &pipeline.AgentSpec{Prompt: "do"}},
			{Name: "b", Kind: pipeline.NodeKindCmd, Event: pipeline.EventLoopEnd, After: []string{"a"}, Cmd: &pipeline.CmdSpec{Command: "echo"}},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error for dependency cycle")
	}
}

// --- v1 loop-body invariant ---

func TestNormalize_noLoopBodyAgent_returnsError(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "setup", Kind: pipeline.NodeKindCmd, Event: pipeline.EventRunStart, Cmd: &pipeline.CmdSpec{Command: "echo"}},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error when no agent node with event=loop-body")
	}
}

func TestNormalize_multipleLoopBodyAgents_returnsError(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			agentNode("first"),
			{Name: "second", Kind: pipeline.NodeKindAgent, Event: pipeline.EventLoopBody, Agent: &pipeline.AgentSpec{Prompt: "also"}},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error for multiple loop-body agents")
	}
}

func TestNormalize_exactlyOneLoopBodyAgent_ok(t *testing.T) {
	p := minimalPipeline()
	if err := pipeline.Normalize(p); err != nil {
		t.Fatalf("expected no error for valid pipeline; got %v", err)
	}
}

func TestNormalize_loopBodyCmdNotAgent_notCounted(t *testing.T) {
	// A cmd with event=loop-body does not count as the required agent loop-body.
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "body", Kind: pipeline.NodeKindCmd, Event: pipeline.EventLoopBody, Cmd: &pipeline.CmdSpec{Command: "echo"}},
		},
	}
	if err := pipeline.Normalize(p); err == nil {
		t.Fatal("expected error: cmd with loop-body does not satisfy agent loop-body requirement")
	}
}
