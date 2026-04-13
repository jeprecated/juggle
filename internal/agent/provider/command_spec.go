package provider

import (
	"os"
	"os/exec"
	"strings"
)

// commandSpec describes how juggle invokes a provider CLI for one run mode.
// Built-in providers act like hard-coded custom providers by translating
// provider-agnostic RunOptions into this explicit command shape.
type commandSpec struct {
	Binary           string
	Args             []string
	Prompt           string
	PromptStdin      bool
	CommandOverride  string // when set, replaces Binary via the user's login shell
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

// commandForSpec builds an exec.Cmd from a commandSpec. When CommandOverride
// is set, the command is executed through the user's login shell ($SHELL -ic)
// so that shell aliases, functions, and environment setup are available.
func commandForSpec(spec commandSpec) *exec.Cmd {
	if spec.CommandOverride != "" {
		return shellCommand(spec.CommandOverride, spec.Args)
	}
	return exec.Command(spec.Binary, spec.Args...)
}

// shellCommand runs "name args..." through the user's shell so that
// shell aliases and functions resolve naturally.
//
// We avoid -i (interactive) because providers pipe stdin/stdout, and an
// interactive shell without a TTY will hang waiting for job control.
// Instead we source the shell's rc file explicitly so that functions and
// aliases defined there are available.
func shellCommand(name string, args []string) *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	// Build the actual command string.
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, name)
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	cmdStr := strings.Join(parts, " ")

	// Source the appropriate rc file for the user's shell so that aliases
	// and functions are available, then exec the command.
	var script string
	base := shellBasename(shell)
	switch base {
	case "zsh":
		// ZDOTDIR defaults to $HOME; .zshrc is where interactive functions live.
		// Stub out completion functions (compdef, compinit, etc.) that are only
		// available in interactive shells after compinit runs.
		script = "compdef() { :; }; compinit() { :; }; " +
			"[ -f \"${ZDOTDIR:-$HOME}/.zshenv\" ] && . \"${ZDOTDIR:-$HOME}/.zshenv\"; " +
			"[ -f \"${ZDOTDIR:-$HOME}/.zshrc\" ] && . \"${ZDOTDIR:-$HOME}/.zshrc\"; " +
			cmdStr
	case "bash":
		script = "[ -f \"$HOME/.bashrc\" ] && . \"$HOME/.bashrc\"; " + cmdStr
	case "fish":
		// Fish config is always sourced, -c is sufficient.
		return exec.Command(shell, "-c", cmdStr)
	default:
		// Unknown shell — try sourcing common rc files.
		script = "[ -f \"$HOME/.profile\" ] && . \"$HOME/.profile\"; " + cmdStr
	}

	return exec.Command(shell, "-c", script)
}

// shellBasename returns the last path component of a shell path, e.g.
// "/usr/bin/zsh" -> "zsh", "/nix/store/.../bin/zsh-5.9" -> "zsh".
func shellBasename(shell string) string {
	// Take everything after the last slash.
	base := shell
	if i := strings.LastIndex(shell, "/"); i >= 0 {
		base = shell[i+1:]
	}
	// Strip version suffixes like "zsh-5.9" -> "zsh".
	if i := strings.Index(base, "-"); i > 0 {
		base = base[:i]
	}
	return base
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// If the string contains no special characters, return as-is.
	if !strings.ContainsAny(s, " \t\n'\"\\$`!#&|;(){}[]<>?*~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
