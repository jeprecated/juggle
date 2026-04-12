package pipeline

import (
	"bytes"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// saveDefaults is the serialization struct for the [defaults] section.
type saveDefaults struct {
	Provider string `toml:"provider,omitempty"`
	Model    string `toml:"model,omitempty"`
}

// saveAgent is the serialization struct for [[agent]] sections.
type saveAgent struct {
	Name            string   `toml:"name"`
	Prompt          string   `toml:"prompt"`
	Event           string   `toml:"event,omitempty"`
	After           []string `toml:"after,omitempty"`
	Parallel        bool     `toml:"parallel,omitempty"`
	When            string   `toml:"when,omitempty"`
	OnFailure       string   `toml:"on_failure,omitempty"`
	Retries         int      `toml:"retries,omitempty"`
	Timeout         string   `toml:"timeout,omitempty"`
	WorkDir         string   `toml:"workdir,omitempty"`
	Provider        string   `toml:"provider,omitempty"`
	Model           string   `toml:"model,omitempty"`
	Plan            bool     `toml:"plan,omitempty"`
	Trust           bool     `toml:"trust,omitempty"`
	SystemPrompt        string   `toml:"system_prompt,omitempty"`
	AllowedTools    []string `toml:"allowed_tools,omitempty"`
	DisallowedTools []string `toml:"disallowed_tools,omitempty"`
	MaxTurns        int      `toml:"max_turns,omitempty"`
	MCPConfig       string   `toml:"mcp_config,omitempty"`
	Passthrough     []string `toml:"passthrough,omitempty"`
}

// saveCmd is the serialization struct for [[cmd]] sections.
type saveCmd struct {
	Name      string   `toml:"name"`
	Command   string   `toml:"command"`
	Event     string   `toml:"event,omitempty"`
	After     []string `toml:"after,omitempty"`
	Parallel  bool     `toml:"parallel,omitempty"`
	When      string   `toml:"when,omitempty"`
	OnFailure string   `toml:"on_failure,omitempty"`
	Retries   int      `toml:"retries,omitempty"`
	Timeout   string   `toml:"timeout,omitempty"`
	WorkDir   string   `toml:"workdir,omitempty"`
	Shell     string   `toml:"shell,omitempty"`
	Env       []string `toml:"env,omitempty"`
}

// saveDoc is the top-level serialization struct for a pipeline file.
type saveDoc struct {
	Iterations       int          `toml:"iterations,omitempty"`
	MaxParallelSteps int          `toml:"max_parallel_steps,omitempty"`
	Defaults         saveDefaults `toml:"defaults,omitempty"`
	Agents           []saveAgent  `toml:"agent,omitempty"`
	Cmds             []saveCmd    `toml:"cmd,omitempty"`
}

// SaveBytes serializes a Pipeline to TOML bytes.
func SaveBytes(p *Pipeline) ([]byte, error) {
	doc := pipelineToSaveDoc(p)
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return nil, fmt.Errorf("pipeline: encode TOML: %w", err)
	}
	return buf.Bytes(), nil
}

// SaveFile writes a Pipeline to a TOML file at path.
func SaveFile(path string, p *Pipeline) error {
	data, err := SaveBytes(p)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("pipeline: write %s: %w", path, err)
	}
	return nil
}

func pipelineToSaveDoc(p *Pipeline) saveDoc {
	doc := saveDoc{
		Iterations:       p.Iterations,
		MaxParallelSteps: p.MaxParallelSteps,
		Defaults: saveDefaults{
			Provider: p.Defaults.Provider,
			Model:    p.Defaults.Model,
		},
	}
	for _, n := range p.Nodes {
		switch n.Kind {
		case NodeKindAgent:
			doc.Agents = append(doc.Agents, nodeToSaveAgent(n))
		case NodeKindCmd:
			doc.Cmds = append(doc.Cmds, nodeToSaveCmd(n))
		}
	}
	return doc
}

func nodeToSaveAgent(n Node) saveAgent {
	a := saveAgent{
		Name:      n.Name,
		Event:     string(n.Event),
		After:     n.After,
		Parallel:  n.Parallel,
		When:      n.When,
		OnFailure: string(n.OnFailure),
		Retries:   n.Retries,
		WorkDir:   n.WorkDir,
	}
	if n.Timeout != 0 {
		a.Timeout = n.Timeout.String()
	}
	if n.Agent != nil {
		a.Prompt = n.Agent.Prompt
		a.Provider = n.Agent.Provider
		a.Model = n.Agent.Model
		a.Plan = n.Agent.Plan
		a.Trust = n.Agent.Trust
		a.SystemPrompt = n.Agent.SystemPrompt
		a.AllowedTools = n.Agent.AllowedTools
		a.DisallowedTools = n.Agent.DisallowedTools
		a.MaxTurns = n.Agent.MaxTurns
		a.MCPConfig = n.Agent.MCPConfig
		a.Passthrough = n.Agent.Passthrough
	}
	return a
}

func nodeToSaveCmd(n Node) saveCmd {
	c := saveCmd{
		Name:      n.Name,
		Event:     string(n.Event),
		After:     n.After,
		Parallel:  n.Parallel,
		When:      n.When,
		OnFailure: string(n.OnFailure),
		Retries:   n.Retries,
		WorkDir:   n.WorkDir,
	}
	if n.Timeout != 0 {
		c.Timeout = n.Timeout.String()
	}
	if n.Cmd != nil {
		c.Command = n.Cmd.Command
		c.Shell = n.Cmd.Shell
		c.Env = n.Cmd.Env
	}
	return c
}
