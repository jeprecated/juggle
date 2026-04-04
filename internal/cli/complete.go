package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// completeArgs provides shell completion for positional arguments.
// When an arg starts with @, it suggests files from $JUGGLE_PROMPTS.
func completeArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !strings.HasPrefix(toComplete, "@") {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	promptsDir := os.Getenv("JUGGLE_PROMPTS")
	if promptsDir == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	partial := strings.ToLower(toComplete[1:]) // strip @ for matching

	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	seen := make(map[string]struct{})
	var completions []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Match against name without extension for convenience
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if strings.HasPrefix(strings.ToLower(name), partial) || strings.HasPrefix(strings.ToLower(base), partial) {
			if _, dup := seen[base]; !dup {
				seen[base] = struct{}{}
				completions = append(completions, "@"+base)
			}
		}
	}

	return completions, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}
