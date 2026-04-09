package cli

import (
	"testing"

	"github.com/ohare93/juggle/internal/pipeline"
)

func minAdapterConfig() Config {
	return Config{
		Content:    "do the thing",
		Iterations: 1,
	}
}

func TestAdaptConfigToPipeline_OnlyMainAgent(t *testing.T) {
	p := AdaptConfigToPipeline(minAdapterConfig())
	if len(p.Nodes) != 1 {
		t.Fatalf("want 1 node, got %d", len(p.Nodes))
	}
	n := p.Nodes[0]
	if n.Kind != pipeline.NodeKindAgent {
		t.Errorf("want NodeKindAgent, got %q", n.Kind)
	}
	if n.Event != pipeline.EventLoopBody {
		t.Errorf("want EventLoopBody, got %q", n.Event)
	}
	if n.Agent.Prompt != "do the thing" {
		t.Errorf("want prompt %q, got %q", "do the thing", n.Agent.Prompt)
	}
}

func TestAdaptConfigToPipeline_AgentPre(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.AgentPre = "setup step"
	p := AdaptConfigToPipeline(cfg)

	var found *pipeline.Node
	for i := range p.Nodes {
		if p.Nodes[i].Name == "agent-pre" {
			found = &p.Nodes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("want agent-pre node, not found")
	}
	if found.Kind != pipeline.NodeKindAgent {
		t.Errorf("want NodeKindAgent, got %q", found.Kind)
	}
	if found.Event != pipeline.EventRunStart {
		t.Errorf("want EventRunStart, got %q", found.Event)
	}
	if found.OnFailure != pipeline.FailurePolicyStop {
		t.Errorf("want FailurePolicyStop, got %q", found.OnFailure)
	}
	if found.Agent.Prompt != "setup step" {
		t.Errorf("want prompt %q, got %q", "setup step", found.Agent.Prompt)
	}
}

func TestAdaptConfigToPipeline_AgentBefore(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.AgentBefore = "before each"
	p := AdaptConfigToPipeline(cfg)

	var found *pipeline.Node
	for i := range p.Nodes {
		if p.Nodes[i].Name == "agent-before" {
			found = &p.Nodes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("want agent-before node, not found")
	}
	if found.Event != pipeline.EventLoopStart {
		t.Errorf("want EventLoopStart, got %q", found.Event)
	}
	if found.OnFailure != pipeline.FailurePolicyStop {
		t.Errorf("want FailurePolicyStop, got %q", found.OnFailure)
	}
}

func TestAdaptConfigToPipeline_AgentAfter(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.AgentAfter = "after each"
	p := AdaptConfigToPipeline(cfg)

	var found *pipeline.Node
	for i := range p.Nodes {
		if p.Nodes[i].Name == "agent-after" {
			found = &p.Nodes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("want agent-after node, not found")
	}
	if found.Event != pipeline.EventLoopEnd {
		t.Errorf("want EventLoopEnd, got %q", found.Event)
	}
	if found.OnFailure != pipeline.FailurePolicyContinue {
		t.Errorf("want FailurePolicyContinue, got %q", found.OnFailure)
	}
}

func TestAdaptConfigToPipeline_AgentPost(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.AgentPost = "teardown step"
	p := AdaptConfigToPipeline(cfg)

	var found *pipeline.Node
	for i := range p.Nodes {
		if p.Nodes[i].Name == "agent-post" {
			found = &p.Nodes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("want agent-post node, not found")
	}
	if found.Event != pipeline.EventRunEnd {
		t.Errorf("want EventRunEnd, got %q", found.Event)
	}
	if found.OnFailure != pipeline.FailurePolicyStop {
		t.Errorf("want FailurePolicyStop, got %q", found.OnFailure)
	}
}

func TestAdaptConfigToPipeline_CmdBefore(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.CmdBefore = "make build"
	p := AdaptConfigToPipeline(cfg)

	var found *pipeline.Node
	for i := range p.Nodes {
		if p.Nodes[i].Name == "cmd-before" {
			found = &p.Nodes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("want cmd-before node, not found")
	}
	if found.Kind != pipeline.NodeKindCmd {
		t.Errorf("want NodeKindCmd, got %q", found.Kind)
	}
	if found.Event != pipeline.EventLoopStart {
		t.Errorf("want EventLoopStart, got %q", found.Event)
	}
	if found.OnFailure != pipeline.FailurePolicyStop {
		t.Errorf("want FailurePolicyStop, got %q", found.OnFailure)
	}
	if found.Cmd.Command != "make build" {
		t.Errorf("want command %q, got %q", "make build", found.Cmd.Command)
	}
}

func TestAdaptConfigToPipeline_CmdAfter(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.CmdAfter = "git status"
	p := AdaptConfigToPipeline(cfg)

	var found *pipeline.Node
	for i := range p.Nodes {
		if p.Nodes[i].Name == "cmd-after" {
			found = &p.Nodes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("want cmd-after node, not found")
	}
	if found.Kind != pipeline.NodeKindCmd {
		t.Errorf("want NodeKindCmd, got %q", found.Kind)
	}
	if found.Event != pipeline.EventLoopEnd {
		t.Errorf("want EventLoopEnd, got %q", found.Event)
	}
	if found.OnFailure != pipeline.FailurePolicyContinue {
		t.Errorf("want FailurePolicyContinue, got %q", found.OnFailure)
	}
}

func TestAdaptConfigToPipeline_NodeOrdering(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.AgentPre = "pre"
	cfg.CmdBefore = "cmd-b"
	cfg.AgentBefore = "before"
	cfg.AgentAfter = "after"
	cfg.CmdAfter = "cmd-a"
	cfg.AgentPost = "post"

	p := AdaptConfigToPipeline(cfg)

	wantOrder := []string{"agent-pre", "cmd-before", "agent-before", "main", "agent-after", "cmd-after", "agent-post"}
	if len(p.Nodes) != len(wantOrder) {
		t.Fatalf("want %d nodes, got %d", len(wantOrder), len(p.Nodes))
	}
	for i, name := range wantOrder {
		if p.Nodes[i].Name != name {
			t.Errorf("node[%d]: want %q, got %q", i, name, p.Nodes[i].Name)
		}
	}
}

func TestAdaptConfigToPipeline_MainAgentFieldsPreserved(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.Provider = "claude"
	cfg.Model = "sonnet"
	cfg.Trust = true
	cfg.Plan = false
	cfg.SystemPrompt = "you are helpful"
	cfg.AllowedTools = []string{"Read", "Edit"}
	cfg.DisallowedTools = []string{"Bash"}
	cfg.MaxTurns = 5
	cfg.MCPConfig = "/path/to/mcp.json"
	cfg.PassthroughArgs = []string{"--flag", "val"}

	p := AdaptConfigToPipeline(cfg)

	var main *pipeline.Node
	for i := range p.Nodes {
		if p.Nodes[i].Name == "main" {
			main = &p.Nodes[i]
			break
		}
	}
	if main == nil {
		t.Fatal("no main node found")
	}

	spec := main.Agent
	if spec.Provider != "claude" {
		t.Errorf("Provider: want %q, got %q", "claude", spec.Provider)
	}
	if spec.Model != "sonnet" {
		t.Errorf("Model: want %q, got %q", "sonnet", spec.Model)
	}
	if !spec.Trust {
		t.Error("Trust: want true")
	}
	if spec.Plan {
		t.Error("Plan: want false")
	}
	if spec.SystemPrompt != "you are helpful" {
		t.Errorf("SystemPrompt: want %q, got %q", "you are helpful", spec.SystemPrompt)
	}
	if len(spec.AllowedTools) != 2 || spec.AllowedTools[0] != "Read" {
		t.Errorf("AllowedTools: got %v", spec.AllowedTools)
	}
	if len(spec.DisallowedTools) != 1 || spec.DisallowedTools[0] != "Bash" {
		t.Errorf("DisallowedTools: got %v", spec.DisallowedTools)
	}
	if spec.MaxTurns != 5 {
		t.Errorf("MaxTurns: want 5, got %d", spec.MaxTurns)
	}
	if spec.MCPConfig != "/path/to/mcp.json" {
		t.Errorf("MCPConfig: want %q, got %q", "/path/to/mcp.json", spec.MCPConfig)
	}
	if len(spec.Passthrough) != 2 || spec.Passthrough[0] != "--flag" {
		t.Errorf("Passthrough: got %v", spec.Passthrough)
	}
}

func TestAdaptConfigToPipeline_Iterations(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.Iterations = 7
	p := AdaptConfigToPipeline(cfg)
	if p.Iterations != 7 {
		t.Errorf("want 7 iterations, got %d", p.Iterations)
	}
}

func TestAdaptConfigToPipeline_ZeroIterationsDefaultsToOne(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.Iterations = 0
	p := AdaptConfigToPipeline(cfg)
	if p.Iterations != 1 {
		t.Errorf("want 1 iteration for zero input, got %d", p.Iterations)
	}
}

func TestAdaptConfigToPipeline_PhaseAgentsInheritProviderModel(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.Provider = "opencode"
	cfg.Model = "gpt-4o"
	cfg.AgentPre = "pre"
	cfg.AgentBefore = "before"
	cfg.AgentAfter = "after"
	cfg.AgentPost = "post"

	p := AdaptConfigToPipeline(cfg)

	for _, n := range p.Nodes {
		if n.Kind != pipeline.NodeKindAgent || n.Name == "main" {
			continue
		}
		if n.Agent.Provider != "opencode" {
			t.Errorf("node %q: Provider want %q, got %q", n.Name, "opencode", n.Agent.Provider)
		}
		if n.Agent.Model != "gpt-4o" {
			t.Errorf("node %q: Model want %q, got %q", n.Name, "gpt-4o", n.Agent.Model)
		}
	}
}

func TestAdaptConfigToPipeline_AdaptedPipelineValidates(t *testing.T) {
	cfg := minAdapterConfig()
	cfg.AgentPre = "pre"
	cfg.CmdBefore = "make build"
	cfg.AgentBefore = "before"
	cfg.AgentAfter = "after"
	cfg.CmdAfter = "git status"
	cfg.AgentPost = "post"

	p := AdaptConfigToPipeline(cfg)
	if err := pipeline.Normalize(p); err != nil {
		t.Errorf("Normalize failed on adapted pipeline: %v", err)
	}
}
