package pipeline_test

import (
	"testing"
	"time"

	"github.com/ohare93/juggle/internal/pipeline"
)

// --- Error cases ---

func TestLoadBytes_invalidTOML_returnsError(t *testing.T) {
	_, err := pipeline.LoadBytes([]byte(`not = [valid toml`))
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoadBytes_agentMissingName_returnsError(t *testing.T) {
	data := []byte(`
[[agent]]
prompt = "do it"
event = "loop-body"
`)
	_, err := pipeline.LoadBytes(data)
	if err == nil {
		t.Fatal("expected error when agent is missing name")
	}
}

func TestLoadBytes_agentMissingPrompt_returnsError(t *testing.T) {
	data := []byte(`
[[agent]]
name = "Setup"
event = "run-start"
`)
	_, err := pipeline.LoadBytes(data)
	if err == nil {
		t.Fatal("expected error when agent is missing prompt")
	}
}

func TestLoadBytes_cmdMissingName_returnsError(t *testing.T) {
	data := []byte(`
[[cmd]]
command = "echo hello"
event = "loop-end"
`)
	_, err := pipeline.LoadBytes(data)
	if err == nil {
		t.Fatal("expected error when cmd is missing name")
	}
}

func TestLoadBytes_cmdMissingCommand_returnsError(t *testing.T) {
	data := []byte(`
[[cmd]]
name = "Commit"
event = "loop-end"
`)
	_, err := pipeline.LoadBytes(data)
	if err == nil {
		t.Fatal("expected error when cmd is missing command")
	}
}

func TestLoadBytes_agentInvalidTimeout_returnsError(t *testing.T) {
	data := []byte(`
[[agent]]
name = "Setup"
prompt = "do it"
timeout = "notaduration"
`)
	_, err := pipeline.LoadBytes(data)
	if err == nil {
		t.Fatal("expected error for invalid agent timeout")
	}
}

func TestLoadBytes_cmdInvalidTimeout_returnsError(t *testing.T) {
	data := []byte(`
[[cmd]]
name = "Run"
command = "echo hi"
timeout = "bad"
`)
	_, err := pipeline.LoadBytes(data)
	if err == nil {
		t.Fatal("expected error for invalid cmd timeout")
	}
}

// --- Single agent node ---

func TestLoadBytes_singleAgentNode_returnsOneAgentNode(t *testing.T) {
	data := []byte(`
[[agent]]
name = "Setup"
prompt = "do the thing"
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(p.Nodes))
	}
	n := p.Nodes[0]
	if n.Name != "Setup" {
		t.Errorf("expected name %q, got %q", "Setup", n.Name)
	}
	if n.Kind != pipeline.NodeKindAgent {
		t.Errorf("expected kind %q, got %q", pipeline.NodeKindAgent, n.Kind)
	}
	if n.Agent == nil {
		t.Fatal("expected Agent spec to be non-nil")
	}
	if n.Agent.Prompt != "do the thing" {
		t.Errorf("expected prompt %q, got %q", "do the thing", n.Agent.Prompt)
	}
	if n.Cmd != nil {
		t.Error("expected Cmd spec to be nil")
	}
}

// --- Single cmd node ---

func TestLoadBytes_singleCmdNode_returnsOneCmdNode(t *testing.T) {
	data := []byte(`
[[cmd]]
name = "Commit"
command = "git commit -m done"
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(p.Nodes))
	}
	n := p.Nodes[0]
	if n.Name != "Commit" {
		t.Errorf("expected name %q, got %q", "Commit", n.Name)
	}
	if n.Kind != pipeline.NodeKindCmd {
		t.Errorf("expected kind %q, got %q", pipeline.NodeKindCmd, n.Kind)
	}
	if n.Cmd == nil {
		t.Fatal("expected Cmd spec to be non-nil")
	}
	if n.Cmd.Command != "git commit -m done" {
		t.Errorf("expected command %q, got %q", "git commit -m done", n.Cmd.Command)
	}
	if n.Agent != nil {
		t.Error("expected Agent spec to be nil")
	}
}

// --- Top-level metadata ---

