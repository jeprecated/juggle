package pipeline

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// ParseArgs parses inline CLI arguments into a Pipeline.
//
// The expected format is one or more node blocks:
//
//	(agent|cmd) <name> <prompt-or-command> [flags...]
//
// Each "agent" or "cmd" keyword starts a new node block. Tokens following the
// keyword belong to that block until the next "agent" or "cmd".
func ParseArgs(args []string) (*Pipeline, error) {
	blocks, err := splitIntoBlocks(args)
	if err != nil {
		return nil, err
	}
	nodes := make([]Node, 0, len(blocks))
	for _, b := range blocks {
		node, err := parseNodeBlock(b)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return &Pipeline{Nodes: nodes}, nil
}

// nodeBlock holds the raw tokens for a single node.
type nodeBlock struct {
	kind string   // "agent" or "cmd"
	args []string // everything after the kind keyword
}

// splitIntoBlocks groups args by "agent" / "cmd" delimiters.
func splitIntoBlocks(args []string) ([]nodeBlock, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("pipeline requires at least one node block (agent or cmd)")
	}
	var blocks []nodeBlock
	var current *nodeBlock
	for _, arg := range args {
		if arg == "agent" || arg == "cmd" {
			if current != nil {
				blocks = append(blocks, *current)
			}
			current = &nodeBlock{kind: arg}
		} else {
			if current == nil {
				return nil, fmt.Errorf("unexpected token %q: expected 'agent' or 'cmd' to start a node block", arg)
			}
			current.args = append(current.args, arg)
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}
	return blocks, nil
}

// parseNodeBlock converts a single nodeBlock into a Node.
func parseNodeBlock(b nodeBlock) (Node, error) {
	if len(b.args) == 0 {
		return Node{}, fmt.Errorf("%s: missing name", b.kind)
	}
	name := b.args[0]
	if strings.HasPrefix(name, "-") {
		return Node{}, fmt.Errorf("%s: expected name, got flag %q", b.kind, name)
	}

	fs := pflag.NewFlagSet(name, pflag.ContinueOnError)
	fs.SetOutput(io.Discard) // suppress pflag's own error messages

	// Shared flags (available on both agent and cmd nodes).
	var (
		event     string
		after     []string
		parallel  bool
		when      string
		onFailure string
		retries   int
		timeout   string
		workdir   string
		nodeID    string
	)
	fs.StringVar(&event, "event", "", "lifecycle event")
	fs.StringArrayVar(&after, "after", nil, "explicit dependency on named node (repeatable)")
	fs.BoolVar(&parallel, "parallel", false, "suppress implicit previous-node dependency")
	fs.StringVar(&when, "when", "", "condition expression")
	fs.StringVar(&onFailure, "on-failure", "", "failure policy: stop, continue, retry")
	fs.IntVar(&retries, "retries", 0, "retry count")
	fs.StringVar(&timeout, "timeout", "", "per-node timeout (e.g. 5m)")
	fs.StringVar(&workdir, "workdir", "", "working directory")
	fs.StringVar(&nodeID, "id", "", "stable identifier for trigger targeting")

	// Kind-specific flags.
	var (
		provider        string
		model           string
		plan            bool
		trust           bool
		systemPrompt        string
		allowedTools    []string
		disallowedTools []string
		maxTurns        int
		mcpConfig       string
		passthrough     []string
		shell           string
		env             []string
	)
	switch b.kind {
	case "agent":
		fs.StringVar(&provider, "provider", "", "agent provider")
		fs.StringVar(&model, "model", "", "agent model")
		fs.BoolVar(&plan, "plan", false, "enable plan mode")
		fs.BoolVar(&trust, "trust", false, "trust all tools")
		fs.StringVar(&systemPrompt, "system-prompt", "", "system prompt")
		fs.StringArrayVar(&allowedTools, "allowed-tools", nil, "allowed tools (repeatable)")
		fs.StringArrayVar(&disallowedTools, "disallowed-tools", nil, "disallowed tools (repeatable)")
		fs.IntVar(&maxTurns, "max-turns", 0, "max agent turns")
		fs.StringVar(&mcpConfig, "mcp-config", "", "MCP config path")
		fs.StringArrayVar(&passthrough, "passthrough", nil, "passthrough args to agent (repeatable)")
	case "cmd":
		fs.StringVar(&shell, "shell", "", "shell interpreter")
		fs.StringArrayVar(&env, "env", nil, "environment variables as KEY=VALUE (repeatable)")
	}

	if err := fs.Parse(b.args[1:]); err != nil {
		return Node{}, fmt.Errorf("%s %q: %w", b.kind, name, err)
	}

	positional := fs.Args()

	node := Node{
		Name:     name,
		ID:       nodeID,
		Kind:     NodeKind(b.kind),
		After:    after,
		Parallel: parallel,
		When:     when,
		Retries:  retries,
		WorkDir:  workdir,
	}
	if event != "" {
		node.Event = Event(event)
	}
	if onFailure != "" {
		node.OnFailure = FailurePolicy(onFailure)
	}
	if timeout != "" {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return Node{}, fmt.Errorf("%s %q: invalid --timeout %q: %w", b.kind, name, timeout, err)
		}
		node.Timeout = d
	}

	switch b.kind {
	case "agent":
		if len(positional) == 0 {
			return Node{}, fmt.Errorf("agent %q: missing prompt", name)
		}
		node.Agent = &AgentSpec{
			Prompt:          strings.Join(positional, " "),
			Provider:        provider,
			Model:           model,
			Plan:            plan,
			Trust:               trust,
			SystemPrompt:        systemPrompt,
			AllowedTools:    allowedTools,
			DisallowedTools: disallowedTools,
			MaxTurns:        maxTurns,
			MCPConfig:       mcpConfig,
			Passthrough:     passthrough,
		}
	case "cmd":
		if len(positional) == 0 {
			return Node{}, fmt.Errorf("cmd %q: missing command", name)
		}
		node.Cmd = &CmdSpec{
			Command: strings.Join(positional, " "),
			Shell:   shell,
			Env:     env,
		}
	}

	return node, nil
}
