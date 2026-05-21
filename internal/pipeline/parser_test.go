package pipeline_test

import (
	"testing"
	"time"

	"github.com/jeprecated/juggle/internal/pipeline"
)

// --- Error cases ---

func TestParseArgs_emptyArgs_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{})
	if err == nil {
		t.Fatal("expected error for empty args")
	}
}

func TestParseArgs_unknownFirstToken_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"task", "mynode", "do something"})
	if err == nil {
		t.Fatal("expected error for unknown first token 'task'")
	}
}

func TestParseArgs_agentMissingName_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"agent"})
	if err == nil {
		t.Fatal("expected error when agent has no name")
	}
}

func TestParseArgs_agentNameIsFlag_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"agent", "--event", "loop-body", "my prompt"})
	if err == nil {
		t.Fatal("expected error when agent name starts with -")
	}
}

func TestParseArgs_agentMissingPrompt_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"agent", "mynode"})
	if err == nil {
		t.Fatal("expected error when agent has no prompt")
	}
}

func TestParseArgs_agentMissingPromptWithFlags_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"agent", "mynode", "--event", "loop-body"})
	if err == nil {
		t.Fatal("expected error when agent has flags but no prompt")
	}
}

func TestParseArgs_cmdMissingName_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"cmd"})
	if err == nil {
		t.Fatal("expected error when cmd has no name")
	}
}

func TestParseArgs_cmdMissingCommand_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"cmd", "mynode"})
	if err == nil {
		t.Fatal("expected error when cmd has no command")
	}
}

func TestParseArgs_invalidTimeout_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--timeout", "notaduration"})
	if err == nil {
		t.Fatal("expected error for invalid timeout value")
	}
}

func TestParseArgs_cmdWithAgentOnlyFlag_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"cmd", "mynode", "echo hello", "--provider", "claude"})
	if err == nil {
		t.Fatal("expected error when cmd uses agent-only flag --provider")
	}
}

func TestParseArgs_cmdWithModelFlag_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"cmd", "mynode", "echo hello", "--model", "sonnet"})
	if err == nil {
		t.Fatal("expected error when cmd uses agent-only flag --model")
	}
}

func TestParseArgs_agentWithCmdOnlyFlag_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--shell", "bash"})
	if err == nil {
		t.Fatal("expected error when agent uses cmd-only flag --shell")
	}
}

func TestParseArgs_agentWithEnvFlag_returnsError(t *testing.T) {
	_, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--env", "KEY=val"})
	if err == nil {
		t.Fatal("expected error when agent uses cmd-only flag --env")
	}
}

// --- Single agent node ---

func TestParseArgs_singleAgent_returnsOneAgentNode(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "Setup", "do the thing"})
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

func TestParseArgs_singleCmd_returnsOneCmdNode(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"cmd", "Commit", "git commit -m done"})
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

// --- Multiple nodes ---

func TestParseArgs_multipleAgentNodes_returnsAllNodes(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "First", "do first",
		"agent", "Second", "do second",
		"agent", "Third", "do third",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(p.Nodes))
	}
	cases := []struct{ name, prompt string }{
		{"First", "do first"},
		{"Second", "do second"},
		{"Third", "do third"},
	}
	for i, tc := range cases {
		n := p.Nodes[i]
		if n.Name != tc.name {
			t.Errorf("node %d: expected name %q, got %q", i, tc.name, n.Name)
		}
		if n.Agent == nil || n.Agent.Prompt != tc.prompt {
			t.Errorf("node %d: expected prompt %q, got %v", i, tc.prompt, n.Agent)
		}
	}
}

func TestParseArgs_multipleCmdNodes_returnsAllNodes(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"cmd", "Lint", "golangci-lint run",
		"cmd", "Test", "go test ./...",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(p.Nodes))
	}
}

func TestParseArgs_mixedAgentAndCmd_correctKindsAssigned(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Implement", "do the work",
		"cmd", "Commit", "git commit -m done",
	})
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
	if p.Nodes[0].Agent == nil || p.Nodes[0].Agent.Prompt != "do the work" {
		t.Error("expected first node to have agent prompt set")
	}
	if p.Nodes[1].Cmd == nil || p.Nodes[1].Cmd.Command != "git commit -m done" {
		t.Error("expected second node to have cmd command set")
	}
}

// --- Shared flags ---

func TestParseArgs_eventFlag_parsedIntoNode(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--event", "run-start"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Event != pipeline.EventRunStart {
		t.Errorf("expected event %q, got %q", pipeline.EventRunStart, p.Nodes[0].Event)
	}
}

