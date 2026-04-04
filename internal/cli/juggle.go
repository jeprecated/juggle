package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

// SetVersion sets the version string (injected at build time).
func SetVersion(v string) { version = v }

var rootCmd = &cobra.Command{
	Use:   "juggle [prompt-content...]",
	Short: "Minimal agent loop runner",
	Long:  "Run an AI agent in a loop. All positional args are prompt content (strings or @file references).",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not yet implemented")
	},
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// BuildPrompt joins content with an iteration footer.
func BuildPrompt(content string, iteration, maxIterations int) string {
	return fmt.Sprintf("%s\n\n---\nThis is iteration %d of %s.\n", content, iteration, maxStr(maxIterations))
}

// BuildWatchPrompt wraps task file contents with content and footer.
func BuildWatchPrompt(taskContents, content, filename string, iteration, maxIterations int) string {
	return fmt.Sprintf("<task>\n%s\n</task>\n\n%s\n\n---\nThis is iteration %d of %s, processing %s.\n",
		taskContents, content, iteration, maxStr(maxIterations), filename)
}

func maxStr(max int) string {
	if max == 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", max)
}