func TestLoadBytes_topLevelMetadata_parsedCorrectly(t *testing.T) {
	data := []byte(`
iterations = 5
max_parallel_steps = 2

[[agent]]
name = "Work"
prompt = "do it"
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Iterations != 5 {
		t.Errorf("expected iterations 5, got %d", p.Iterations)
	}
	if p.MaxParallelSteps != 2 {
		t.Errorf("expected max_parallel_steps 2, got %d", p.MaxParallelSteps)
	}
}

// --- Default agent settings ---

func TestLoadBytes_defaults_storedInPipeline(t *testing.T) {
	data := []byte(`
[defaults]
provider = "claude"
model = "sonnet"

[[agent]]
name = "Work"
prompt = "do it"
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Defaults.Provider != "claude" {
		t.Errorf("expected defaults.provider %q, got %q", "claude", p.Defaults.Provider)
	}
	if p.Defaults.Model != "sonnet" {
		t.Errorf("expected defaults.model %q, got %q", "sonnet", p.Defaults.Model)
	}
}

func TestLoadBytes_agentWithoutModel_nodeModelIsEmpty(t *testing.T) {
	data := []byte(`
[defaults]
provider = "claude"
model = "sonnet"

[[agent]]
name = "Work"
prompt = "do it"
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Agent.Model != "" {
		t.Errorf("expected empty node model (defaults not applied at load time), got %q", p.Nodes[0].Agent.Model)
	}
}

func TestLoadBytes_agentWithExplicitModel_overridesDefault(t *testing.T) {
	data := []byte(`
[defaults]
provider = "claude"
model = "sonnet"

[[agent]]
name = "Work"
prompt = "do it"
model = "haiku"
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Agent.Model != "haiku" {
		t.Errorf("expected node model %q, got %q", "haiku", p.Nodes[0].Agent.Model)
	}
}

// --- Mixed node kinds ---

func TestLoadBytes_mixedNodes_agentsBeforeCmds(t *testing.T) {
	data := []byte(`
[[agent]]
name = "Implement"
prompt = "do the work"
event = "loop-body"

[[cmd]]
name = "Commit"
command = "git commit -m done"
event = "loop-end"
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(p.Nodes))
	}
	if p.Nodes[0].Kind != pipeline.NodeKindAgent {
		t.Errorf("expected first node to be agent, got %q", p.Nodes[0].Kind)
	}
	if p.Nodes[1].Kind != pipeline.NodeKindCmd {
		t.Errorf("expected second node to be cmd, got %q", p.Nodes[1].Kind)
	}
}

func TestLoadBytes_multipleAgentNodes_preserveOrder(t *testing.T) {
	data := []byte(`
[[agent]]
name = "First"
prompt = "do first"

[[agent]]
name = "Second"
prompt = "do second"

[[agent]]
name = "Third"
prompt = "do third"
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(p.Nodes))
	}
	names := []string{"First", "Second", "Third"}
	for i, name := range names {
		if p.Nodes[i].Name != name {
			t.Errorf("node %d: expected name %q, got %q", i, name, p.Nodes[i].Name)
		}
	}
}

func TestLoadBytes_multipleCmdNodes_preserveOrder(t *testing.T) {
	data := []byte(`
[[cmd]]
name = "Lint"
command = "golangci-lint run"

[[cmd]]
name = "Test"
command = "go test ./..."
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(p.Nodes))
	}
	if p.Nodes[0].Name != "Lint" {
		t.Errorf("expected first node name %q, got %q", "Lint", p.Nodes[0].Name)
	}
	if p.Nodes[1].Name != "Test" {
		t.Errorf("expected second node name %q, got %q", "Test", p.Nodes[1].Name)
	}
}

// --- Shared node fields ---

func TestLoadBytes_agentSharedFields_parsedCorrectly(t *testing.T) {
	data := []byte(`
[[agent]]
name = "Setup"
prompt = "do it"
event = "run-start"
after = ["other"]
parallel = true
when = "iteration==1"
on_failure = "continue"
retries = 3
timeout = "5m"
workdir = "/tmp"
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := p.Nodes[0]
	if n.Event != pipeline.EventRunStart {
		t.Errorf("event: expected %q, got %q", pipeline.EventRunStart, n.Event)
	}
	if len(n.After) != 1 || n.After[0] != "other" {
		t.Errorf("after: expected [other], got %v", n.After)
	}
	if !n.Parallel {
		t.Error("expected Parallel to be true")
	}
	if n.When != "iteration==1" {
		t.Errorf("when: expected %q, got %q", "iteration==1", n.When)
	}
	if n.OnFailure != pipeline.FailurePolicyContinue {
		t.Errorf("on_failure: expected %q, got %q", pipeline.FailurePolicyContinue, n.OnFailure)
	}
	if n.Retries != 3 {
		t.Errorf("retries: expected 3, got %d", n.Retries)
	}
	if n.Timeout != 5*time.Minute {
		t.Errorf("timeout: expected 5m, got %v", n.Timeout)
	}
	if n.WorkDir != "/tmp" {
		t.Errorf("workdir: expected /tmp, got %q", n.WorkDir)
	}
}

// --- Agent-specific fields ---

func TestLoadBytes_agentSpecificFields_parsedCorrectly(t *testing.T) {
	data := []byte(`
[[agent]]
name = "Implement"
prompt = "@task.md"
provider = "codex"
model = "gpt-5.4"
plan = true
trust = true
system_prompt = "be concise"
allowed_tools = ["Read", "Grep"]
disallowed_tools = ["Bash"]
max_turns = 10
mcp_config = "mcp.json"
passthrough = ["--verbose"]
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec := p.Nodes[0].Agent
	if spec.Provider != "codex" {
		t.Errorf("provider: expected %q, got %q", "codex", spec.Provider)
	}
	if spec.Model != "gpt-5.4" {
		t.Errorf("model: expected %q, got %q", "gpt-5.4", spec.Model)
	}
	if !spec.Plan {
		t.Error("expected Plan to be true")
	}
	if !spec.Trust {
		t.Error("expected Trust to be true")
	}
	if spec.SystemPrompt != "be concise" {
		t.Errorf("system_prompt: expected %q, got %q", "be concise", spec.SystemPrompt)
	}
	if len(spec.AllowedTools) != 2 || spec.AllowedTools[0] != "Read" || spec.AllowedTools[1] != "Grep" {
		t.Errorf("allowed_tools: expected [Read Grep], got %v", spec.AllowedTools)
	}
	if len(spec.DisallowedTools) != 1 || spec.DisallowedTools[0] != "Bash" {
		t.Errorf("disallowed_tools: expected [Bash], got %v", spec.DisallowedTools)
	}
	if spec.MaxTurns != 10 {
		t.Errorf("max_turns: expected 10, got %d", spec.MaxTurns)
	}
	if spec.MCPConfig != "mcp.json" {
		t.Errorf("mcp_config: expected %q, got %q", "mcp.json", spec.MCPConfig)
	}
	if len(spec.Passthrough) != 1 || spec.Passthrough[0] != "--verbose" {
		t.Errorf("passthrough: expected [--verbose], got %v", spec.Passthrough)
	}
}

// --- Cmd-specific fields ---

func TestLoadBytes_cmdSpecificFields_parsedCorrectly(t *testing.T) {
	data := []byte(`
[[cmd]]
name = "Run"
command = "echo hello"
shell = "bash"
env = ["KEY=value", "OTHER=val"]
`)
	p, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec := p.Nodes[0].Cmd
	if spec.Shell != "bash" {
		t.Errorf("shell: expected %q, got %q", "bash", spec.Shell)
	}
	if len(spec.Env) != 2 || spec.Env[0] != "KEY=value" || spec.Env[1] != "OTHER=val" {
		t.Errorf("env: expected [KEY=value OTHER=val], got %v", spec.Env)
	}
}

// --- File loading ---

func TestLoadFile_validFixture_returnsFullPipeline(t *testing.T) {
	p, err := pipeline.LoadFile("testdata/valid.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Iterations != 10 {
		t.Errorf("iterations: expected 10, got %d", p.Iterations)
	}
	if p.MaxParallelSteps != 4 {
		t.Errorf("max_parallel_steps: expected 4, got %d", p.MaxParallelSteps)
	}
	if p.Defaults.Provider != "claude" {
		t.Errorf("defaults.provider: expected %q, got %q", "claude", p.Defaults.Provider)
	}
	if p.Defaults.Model != "sonnet" {
		t.Errorf("defaults.model: expected %q, got %q", "sonnet", p.Defaults.Model)
	}
	// 3 agents + 2 cmds = 5 nodes
	if len(p.Nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d", len(p.Nodes))
	}
	// Agents come first in V1 file format order.
	if p.Nodes[0].Name != "Setup" || p.Nodes[0].Kind != pipeline.NodeKindAgent {
		t.Errorf("node 0: expected agent %q, got %q (%s)", "Setup", p.Nodes[0].Name, p.Nodes[0].Kind)
	}
	if p.Nodes[1].Name != "Gather" {
		t.Errorf("node 1: expected %q, got %q", "Gather", p.Nodes[1].Name)
	}
	if p.Nodes[2].Name != "Implement" {
		t.Errorf("node 2: expected %q, got %q", "Implement", p.Nodes[2].Name)
	}
	if p.Nodes[3].Name != "Commit" || p.Nodes[3].Kind != pipeline.NodeKindCmd {
		t.Errorf("node 3: expected cmd %q, got %q (%s)", "Commit", p.Nodes[3].Name, p.Nodes[3].Kind)
	}
	if p.Nodes[4].Name != "Notify" {
		t.Errorf("node 4: expected %q, got %q", "Notify", p.Nodes[4].Name)
	}
}

func TestLoadFile_missingFile_returnsError(t *testing.T) {
	_, err := pipeline.LoadFile("testdata/does-not-exist.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