func TestParseArgs_afterFlag_singleValue(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--after", "other"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes[0].After) != 1 || p.Nodes[0].After[0] != "other" {
		t.Errorf("expected After [other], got %v", p.Nodes[0].After)
	}
}

func TestParseArgs_afterFlag_multipleValues(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "mynode", "do it",
		"--after", "first",
		"--after", "second",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := p.Nodes[0]
	if len(n.After) != 2 {
		t.Fatalf("expected 2 After values, got %d: %v", len(n.After), n.After)
	}
	if n.After[0] != "first" || n.After[1] != "second" {
		t.Errorf("expected After [first second], got %v", n.After)
	}
}

func TestParseArgs_parallelFlag_setsParallel(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--parallel"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Nodes[0].Parallel {
		t.Error("expected Parallel to be true")
	}
}

func TestParseArgs_whenFlag_parsedIntoNode(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--when", "iteration==1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].When != "iteration==1" {
		t.Errorf("expected when %q, got %q", "iteration==1", p.Nodes[0].When)
	}
}

func TestParseArgs_onFailureFlag_parsedIntoNode(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--on-failure", "continue"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].OnFailure != pipeline.FailurePolicyContinue {
		t.Errorf("expected on-failure %q, got %q", pipeline.FailurePolicyContinue, p.Nodes[0].OnFailure)
	}
}

func TestParseArgs_retriesFlag_parsedIntoNode(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--retries", "3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Retries != 3 {
		t.Errorf("expected retries 3, got %d", p.Nodes[0].Retries)
	}
}

func TestParseArgs_timeoutFlag_parsedIntoNode(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--timeout", "5m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Timeout != 5*time.Minute {
		t.Errorf("expected timeout 5m, got %v", p.Nodes[0].Timeout)
	}
}

func TestParseArgs_workdirFlag_parsedIntoNode(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "mynode", "do it", "--workdir", "/tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].WorkDir != "/tmp" {
		t.Errorf("expected workdir /tmp, got %q", p.Nodes[0].WorkDir)
	}
}

func TestParseArgs_allSharedFlags_parsedTogether(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Setup", "do setup",
		"--event", "run-start",
		"--after", "other",
		"--when", "iteration==1",
		"--on-failure", "continue",
		"--retries", "3",
		"--timeout", "5m",
		"--workdir", "/tmp",
	})
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
	if n.When != "iteration==1" {
		t.Errorf("when: expected %q, got %q", "iteration==1", n.When)
	}
	if n.OnFailure != pipeline.FailurePolicyContinue {
		t.Errorf("on-failure: expected %q, got %q", pipeline.FailurePolicyContinue, n.OnFailure)
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

func TestParseArgs_sharedFlagsOnCmdNode_parsedCorrectly(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"cmd", "Commit", "git commit -m done",
		"--event", "loop-end",
		"--parallel",
		"--retries", "2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := p.Nodes[0]
	if n.Event != pipeline.EventLoopEnd {
		t.Errorf("expected event %q, got %q", pipeline.EventLoopEnd, n.Event)
	}
	if !n.Parallel {
		t.Error("expected Parallel to be true")
	}
	if n.Retries != 2 {
		t.Errorf("expected retries 2, got %d", n.Retries)
	}
}

// --- Agent-only flags ---

func TestParseArgs_agentProviderAndModel_parsedIntoSpec(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Implement", "do the work",
		"--provider", "claude",
		"--model", "sonnet",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec := p.Nodes[0].Agent
	if spec.Provider != "claude" {
		t.Errorf("expected provider %q, got %q", "claude", spec.Provider)
	}
	if spec.Model != "sonnet" {
		t.Errorf("expected model %q, got %q", "sonnet", spec.Model)
	}
}

func TestParseArgs_agentPlanAndTrust_parsedIntoSpec(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Implement", "do the work",
		"--plan",
		"--trust",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec := p.Nodes[0].Agent
	if !spec.Plan {
		t.Error("expected Plan to be true")
	}
	if !spec.Trust {
		t.Error("expected Trust to be true")
	}
}

func TestParseArgs_agentSystemPrompt_parsedIntoSpec(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Implement", "do the work",
		"--system-prompt", "be helpful",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Agent.SystemPrompt != "be helpful" {
		t.Errorf("expected system-prompt %q, got %q", "be helpful", p.Nodes[0].Agent.SystemPrompt)
	}
}

func TestParseArgs_agentAllowedAndDisallowedTools_parsedIntoSpec(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Implement", "do the work",
		"--allowed-tools", "Read",
		"--allowed-tools", "Grep",
		"--disallowed-tools", "Bash",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec := p.Nodes[0].Agent
	if len(spec.AllowedTools) != 2 || spec.AllowedTools[0] != "Read" || spec.AllowedTools[1] != "Grep" {
		t.Errorf("expected AllowedTools [Read Grep], got %v", spec.AllowedTools)
	}
	if len(spec.DisallowedTools) != 1 || spec.DisallowedTools[0] != "Bash" {
		t.Errorf("expected DisallowedTools [Bash], got %v", spec.DisallowedTools)
	}
}

func TestParseArgs_agentMaxTurns_parsedIntoSpec(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Implement", "do the work",
		"--max-turns", "10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Agent.MaxTurns != 10 {
		t.Errorf("expected max-turns 10, got %d", p.Nodes[0].Agent.MaxTurns)
	}
}

