// Package pipeline defines the canonical types for juggle pipeline execution.
// Pipelines are composed of ordered nodes (agent or cmd) with lifecycle events,
// dependency declarations, conditions, and failure policies.
package pipeline

import "time"

// Event names the lifecycle point at which a node runs.
type Event string

const (
	EventRunStart  Event = "run-start"
	EventLoopStart Event = "loop-start"
	EventLoopBody  Event = "loop-body"
	EventLoopEnd   Event = "loop-end"
	EventRunEnd    Event = "run-end"
	EventFailure   Event = "failure"
)

var validEvents = map[Event]struct{}{
	EventRunStart:  {},
	EventLoopStart: {},
	EventLoopBody:  {},
	EventLoopEnd:   {},
	EventRunEnd:    {},
	EventFailure:   {},
}

// Valid reports whether e is a recognised event name.
func (e Event) Valid() bool {
	_, ok := validEvents[e]
	return ok
}

// NodeKind is either "agent" or "cmd".
type NodeKind string

const (
	NodeKindAgent NodeKind = "agent"
	NodeKindCmd   NodeKind = "cmd"
)

// Valid reports whether k is a recognised node kind.
func (k NodeKind) Valid() bool {
	return k == NodeKindAgent || k == NodeKindCmd
}

// FailurePolicy controls what happens when a node fails.
type FailurePolicy string

const (
	FailurePolicyStop     FailurePolicy = "stop"
	FailurePolicyContinue FailurePolicy = "continue"
	FailurePolicyRetry    FailurePolicy = "retry"
)

// Valid reports whether p is a recognised failure policy.
func (p FailurePolicy) Valid() bool {
	return p == FailurePolicyStop || p == FailurePolicyContinue || p == FailurePolicyRetry
}

// Defaults holds pipeline-wide default settings applied when a node omits them.
type Defaults struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
}

// Pipeline is the top-level canonical representation of a juggle pipeline.
type Pipeline struct {
	Iterations       int      `toml:"iterations"`
	MaxParallelSteps int      `toml:"max_parallel_steps"`
	Defaults         Defaults `toml:"defaults"`
	Nodes            []Node
}

// Node is a single executable step in a pipeline. Either Agent or Cmd must be
// set, consistent with Kind.
type Node struct {
	Name      string
	Kind      NodeKind
	Event     Event
	After     []string // explicit dependency names; empty means implicit previous-node
	Parallel  bool     // when true, suppresses the implicit previous-node dependency
	When      string   // optional condition expression
	OnFailure FailurePolicy
	Retries   int
	Timeout   time.Duration
	WorkDir   string
	Agent     *AgentSpec // non-nil when Kind == NodeKindAgent
	Cmd       *CmdSpec   // non-nil when Kind == NodeKindCmd
}

// EffectiveFailurePolicy returns the node's failure policy, defaulting to
// FailurePolicyStop when OnFailure is unset.
func (n Node) EffectiveFailurePolicy() FailurePolicy {
	if n.OnFailure == "" {
		return FailurePolicyStop
	}
	return n.OnFailure
}

// AgentSpec holds execution settings for an agent node.
type AgentSpec struct {
	Prompt          string
	Provider        string
	Model           string
	Plan            bool
	Trust           bool
	SystemPrompt    string
	AllowedTools    []string
	DisallowedTools []string
	MaxTurns        int
	MCPConfig       string
	Passthrough     []string
}

// CmdSpec holds execution settings for a cmd node.
type CmdSpec struct {
	Command string
	Shell   string
	Env     []string
}
