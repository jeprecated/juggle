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