func TestParseArgs_agentMCPConfig_parsedIntoSpec(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Implement", "do the work",
		"--mcp-config", "mcp.json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Agent.MCPConfig != "mcp.json" {
		t.Errorf("expected mcp-config %q, got %q", "mcp.json", p.Nodes[0].Agent.MCPConfig)
	}
}

func TestParseArgs_agentPassthrough_parsedIntoSpec(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Implement", "do the work",
		"--passthrough=--verbose",
		"--passthrough=--debug",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spec := p.Nodes[0].Agent
	if len(spec.Passthrough) != 2 || spec.Passthrough[0] != "--verbose" || spec.Passthrough[1] != "--debug" {
		t.Errorf("expected Passthrough [--verbose --debug], got %v", spec.Passthrough)
	}
}

// --- Cmd-only flags ---

func TestParseArgs_cmdShell_parsedIntoSpec(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"cmd", "Run", "echo hello",
		"--shell", "bash",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Cmd.Shell != "bash" {
		t.Errorf("expected shell %q, got %q", "bash", p.Nodes[0].Cmd.Shell)
	}
}

func TestParseArgs_cmdEnv_parsedIntoSpec(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"cmd", "Run", "echo hello",
		"--env", "KEY=value",
		"--env", "OTHER=val",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := p.Nodes[0].Cmd.Env
	if len(env) != 2 || env[0] != "KEY=value" || env[1] != "OTHER=val" {
		t.Errorf("expected Env [KEY=value OTHER=val], got %v", env)
	}
}

// --- Flags before prompt (interleaved) ---

func TestParseArgs_flagsBeforePrompt_parsedCorrectly(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "mynode",
		"--event", "loop-body",
		"my prompt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	n := p.Nodes[0]
	if n.Event != pipeline.EventLoopBody {
		t.Errorf("expected event loop-body, got %q", n.Event)
	}
	if n.Agent.Prompt != "my prompt" {
		t.Errorf("expected prompt %q, got %q", "my prompt", n.Agent.Prompt)
	}
}

// --- Multi-word prompt (joined) ---

func TestParseArgs_multiWordPromptTokens_joinedWithSpaces(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{"agent", "mynode", "do", "the", "thing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Agent.Prompt != "do the thing" {
		t.Errorf("expected joined prompt %q, got %q", "do the thing", p.Nodes[0].Agent.Prompt)
	}
}

// --- Nodes scoped correctly (flags don't bleed across blocks) ---

func TestParseArgs_perNodeFlags_doNotBleedAcrossNodes(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "First", "first prompt", "--event", "run-start",
		"agent", "Second", "second prompt", "--event", "loop-body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Nodes[0].Event != pipeline.EventRunStart {
		t.Errorf("node 0: expected event run-start, got %q", p.Nodes[0].Event)
	}
	if p.Nodes[1].Event != pipeline.EventLoopBody {
		t.Errorf("node 1: expected event loop-body, got %q", p.Nodes[1].Event)
	}
}

func TestParseArgs_mixedFlagsPerNode_scopedCorrectly(t *testing.T) {
	p, err := pipeline.ParseArgs([]string{
		"agent", "Setup", "@setup.md", "--event", "run-start", "--model", "haiku",
		"agent", "Implement", "@task.md", "--event", "loop-body",
		"cmd", "Commit", "git add -A && git commit -m done", "--event", "loop-end",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(p.Nodes))
	}
	if p.Nodes[0].Agent.Model != "haiku" {
		t.Errorf("node 0: expected model haiku, got %q", p.Nodes[0].Agent.Model)
	}
	if p.Nodes[1].Agent.Model != "" {
		t.Errorf("node 1: expected no model, got %q", p.Nodes[1].Agent.Model)
	}
	if p.Nodes[2].Kind != pipeline.NodeKindCmd {
		t.Errorf("node 2: expected cmd kind")
	}
}
