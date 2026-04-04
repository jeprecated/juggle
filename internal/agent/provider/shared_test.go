package provider

import (
	"strings"
	"testing"
)

func TestStreamJSONOutput_DedupToolMarkers(t *testing.T) {
	t.Run("consecutive identical tool_use deduplicated", func(t *testing.T) {
		// Two consecutive Read tool uses
		input := `{"type":"assistant","message":{"id":"1","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Read"}]}}
{"type":"assistant","message":{"id":"2","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Read"}]}}
`
		var buf strings.Builder
		var out strings.Builder
		acc := NewStreamAccumulator()
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false, false)

		output := out.String()
		count := strings.Count(output, "[Tool: Read]")
		if count != 1 {
			t.Errorf("expected 1 [Tool: Read], got %d in %q", count, output)
		}
	})

	t.Run("different tools print separately", func(t *testing.T) {
		input := `{"type":"assistant","message":{"id":"1","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Read"}]}}
{"type":"assistant","message":{"id":"2","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Write"}]}}
`
		var buf strings.Builder
		var out strings.Builder
		acc := NewStreamAccumulator()
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false, false)

		output := out.String()
		if !strings.Contains(output, "[Tool: Read]") {
			t.Error("missing [Tool: Read]")
		}
		if !strings.Contains(output, "[Tool: Write]") {
			t.Error("missing [Tool: Write]")
		}
	})

	t.Run("text between tools resets dedup", func(t *testing.T) {
		input := `{"type":"assistant","message":{"id":"1","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Read"}]}}
{"type":"assistant","message":{"id":"2","model":"m","role":"assistant","content":[{"type":"text","text":"some output"}]}}
{"type":"assistant","message":{"id":"3","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Read"}]}}
`
		var buf strings.Builder
		var out strings.Builder
		acc := NewStreamAccumulator()
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false, false)

		output := out.String()
		count := strings.Count(output, "[Tool: Read]")
		if count != 2 {
			t.Errorf("expected 2 [Tool: Read] (reset by text), got %d in %q", count, output)
		}
	})

	t.Run("system tool events deduplicated", func(t *testing.T) {
		input := `{"type":"system","subtype":"tool_use","tool_name":"Bash"}
{"type":"system","subtype":"tool_use","tool_name":"Bash"}
`
		var buf strings.Builder
		var out strings.Builder
		acc := NewStreamAccumulator()
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false, false)

		output := out.String()
		count := strings.Count(output, "[Tool: Bash]")
		if count != 1 {
			t.Errorf("expected 1 [Tool: Bash], got %d in %q", count, output)
		}
	})

	t.Run("system and assistant tool events share dedup state", func(t *testing.T) {
		input := `{"type":"assistant","message":{"id":"1","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Read"}]}}
{"type":"system","subtype":"tool_use","tool_name":"Read"}
`
		var buf strings.Builder
		var out strings.Builder
		acc := NewStreamAccumulator()
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false, false)

		output := out.String()
		count := strings.Count(output, "[Tool: Read]")
		if count != 1 {
			t.Errorf("expected 1 [Tool: Read] (shared dedup), got %d in %q", count, output)
		}
	})
}

func TestStreamJSONOutput_Verbose(t *testing.T) {
	t.Run("verbose shows bash command", func(t *testing.T) {
		input := `{"type":"assistant","message":{"id":"1","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}
`
		var buf, out strings.Builder
		acc := NewStreamAccumulator()
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false, true)

		output := out.String()
		if !strings.Contains(output, "[Tool: Bash] go test ./...") {
			t.Errorf("expected verbose bash output, got %q", output)
		}
	})

	t.Run("verbose shows file path for Read", func(t *testing.T) {
		input := `{"type":"assistant","message":{"id":"1","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"/tmp/foo.go"}}]}}
`
		var buf, out strings.Builder
		acc := NewStreamAccumulator()
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false, true)

		output := out.String()
		if !strings.Contains(output, "[Tool: Read] /tmp/foo.go") {
			t.Errorf("expected verbose read output, got %q", output)
		}
	})

	t.Run("non-verbose hides tool input", func(t *testing.T) {
		input := `{"type":"assistant","message":{"id":"1","model":"m","role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}
`
		var buf, out strings.Builder
		acc := NewStreamAccumulator()
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false, false)

		output := out.String()
		if !strings.Contains(output, "[Tool: Bash]") {
			t.Errorf("expected tool marker, got %q", output)
		}
		if strings.Contains(output, "go test") {
			t.Errorf("non-verbose should not show command, got %q", output)
		}
	})
}

func TestFormatToolInput(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		input    any
		expected string
	}{
		{"bash command", "Bash", map[string]interface{}{"command": "ls -la"}, "ls -la"},
		{"read path", "Read", map[string]interface{}{"file_path": "/a/b.go"}, "/a/b.go"},
		{"write path", "Write", map[string]interface{}{"file_path": "/a/b.go", "content": "lots of text"}, "/a/b.go"},
		{"edit path", "Edit", map[string]interface{}{"file_path": "/a/b.go"}, "/a/b.go"},
		{"grep pattern and path", "Grep", map[string]interface{}{"pattern": "TODO", "path": "./src"}, `"TODO" in ./src`},
		{"grep with glob", "Grep", map[string]interface{}{"pattern": "func", "glob": "*.go"}, `"func" in *.go`},
		{"glob pattern", "Glob", map[string]interface{}{"pattern": "**/*.go"}, "**/*.go"},
		{"glob with path", "Glob", map[string]interface{}{"pattern": "*.ts", "path": "./src"}, "*.ts in ./src"},
		{"nil input", "Bash", nil, ""},
		{"empty map", "Bash", map[string]interface{}{}, ""},
		{"wrong type", "Bash", "not a map", ""},
		{"unknown tool", "CustomTool", map[string]interface{}{"key": "val"}, `{"key":"val"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatToolInput(tt.tool, tt.input)
			if got != tt.expected {
				t.Errorf("formatToolInput(%q, %v) = %q, want %q", tt.tool, tt.input, got, tt.expected)
			}
		})
	}
}
