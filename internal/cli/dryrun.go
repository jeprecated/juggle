package cli

import (
	"fmt"
	"io"
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
	var prompt string
	if len(cfg.Watch) > 0 {
		prompt = BuildWatchPrompt("<task file contents>", cfg.Content, "<task-file>", 1, cfg.Iterations)
	} else {
		prompt = BuildPrompt(cfg.Content, 1, cfg.Iterations)
	}
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
