package provider

import "os/exec"

// commandSpec describes how juggle invokes a provider CLI for one run mode.
// Built-in providers act like hard-coded custom providers by translating
// provider-agnostic RunOptions into this explicit command shape.
type commandSpec struct {
	Binary           string
	Args             []string
	Prompt           string
	PromptStdin      bool
	CommandOverride  string // when set, replaces Binary in the exec.Command call
}

func appendFlag(args []string, flag, value string) []string {
	if flag == "" {
		return args
	}
	if value != "" {
		return append(args, flag, value)
	}
	return append(args, flag)
}

func commandForSpec(spec commandSpec) *exec.Cmd {
	binary := spec.Binary
	if spec.CommandOverride != "" {
		binary = spec.CommandOverride
	}
	return exec.Command(binary, spec.Args...)
}
