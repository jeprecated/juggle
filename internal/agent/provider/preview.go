package provider

import "fmt"

// CommandPreview is a dry-run description of the exact provider command shape.
type CommandPreview struct {
	Binary      string
	Args        []string
	Prompt      string
	PromptStdin bool
}

// PreviewCommand resolves the exact command shape a provider would execute for opts.
func PreviewCommand(p Provider, opts RunOptions) (CommandPreview, error) {
	if p == nil {
		p = NewClaudeProvider()
	}

	spec, err := previewSpec(p, opts)
	if err != nil {
		return CommandPreview{}, err
	}

	binary := spec.Binary
	if spec.CommandOverride != "" {
		binary = spec.CommandOverride
	}
	return CommandPreview{
		Binary:      binary,
		Args:        append([]string(nil), spec.Args...),
		Prompt:      spec.Prompt,
		PromptStdin: spec.PromptStdin,
	}, nil
}

func previewSpec(p Provider, opts RunOptions) (commandSpec, error) {
	switch v := p.(type) {
	case *ClaudeProvider:
		if opts.Mode == ModeInteractive {
			return claudeInteractiveSpec(opts), nil
		}
		return claudeHeadlessSpec(opts), nil
	case *CodexProvider:
		if opts.Mode == ModeInteractive {
			return codexInteractiveSpec(opts), nil
		}
		return codexHeadlessSpec(opts), nil
	case *GeminiProvider:
		if opts.Mode == ModeInteractive {
			return geminiInteractiveSpec(opts), nil
		}
		return geminiHeadlessSpec(opts), nil
	case *OpenCodeProvider:
		if opts.Mode == ModeInteractive {
			return opencodeInteractiveSpec(opts), nil
		}
		return opencodeHeadlessSpec(opts), nil
	case *CopilotProvider:
		if opts.Mode == ModeInteractive {
			return copilotInteractiveSpec(opts), nil
		}
		return copilotHeadlessSpec(opts), nil
	case *CustomProvider:
		binary, args, cleanup, err := buildCustomCmd(v.agentCmd, opts)
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			return commandSpec{}, err
		}
		return commandSpec{Binary: binary, Args: args, Prompt: opts.Prompt, CommandOverride: opts.CommandOverride}, nil
	default:
		return commandSpec{}, fmt.Errorf("unsupported provider preview type %T", p)
	}
}
