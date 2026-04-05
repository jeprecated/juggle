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
// Aliases declared in frontmatter are also suggested with a source hint.
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
	promptsDir := os.Getenv("JUGGLE_PROMPTS")
	if promptsDir != "" {
		addFromDir(promptsDir, false)
		// Also add alias completions from JUGGLE_PROMPTS frontmatter
		addAliasCompletions(promptsDir, partial, seen, &completions)
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

// addAliasCompletions scans promptsDir for files with aliases in frontmatter
// and appends matching alias completions (with source hint) to completions.
// An alias is not added if a base name already occupies that slot in seen.
func addAliasCompletions(promptsDir, partial string, seen map[string]struct{}, completions *[]string) {
	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		filePath := filepath.Join(promptsDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		fm, _ := parseFrontmatter(data)
		if len(fm.Aliases) == 0 {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		for _, alias := range fm.Aliases {
			if !strings.HasPrefix(strings.ToLower(alias), partial) {
				continue
			}
			// Skip if base name already provides this slot
			if _, exists := seen[alias]; exists {
				continue
			}
			seen[alias] = struct{}{}
			*completions = append(*completions, "@"+alias+"\t(→ "+base+")")
		}
	}
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
