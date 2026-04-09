package cli

import "github.com/ohare93/juggle/internal/pipeline"

// AdaptConfigToPipeline converts a Config's lifecycle hook flags into an
// equivalent Pipeline representation. The main agent prompt (Content) becomes
// the EventLoopBody node, and hook flags become surrounding nodes.
//
// Failure policy mapping:
//   - AgentPre / AgentPost: FailurePolicyStop (stop run on failure)
//   - AgentBefore / CmdBefore: FailurePolicyStop (stop run on failure)
//     Note: original flags skip the current iteration on failure; in pipeline
//     terms, a failure at EventLoopStart stops the whole run.
//   - AgentAfter / CmdAfter: FailurePolicyContinue (log warning, continue)
//
// StopWhen has no direct pipeline equivalent and is not translated.
func AdaptConfigToPipeline(cfg Config) *pipeline.Pipeline {
	var nodes []pipeline.Node

	// AgentPre → runs once before all iterations.
	if cfg.AgentPre != "" {
		nodes = append(nodes, pipeline.Node{
			Name:      "agent-pre",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventRunStart,
			OnFailure: pipeline.FailurePolicyStop,
			Agent: &pipeline.AgentSpec{
				Prompt:   cfg.AgentPre,
				Provider: cfg.Provider,
				Model:    cfg.Model,
			},
		})
	}

	// CmdBefore → runs before each iteration, before AgentBefore.
	if cfg.CmdBefore != "" {
		nodes = append(nodes, pipeline.Node{
			Name:      "cmd-before",
			Kind:      pipeline.NodeKindCmd,
			Event:     pipeline.EventLoopStart,
			OnFailure: pipeline.FailurePolicyStop,
			Cmd: &pipeline.CmdSpec{
				Command: cfg.CmdBefore,
			},
		})
	}

	// AgentBefore → runs before each iteration main body.
	if cfg.AgentBefore != "" {
		nodes = append(nodes, pipeline.Node{
			Name:      "agent-before",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventLoopStart,
			OnFailure: pipeline.FailurePolicyStop,
			Agent: &pipeline.AgentSpec{
				Prompt:   cfg.AgentBefore,
				Provider: cfg.Provider,
				Model:    cfg.Model,
			},
		})
	}

	// Main agent → the core iteration task.
	nodes = append(nodes, pipeline.Node{
		Name:  "main",
		Kind:  pipeline.NodeKindAgent,
		Event: pipeline.EventLoopBody,
		Agent: &pipeline.AgentSpec{
			Prompt:          cfg.Content,
			Provider:        cfg.Provider,
			Model:           cfg.Model,
			Plan:            cfg.Plan,
			Trust:           cfg.Trust,
			SystemPrompt:    cfg.SystemPrompt,
			AllowedTools:    cfg.AllowedTools,
			DisallowedTools: cfg.DisallowedTools,
			MaxTurns:        cfg.MaxTurns,
			MCPConfig:       cfg.MCPConfig,
			Passthrough:     cfg.PassthroughArgs,
		},
	})

	// AgentAfter → runs after each iteration; failure is non-fatal.
	if cfg.AgentAfter != "" {
		nodes = append(nodes, pipeline.Node{
			Name:      "agent-after",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventLoopEnd,
			OnFailure: pipeline.FailurePolicyContinue,
			Agent: &pipeline.AgentSpec{
				Prompt:   cfg.AgentAfter,
				Provider: cfg.Provider,
				Model:    cfg.Model,
			},
		})
	}

	// CmdAfter → runs after each iteration; failure is non-fatal.
	if cfg.CmdAfter != "" {
		nodes = append(nodes, pipeline.Node{
			Name:      "cmd-after",
			Kind:      pipeline.NodeKindCmd,
			Event:     pipeline.EventLoopEnd,
			OnFailure: pipeline.FailurePolicyContinue,
			Cmd: &pipeline.CmdSpec{
				Command: cfg.CmdAfter,
			},
		})
	}

	// AgentPost → runs once after all iterations.
	if cfg.AgentPost != "" {
		nodes = append(nodes, pipeline.Node{
			Name:      "agent-post",
			Kind:      pipeline.NodeKindAgent,
			Event:     pipeline.EventRunEnd,
			OnFailure: pipeline.FailurePolicyStop,
			Agent: &pipeline.AgentSpec{
				Prompt:   cfg.AgentPost,
				Provider: cfg.Provider,
				Model:    cfg.Model,
			},
		})
	}

	iterations := cfg.Iterations
	if iterations == 0 {
		iterations = 1
	}

	return &pipeline.Pipeline{
		Iterations: iterations,
		Nodes:      nodes,
	}
}
