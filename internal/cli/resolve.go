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
// Bare @name (no /) checks $JUGGLE_PROMPTS/<name> and <name>.md as fallbacks.
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
// Fallback chain: literal path → $JUGGLE_PROMPTS/name → $JUGGLE_PROMPTS/name.md → error.
func resolveFile(name string) ([]byte, error) {
	// Try literal path first
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
		return data, nil
	}

	// Try $JUGGLE_PROMPTS/name.md (only if name doesn't already have an extension)
	if filepath.Ext(name) == "" {
		candidate = filepath.Join(promptsDir, name+".md")
		data, err = os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("failed to read %s (also tried %s): %w", name, promptsDir, origErr)
}
