package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// hookSpec represents a parsed --hook flag entry.
type hookSpec struct {
	Event   string
	Command string
}

// parseHookFlag parses a --hook flag value "EVENT:CMD".
// Splits on the first colon only. Returns error if no colon present.
// If CMD starts with '@', resolves the file reference for its contents.
func parseHookFlag(s string) (hookSpec, error) {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return hookSpec{}, fmt.Errorf("--hook %q: expected EVENT:CMD format", s)
	}
	event := s[:idx]
	cmd := s[idx+1:]

	if strings.HasPrefix(cmd, "@") {
		data, err := resolveFile(cmd[1:])
		if err != nil {
			return hookSpec{}, fmt.Errorf("--hook %q: %w", s, err)
		}
		cmd = strings.TrimSpace(string(data))
	}

	return hookSpec{Event: event, Command: cmd}, nil
}

// buildHooksJSON builds a Claude Code settings JSON object from hook specs and optional base JSON.
// baseJSON is the raw content of a --hooks-file (merged as base settings).
// specs from --hook flags are appended to (not replacing) the base.
func buildHooksJSON(specs []hookSpec, baseJSON []byte) ([]byte, error) {
	// Start with base settings (from --hooks-file) or empty map
	var settings map[string]interface{}
	if len(baseJSON) > 0 {
		if err := json.Unmarshal(baseJSON, &settings); err != nil {
			return nil, fmt.Errorf("parsing hooks file: %w", err)
		}
	} else {
		settings = make(map[string]interface{})
	}

	if len(specs) == 0 {
		return json.Marshal(settings)
	}

	// Get or create the "hooks" map
	var hooks map[string]interface{}
	if hooksRaw, ok := settings["hooks"]; ok {
		h, ok := hooksRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("hooks file: 'hooks' field is not an object")
		}
		hooks = h
	} else {
		hooks = make(map[string]interface{})
		settings["hooks"] = hooks
	}

	// Group specs by event (preserve order for determinism)
	seen := make(map[string]bool)
	var events []string
	byEvent := make(map[string][]string)
	for _, spec := range specs {
		byEvent[spec.Event] = append(byEvent[spec.Event], spec.Command)
		if !seen[spec.Event] {
			seen[spec.Event] = true
			events = append(events, spec.Event)
		}
	}

	// Append hook entries for each event
	for _, event := range events {
		var entries []interface{}
		// Preserve existing entries from base JSON
		if existing, ok := hooks[event]; ok {
			if arr, ok := existing.([]interface{}); ok {
				entries = arr
			}
		}
		for _, cmd := range byEvent[event] {
			entry := map[string]interface{}{
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": cmd,
					},
				},
			}
			entries = append(entries, entry)
		}
		hooks[event] = entries
	}

	return json.Marshal(settings)
}

// buildHooksSettingsFile creates a temp JSON file with merged hooks settings.
// Returns the file path and a cleanup function that deletes it.
// When no hooks are configured (empty hookFlags and empty hooksFile), returns ("", no-op, nil).
func buildHooksSettingsFile(hookFlags []string, hooksFile string) (string, func(), error) {
	if len(hookFlags) == 0 && hooksFile == "" {
		return "", func() {}, nil
	}

	// Parse --hook flags
	specs := make([]hookSpec, 0, len(hookFlags))
	for _, flag := range hookFlags {
		spec, err := parseHookFlag(flag)
		if err != nil {
			return "", func() {}, err
		}
		specs = append(specs, spec)
	}

	// Read --hooks-file if provided
	var baseJSON []byte
	if hooksFile != "" {
		data, err := os.ReadFile(hooksFile)
		if err != nil {
			return "", func() {}, fmt.Errorf("--hooks-file: %w", err)
		}
		baseJSON = data
	}

	// Build merged JSON
	data, err := buildHooksJSON(specs, baseJSON)
	if err != nil {
		return "", func() {}, err
	}

	// Write to temp file
	f, err := os.CreateTemp("", "juggle-hooks-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("creating hooks settings file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("writing hooks settings file: %w", err)
	}

	path := f.Name()
	return path, func() { os.Remove(path) }, nil
}
