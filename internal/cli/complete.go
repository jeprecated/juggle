package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// completeArgs provides shell completion for positional arguments.
// When an arg starts with @, it suggests files from $JUGGLE_PROMPTS (recursively,
// with subdirectory paths prefixed) and any subdirectory whose name contains
// "prompts" found under the working directory.
// Aliases declared in frontmatter are also suggested with a source hint.
// Bare partial (no /) matches by base filename. Partial with / matches full path.
func completeArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !strings.HasPrefix(toComplete, "@") {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	partial := strings.ToLower(toComplete[1:]) // strip @ for matching
	seen := make(map[string]struct{})
	var completions []string

	// matchesPartial returns true if the relative path (e.g. "workflows/fix")
	// matches the partial. If partial has no /, match by base filename (fuzzy).
	// If partial has /, match by full relative path prefix.
	matchesPartial := func(rel string) bool {
		if strings.Contains(partial, "/") {
			return strings.HasPrefix(strings.ToLower(rel), partial)
		}
		// Bare partial: match against the base filename
		base := filepath.Base(rel)
		baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))
		return strings.HasPrefix(strings.ToLower(base), partial) ||
			strings.HasPrefix(strings.ToLower(baseNoExt), partial)
	}

	// addFromPromptsDirRecursive walks promptsDir recursively and adds completions
	// using relative paths. mdOnly restricts to .md files.
	addFromPromptsDirRecursive := func(promptsDir string, mdOnly bool) {
		_ = filepath.WalkDir(promptsDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != promptsDir && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				return nil
			}
			if mdOnly && !strings.EqualFold(filepath.Ext(name), ".md") {
				return nil
			}
			rel, rerr := filepath.Rel(promptsDir, path)
			if rerr != nil {
				return nil
			}
			// Use path-separator-normalized forward slashes for completions
			rel = filepath.ToSlash(rel)
			baseNoExt := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(filepath.Base(rel)))
			// Dedup key: use the path without extension
			relNoExt := strings.TrimSuffix(rel, filepath.Ext(rel))
			if !matchesPartial(rel) {
				return nil
			}
			if _, dup := seen[relNoExt]; !dup {
				seen[relNoExt] = struct{}{}
				_ = baseNoExt
				completions = append(completions, "@"+relNoExt)
			}
			return nil
		})
	}

	// addFromDir lists files in a single directory (non-recursive), used for
	// cwd-local prompt dirs (existing behaviour, not changed to recursive).
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

	// Search $JUGGLE_PROMPTS recursively
	promptsDir := os.Getenv("JUGGLE_PROMPTS")
	if promptsDir != "" {
		addFromPromptsDirRecursive(promptsDir, false)
		// Also add alias completions from JUGGLE_PROMPTS frontmatter (recursively)
		addAliasCompletions(promptsDir, partial, seen, &completions)
	}

	// Search subdirs containing "prompts" in their name under cwd (non-recursive per dir)
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

// addAliasCompletions walks promptsDir recursively for files with aliases in
// frontmatter and appends matching alias completions (with source hint) to
// completions. An alias is not added if its slot is already in seen.
// Hidden directories are skipped.
func addAliasCompletions(promptsDir, partial string, seen map[string]struct{}, completions *[]string) {
	_ = filepath.WalkDir(promptsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != promptsDir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		filePath := path
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}
		fm, _ := parseFrontmatter(data)
		if len(fm.Aliases) == 0 {
			return nil
		}
		rel, rerr := filepath.Rel(promptsDir, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		base := strings.TrimSuffix(rel, filepath.Ext(rel))
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
		return nil
	})
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
