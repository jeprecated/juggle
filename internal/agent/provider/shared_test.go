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
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false)

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
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false)

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
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false)

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
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false)

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
		streamJSONOutput(strings.NewReader(input), &buf, &out, acc, false)

		output := out.String()
		count := strings.Count(output, "[Tool: Read]")
		if count != 1 {
			t.Errorf("expected 1 [Tool: Read] (shared dedup), got %d in %q", count, output)
		}
	})
}
