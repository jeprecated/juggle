package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ReadStdin reads all content from r if isTTY is false.
// Returns trimmed content, or "" if isTTY is true or content is blank.
func ReadStdin(r io.Reader, isTTY bool) (string, error) {
	if isTTY {
		return "", nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// ResolveArgs processes positional arguments.
// Args starting with @ are file paths (contents read and returned).
// Bare @name (no /) checks $JUGGLE_PROMPTS/<name> and <name>.md as fallbacks,
// then scans for alias declarations in frontmatter. Frontmatter is stripped
// from files resolved via JUGGLE_PROMPTS.
// Explicit @subdir/name looks up $JUGGLE_PROMPTS/subdir/name as well.
// All other args are returned as-is.
func ResolveArgs(args []string) ([]string, error) {
	resolved := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			name := arg[1:]
			data, err := resolveFile(name)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, string(data))
		} else {
			resolved = append(resolved, arg)
		}
	}
	return resolved, nil
}

// isLiteralPath reports whether name is an explicit filesystem path that should
// not fall back to JUGGLE_PROMPTS lookup. Names starting with /, ./, or ../ are
// treated as literal paths.
func isLiteralPath(name string) bool {
	return strings.HasPrefix(name, "/") ||
		strings.HasPrefix(name, "./") ||
		strings.HasPrefix(name, "../")
}

// resolveFile reads a file reference, trying JUGGLE_PROMPTS fallbacks.
// Fallback chain:
//   - literal path (raw, no frontmatter stripping)
//   - if name is a literal path (starts with /, ./, ../): error
//   - $JUGGLE_PROMPTS/name (stripped)
//   - $JUGGLE_PROMPTS/name.md (stripped, if no extension)
//   - if bare name (no /): recursive walk of subdirs, then alias scan
//   - error
func resolveFile(name string) ([]byte, error) {
	// Try literal path first — returned raw, no frontmatter stripping
	data, err := os.ReadFile(name)
	if err == nil {
		return data, nil
	}
	origErr := err

	// Names that look like explicit filesystem paths don't fall back to JUGGLE_PROMPTS
	if isLiteralPath(name) {
		return nil, fmt.Errorf("failed to read %s: %w", name, origErr)
	}

	promptsDir := os.Getenv("JUGGLE_PROMPTS")
	if promptsDir == "" {
		return nil, fmt.Errorf("failed to read %s: %w", name, origErr)
	}

	// Try $JUGGLE_PROMPTS/name (works for both bare names and explicit subdir paths)
	candidate := filepath.Join(promptsDir, name)
	data, err = os.ReadFile(candidate)
	if err == nil {
		_, body := parseFrontmatter(data)
		return body, nil
	}

	// Try $JUGGLE_PROMPTS/name.md (only if name doesn't already have an extension)
	if filepath.Ext(name) == "" {
		candidate = filepath.Join(promptsDir, name+".md")
		data, err = os.ReadFile(candidate)
		if err == nil {
			_, body := parseFrontmatter(data)
			return body, nil
		}
	}

	// For bare names (no /), also walk subdirectories and scan aliases
	if !strings.Contains(name, "/") {
		// Walk subdirectories recursively for bare name
		body, err := resolveNestedBare(name, promptsDir)
		if err != nil {
			return nil, err
		}
		if body != nil {
			return body, nil
		}

		// Scan for alias match across all files in $JUGGLE_PROMPTS (recursively)
		body, err = resolveByAlias(name, promptsDir)
		if err != nil {
			return nil, err
		}
		if body != nil {
			return body, nil
		}

		return nil, fmt.Errorf("failed to read %s (also tried %s): %w", name, promptsDir, origErr)
	}

	return nil, fmt.Errorf("failed to read %s: %w", name, origErr)
}

// resolveNestedBare walks subdirectories of promptsDir (not root level, which is
// already tried) looking for files matching name or name.md (case-sensitive).
// Returns error if multiple files match the same bare name. Skips hidden dirs.
func resolveNestedBare(name, promptsDir string) ([]byte, error) {
	var matches []string

	_ = filepath.WalkDir(promptsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == promptsDir {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip root-level files (already tried by caller) and hidden files
		rel, rerr := filepath.Rel(promptsDir, path)
		if rerr != nil || !strings.Contains(rel, string(filepath.Separator)) {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		filename := d.Name()
		basename := strings.TrimSuffix(filename, filepath.Ext(filename))
		if filename == name || (filepath.Ext(name) == "" && basename == name) {
			matches = append(matches, rel)
		}
		return nil
	})

	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) > 1 {
		for i, m := range matches {
			matches[i] = filepath.ToSlash(m)
		}
		return nil, fmt.Errorf("ambiguous name %q matches multiple files: %s", name, strings.Join(matches, ", "))
	}

	data, err := os.ReadFile(filepath.Join(promptsDir, matches[0]))
	if err != nil {
		return nil, err
	}
	_, body := parseFrontmatter(data)
	return body, nil
}

// resolveByAlias walks all files in promptsDir (recursively) for an alias matching
// name (case-insensitive). Returns the frontmatter-stripped body of the matching
// file, nil if no match, or an error if two files declare the same alias.
// Hidden directories are skipped.
func resolveByAlias(name, promptsDir string) ([]byte, error) {
	lower := strings.ToLower(name)

	// alias (lower) → relative path
	aliasOwners := make(map[string]string)
	var matchRelPath string
	var dupAlias, dupFile1, dupFile2 string

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
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(promptsDir, path)
		fm, _ := parseFrontmatter(data)
		for _, alias := range fm.Aliases {
			aliasLower := strings.ToLower(alias)
			if prev, exists := aliasOwners[aliasLower]; exists && dupAlias == "" {
				dupAlias = alias
				dupFile1 = prev
				dupFile2 = rel
			}
			aliasOwners[aliasLower] = rel
			if aliasLower == lower {
				matchRelPath = rel
			}
		}
		return nil
	})

	// Only error on duplicate if it's the alias we're looking for
	if dupAlias != "" && strings.ToLower(dupAlias) == lower {
		return nil, fmt.Errorf("alias %q declared by both %s and %s", dupAlias, dupFile1, dupFile2)
	}

	if matchRelPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filepath.Join(promptsDir, matchRelPath))
	if err != nil {
		return nil, err
	}
	_, body := parseFrontmatter(data)
	return body, nil
}
