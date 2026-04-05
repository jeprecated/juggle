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

// resolveFile reads a file reference, trying JUGGLE_PROMPTS fallbacks for bare names.
// Fallback chain: literal path (raw) → $JUGGLE_PROMPTS/name (stripped) →
// $JUGGLE_PROMPTS/name.md (stripped) → alias scan (stripped) → error.
func resolveFile(name string) ([]byte, error) {
	// Try literal path first — returned raw, no frontmatter stripping
	data, err := os.ReadFile(name)
	if err == nil {
		return data, nil
	}
	origErr := err

	// Only try JUGGLE_PROMPTS for bare names (no path separator)
	if strings.Contains(name, "/") {
		return nil, fmt.Errorf("failed to read %s: %w", name, origErr)
	}

	promptsDir := os.Getenv("JUGGLE_PROMPTS")
	if promptsDir == "" {
		return nil, fmt.Errorf("failed to read %s: %w", name, origErr)
	}

	// Try $JUGGLE_PROMPTS/name
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

	// Scan for alias match in $JUGGLE_PROMPTS
	body, err := resolveByAlias(name, promptsDir)
	if err != nil {
		return nil, err
	}
	if body != nil {
		return body, nil
	}

	return nil, fmt.Errorf("failed to read %s (also tried %s): %w", name, promptsDir, origErr)
}

// resolveByAlias scans all files in promptsDir for an alias matching name (case-insensitive).
// Returns the frontmatter-stripped body of the matching file, nil if no match,
// or an error if two files declare the same alias.
func resolveByAlias(name, promptsDir string) ([]byte, error) {
	lower := strings.ToLower(name)

	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return nil, nil
	}

	// alias → filename for collision detection
	aliasOwners := make(map[string]string)
	var matchFile string

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
		for _, alias := range fm.Aliases {
			aliasLower := strings.ToLower(alias)
			if prev, exists := aliasOwners[aliasLower]; exists {
				return nil, fmt.Errorf("alias %q declared by both %s and %s", alias, prev, entry.Name())
			}
			aliasOwners[aliasLower] = entry.Name()
			if aliasLower == lower {
				matchFile = entry.Name()
			}
		}
	}

	if matchFile == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filepath.Join(promptsDir, matchFile))
	if err != nil {
		return nil, err
	}
	_, body := parseFrontmatter(data)
	return body, nil
}
