package cli

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/ohare93/juggle/internal/agent"
)

// resolveContexts processes --context flag values.
// Values starting with @ are treated as file paths (contents are read).
// All other values are used as-is.
func resolveContexts(values []string) ([]string, error) {
	resolved := make([]string, 0, len(values))
	for _, v := range values {
		if strings.HasPrefix(v, "@") {
			path := v[1:]
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read context file %s: %w", path, err)
			}
			resolved = append(resolved, string(data))
		} else {
			resolved = append(resolved, v)
		}
	}
	return resolved, nil
}

// manualSessionID generates a deterministic session ID from context values.
// Contexts are sorted before hashing so order doesn't matter.
func manualSessionID(contexts []string) string {
	sorted := make([]string, len(contexts))
	copy(sorted, contexts)
	sort.Strings(sorted)
	h := sha256.Sum256([]byte(strings.Join(sorted, "\x00")))
	return fmt.Sprintf("manual-%x", h[:3])
}

// watchSessionID generates a deterministic session ID from a directory path.
func watchSessionID(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("watch-%x", h[:3])
}

// manualPromptData holds template data for manual mode prompts.
type manualPromptData struct {
	Contexts  []string
	Iteration int
}

// watchPromptData holds template data for watch mode prompts.
type watchPromptData struct {
	TaskContents string
	Contexts     []string
	Iteration    int
}

// generateManualPrompt renders the manual mode prompt template.
func generateManualPrompt(contexts []string, iteration int) (string, error) {
	tmpl, err := template.New("manual").Parse(agent.GetManualPromptTemplate())
	if err != nil {
		return "", fmt.Errorf("failed to parse manual prompt template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, manualPromptData{
		Contexts:  contexts,
		Iteration: iteration,
	})
	if err != nil {
		return "", fmt.Errorf("failed to render manual prompt: %w", err)
	}

	return buf.String(), nil
}

// generateWatchPrompt renders the watch mode prompt template.
func generateWatchPrompt(taskContents string, contexts []string, iteration int) (string, error) {
	tmpl, err := template.New("watch").Parse(agent.GetWatchPromptTemplate())
	if err != nil {
		return "", fmt.Errorf("failed to parse watch prompt template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, watchPromptData{
		TaskContents: taskContents,
		Contexts:     contexts,
		Iteration:    iteration,
	})
	if err != nil {
		return "", fmt.Errorf("failed to render watch prompt: %w", err)
	}

	return buf.String(), nil
}

// runWatchLoop processes task files from a directory. Implemented in Task 7.
func runWatchLoop(config AgentLoopConfig) error {
	return fmt.Errorf("watch mode not yet implemented")
}
