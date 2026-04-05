package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// completeArgs provides shell completion for positional arguments.
// When an arg starts with @, it suggests files from $JUGGLE_PROMPTS and any
// subdirectory whose name contains "prompts" found under the working directory.
func completeArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !strings.HasPrefix(toComplete, "@") {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	partial := strings.ToLower(toComplete[1:]) // strip @ for matching
	seen := make(map[string]struct{})
	var completions []string

	addFromDir := func(dir string, mdOnly bool) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if mdOnly && !strings.EqualFold(filepath.Ext(name), ".md") {
				continue
			}
			base := strings.TrimSuffix(name, filepath.Ext(name))
			if strings.HasPrefix(strings.ToLower(name), partial) || strings.HasPrefix(strings.ToLower(base), partial) {
				if _, dup := seen[base]; !dup {
					seen[base] = struct{}{}
					completions = append(completions, "@"+base)
				}
			}
		}
	}

	// Search $JUGGLE_PROMPTS (all files, existing behaviour)
	if promptsDir := os.Getenv("JUGGLE_PROMPTS"); promptsDir != "" {
		addFromDir(promptsDir, false)
	}

	// Search subdirs containing "prompts" in their name under cwd
	if cwd, err := os.Getwd(); err == nil {
		for _, dir := range findPromptDirs(cwd) {
			addFromDir(dir, true)
		}
	}

	if len(completions) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completions, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}

// findPromptDirs walks root and returns all subdirectory paths whose base name
// contains "prompts" (case-insensitive). Hidden directories are skipped entirely.
func findPromptDirs(root string) []string {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		if strings.Contains(strings.ToLower(name), "prompts") {
			dirs = append(dirs, path)
			return filepath.SkipDir // don't recurse into matched dir
		}
		return nil
	})
	return dirs
}
