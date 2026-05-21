package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/jeprecated/juggle/internal/agent/provider"
)

// printDryRun writes a full dry-run preview to w showing every configured
// section in execution order. Sections that are not configured are omitted.
// Headers use ANSI color when w is a TTY and NO_COLOR is unset.
func printDryRun(cfg Config, w io.Writer) {
	color := isColorEnabled(w)

	section := func(name, body string) {
		header := colorizeHeading(fmt.Sprintf("=== [%s] ===", name), color)
		fmt.Fprintf(w, "%s\n%s\n\n", header, body)
	}

	prompt := dryRunPrompt(cfg)
	if body, ok := providerPreviewBody(cfg, prompt, color); ok {
		section("provider command", body)
	}

	// 1. agent-pre
	if cfg.AgentPre != "" {
		section("agent-pre", cfg.AgentPre)
	}

	// 2. cmd-before
	if cfg.CmdBefore != "" {
		section("cmd-before", cfg.CmdBefore)
	}

	// 3. agent-before
	if cfg.AgentBefore != "" {
		section("agent-before", cfg.AgentBefore)
	}

	// 4. main prompt
	section("main prompt", prompt)

	// 5. agent-after
	if cfg.AgentAfter != "" {
		section("agent-after", cfg.AgentAfter)
	}

	// 6. cmd-after
	if cfg.CmdAfter != "" {
		section("cmd-after", cfg.CmdAfter)
	}

	// 7. stop-when
	if cfg.StopWhen != "" {
		section("stop-when", cfg.StopWhen)
	}

	// 8. agent-post
	if cfg.AgentPost != "" {
		section("agent-post", cfg.AgentPost)
	}

	// 9. session hooks
	if len(cfg.Hooks) > 0 {
		body := ""
		for _, h := range cfg.Hooks {
			body += h + "\n"
		}
		section("hooks", body)
	}
}

func providerPreviewBody(cfg Config, prompt string, color bool) (string, bool) {
	p, err := dryRunProvider(cfg)
	if err != nil {
		return fmt.Sprintf("error: %v", err), true
	}

	opts := buildRunOptions(cfg, prompt)
	preview, err := provider.PreviewCommand(p, opts)
	if err != nil {
		return fmt.Sprintf("error: %v", err), true
	}

	cmdArgs := preview.Args
	showPromptBlock := preview.Prompt != ""
	if showPromptBlock && !preview.PromptStdin {
		cmdArgs = replacePromptArg(cmdArgs, preview.Prompt, "<prompt>")
	}

	var lines []string
	lines = append(lines, colorizeExamples(shellJoin(preview.Binary, cmdArgs...), color))
	if preview.PromptStdin {
		lines = append(lines, "")
		lines = append(lines, "stdin:")
		lines = append(lines, preview.Prompt)
	} else if showPromptBlock {
		lines = append(lines, "")
		lines = append(lines, "prompt:")
		lines = append(lines, preview.Prompt)
	}
	return strings.Join(lines, "\n"), true
}

func dryRunPrompt(cfg Config) string {
	if len(cfg.Watch) > 0 {
		return BuildWatchPrompt("<task file contents>", cfg.Content, "<task-file>", 1, cfg.Iterations)
	}
	return BuildPrompt(cfg.Content, 1, cfg.Iterations)
}

func printVerboseProviderCommand(cfg Config, prompt string) {
	if cfg.Stderr == nil {
		return
	}
	if !cfg.Verbose {
		return
	}
	color := isColorEnabled(cfg.Stderr)
	body, ok := providerPreviewBody(cfg, prompt, color)
	if !ok {
		return
	}
	header := colorizeHeading("  provider command:", color)
	fmt.Fprintf(cfg.Stderr, "%s\n%s\n", header, indentBlock(body, "    "))
}

func dryRunProvider(cfg Config) (provider.Provider, error) {
	providerName := cfg.Provider
	if cfg.AgentCmd != "" && providerName == "claude" {
		providerName = "custom"
	}

	if provider.Type(providerName) == provider.TypeCustom {
		if cfg.AgentCmd == "" {
			return nil, fmt.Errorf("--provider custom requires --agent-cmd")
		}
		return provider.GetCustom(cfg.AgentCmd), nil
	}
	return provider.Get(provider.Type(providerName)), nil
}

var shellSafeArgRe = regexp.MustCompile(`^[A-Za-z0-9_./:=,@%+<>-]+$`)

func shellJoin(binary string, args ...string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, shellArg(binary))
	for _, arg := range args {
		parts = append(parts, shellArg(arg))
	}
	return strings.Join(parts, " ")
}

func shellArg(arg string) string {
	if arg == "" {
		return "''"
	}
	if shellSafeArgRe.MatchString(arg) {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

func replacePromptArg(args []string, prompt, placeholder string) []string {
	out := append([]string(nil), args...)
	for i, arg := range out {
		if arg == prompt {
			out[i] = placeholder
			break
		}
	}
	return out
}

func indentBlock(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
