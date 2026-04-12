package pipeline

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// rawAgent is the TOML intermediate representation for an [[agent]] node.
type rawAgent struct {
	Name            string   `toml:"name"`
	Prompt          string   `toml:"prompt"`
	Event           string   `toml:"event"`
	After           []string `toml:"after"`
	Parallel        bool     `toml:"parallel"`
	When            string   `toml:"when"`
	OnFailure       string   `toml:"on_failure"`
	Retries         int      `toml:"retries"`
	Timeout         string   `toml:"timeout"`
	WorkDir         string   `toml:"workdir"`
	Provider        string   `toml:"provider"`
	Model           string   `toml:"model"`
	Plan            bool     `toml:"plan"`
	Trust               bool     `toml:"trust"`
	SystemPrompt        string   `toml:"system_prompt"`
	AllowedTools        []string `toml:"allowed_tools"`
	DisallowedTools []string `toml:"disallowed_tools"`
	MaxTurns        int      `toml:"max_turns"`
	MCPConfig       string   `toml:"mcp_config"`
	Passthrough     []string `toml:"passthrough"`
}

// rawCmd is the TOML intermediate representation for a [[cmd]] node.
type rawCmd struct {
	Name      string   `toml:"name"`
	Command   string   `toml:"command"`
	Event     string   `toml:"event"`
	After     []string `toml:"after"`
	Parallel  bool     `toml:"parallel"`
	When      string   `toml:"when"`
	OnFailure string   `toml:"on_failure"`
	Retries   int      `toml:"retries"`
	Timeout   string   `toml:"timeout"`
	WorkDir   string   `toml:"workdir"`
	Shell     string   `toml:"shell"`
	Env       []string `toml:"env"`
}

// fileDoc is the top-level TOML document structure for a pipeline file.
type fileDoc struct {
	Iterations       int        `toml:"iterations"`
	MaxParallelSteps int        `toml:"max_parallel_steps"`
	Defaults         Defaults   `toml:"defaults"`
	Agents           []rawAgent `toml:"agent"`
	Cmds             []rawCmd   `toml:"cmd"`
}

// LoadFile reads a TOML pipeline file and returns the canonical Pipeline.
func LoadFile(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("pipeline: read %s: %w", path, err)
	}
	p, err := LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("pipeline: %s: %w", path, err)
	}
	return p, nil
}

// LoadBytes parses TOML pipeline content from bytes and returns the canonical Pipeline.
func LoadBytes(data []byte) (*Pipeline, error) {
	var doc fileDoc
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	return buildFromDoc(doc)
}

func buildFromDoc(doc fileDoc) (*Pipeline, error) {
	p := &Pipeline{
		Iterations:       doc.Iterations,
		MaxParallelSteps: doc.MaxParallelSteps,
		Defaults:         doc.Defaults,
	}

	nodes := make([]Node, 0, len(doc.Agents)+len(doc.Cmds))

	// V1: [[agent]] nodes are ordered before [[cmd]] nodes. Use "after" for
	// explicit cross-kind dependencies when interleaving is required.
	for i, ra := range doc.Agents {
		n, err := agentToNode(ra)
		if err != nil {
			return nil, fmt.Errorf("[[agent]] #%d: %w", i+1, err)
		}
		nodes = append(nodes, n)
	}

	for i, rc := range doc.Cmds {
		n, err := cmdToNode(rc)
		if err != nil {
			return nil, fmt.Errorf("[[cmd]] #%d: %w", i+1, err)
		}
		nodes = append(nodes, n)
	}

	p.Nodes = nodes
	return p, nil
}

func agentToNode(ra rawAgent) (Node, error) {
	if ra.Name == "" {
		return Node{}, fmt.Errorf("missing name")
	}
	if ra.Prompt == "" {
		return Node{}, fmt.Errorf("%q: missing prompt", ra.Name)
	}

	n := Node{
		Name:     ra.Name,
		Kind:     NodeKindAgent,
		After:    ra.After,
		Parallel: ra.Parallel,
		When:     ra.When,
		Retries:  ra.Retries,
		WorkDir:  ra.WorkDir,
		Agent: &AgentSpec{
			Prompt:          ra.Prompt,
			Provider:        ra.Provider,
			Model:           ra.Model,
			Plan:            ra.Plan,
			Trust:               ra.Trust,
			SystemPrompt:        ra.SystemPrompt,
			AllowedTools:        ra.AllowedTools,
			DisallowedTools: ra.DisallowedTools,
			MaxTurns:        ra.MaxTurns,
			MCPConfig:       ra.MCPConfig,
			Passthrough:     ra.Passthrough,
		},
	}
	if ra.Event != "" {
		n.Event = Event(ra.Event)
	}
	if ra.OnFailure != "" {
		n.OnFailure = FailurePolicy(ra.OnFailure)
	}
	if ra.Timeout != "" {
		d, err := time.ParseDuration(ra.Timeout)
		if err != nil {
			return Node{}, fmt.Errorf("%q: invalid timeout %q: %w", ra.Name, ra.Timeout, err)
		}
		n.Timeout = d
	}
	return n, nil
}

func cmdToNode(rc rawCmd) (Node, error) {
	if rc.Name == "" {
		return Node{}, fmt.Errorf("missing name")
	}
	if rc.Command == "" {
		return Node{}, fmt.Errorf("%q: missing command", rc.Name)
	}

	n := Node{
		Name:     rc.Name,
		Kind:     NodeKindCmd,
		After:    rc.After,
		Parallel: rc.Parallel,
		When:     rc.When,
		Retries:  rc.Retries,
		WorkDir:  rc.WorkDir,
		Cmd: &CmdSpec{
			Command: rc.Command,
			Shell:   rc.Shell,
			Env:     rc.Env,
		},
	}
	if rc.Event != "" {
		n.Event = Event(rc.Event)
	}
	if rc.OnFailure != "" {
		n.OnFailure = FailurePolicy(rc.OnFailure)
	}
	if rc.Timeout != "" {
		d, err := time.ParseDuration(rc.Timeout)
		if err != nil {
			return Node{}, fmt.Errorf("%q: invalid timeout %q: %w", rc.Name, rc.Timeout, err)
		}
		n.Timeout = d
	}
	return n, nil
}
