package provider

import (
	"io"
	"strings"
	"testing"
)

func TestNewStreamAccumulator(t *testing.T) {
	sa := NewStreamAccumulator()

	if sa == nil {
		t.Fatal("NewStreamAccumulator returned nil")
	}
	if sa.ActiveBlocks == nil {
		t.Error("ActiveBlocks map not initialized")
	}
	if sa.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", sa.InputTokens)
	}
	if sa.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", sa.OutputTokens)
	}
}

func TestStreamAccumulator_ProcessEvent_Assistant(t *testing.T) {
	sa := NewStreamAccumulator()

	event := &StreamEvent{
		Type: "assistant",
		Message: &AssistantMessage{
			ID:    "msg_123",
			Model: "claude-3-opus",
			Role:  "assistant",
			Content: []ContentBlock{
				{Type: "text", Text: "Hello, world!"},
			},
			Usage: &AssistantUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		},
	}

	err := sa.ProcessEvent(event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}

	if sa.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", sa.InputTokens)
	}
	if sa.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", sa.OutputTokens)
	}
	if sa.GetText() != "Hello, world!" {
		t.Errorf("GetText() = %q, want 'Hello, world!'", sa.GetText())
	}
}

func TestStreamAccumulator_ProcessEvent_AssistantToolUse(t *testing.T) {
	sa := NewStreamAccumulator()

	event := &StreamEvent{
		Type: "assistant",
		Message: &AssistantMessage{
			Content: []ContentBlock{
				{Type: "tool_use", Name: "bash", ID: "tool_123"},
			},
		},
	}

	err := sa.ProcessEvent(event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}

	if sa.CurrentTool != "bash" {
		t.Errorf("CurrentTool = %q, want 'bash'", sa.CurrentTool)
	}
}

func TestStreamAccumulator_ProcessEvent_AssistantThinking(t *testing.T) {
	sa := NewStreamAccumulator()

	event := &StreamEvent{
		Type: "assistant",
		Message: &AssistantMessage{
			Content: []ContentBlock{
				{Type: "thinking", Text: "Let me think about this..."},
			},
		},
	}

	err := sa.ProcessEvent(event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}

	if len(sa.ThinkingBlocks) != 1 {
		t.Fatalf("expected 1 thinking block, got %d", len(sa.ThinkingBlocks))
	}
	if sa.ThinkingBlocks[0] != "Let me think about this..." {
		t.Errorf("ThinkingBlocks[0] = %q, want 'Let me think about this...'", sa.ThinkingBlocks[0])
	}
}

func TestStreamAccumulator_ProcessEvent_Result(t *testing.T) {
	sa := NewStreamAccumulator()

	event := &StreamEvent{
		Type:   "result",
		Result: "Success",
		Usage: &ResultUsage{
			InputTokens:  200,
			OutputTokens: 100,
		},
		TotalCost: 0.05,
	}

	err := sa.ProcessEvent(event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}

	if sa.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200", sa.InputTokens)
	}
	if sa.OutputTokens != 100 {
		t.Errorf("OutputTokens = %d, want 100", sa.OutputTokens)
	}
}

func TestStreamAccumulator_ProcessEvent_SystemTool(t *testing.T) {
	sa := NewStreamAccumulator()

	event := &StreamEvent{
		Type:     "system",
		Subtype:  "tool_use",
		ToolName: "Read",
	}

	err := sa.ProcessEvent(event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}

	if sa.CurrentTool != "Read" {
		t.Errorf("CurrentTool = %q, want 'Read'", sa.CurrentTool)
	}
}

func TestStreamAccumulator_ProcessEvent_UnknownType(t *testing.T) {
	sa := NewStreamAccumulator()

	event := &StreamEvent{
		Type: "unknown_type",
	}

	err := sa.ProcessEvent(event)
	if err != nil {
		t.Errorf("ProcessEvent should not error on unknown type: %v", err)
	}
}

func TestJSONLinesParser_Next_SingleEvent(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}` + "\n"
	reader := strings.NewReader(input)
	parser := NewJSONLinesParser(reader)

	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != "assistant" {
		t.Errorf("Type = %q, want 'assistant'", event.Type)
	}
	if event.Message == nil {
		t.Fatal("Message is nil")
	}
	if len(event.Message.Content) != 1 {
		t.Fatalf("Content length = %d, want 1", len(event.Message.Content))
	}
	if event.Message.Content[0].Text != "Hello" {
		t.Errorf("Content[0].Text = %q, want 'Hello'", event.Message.Content[0].Text)
	}
}

func TestJSONLinesParser_Next_MultipleEvents(t *testing.T) {
	input := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[]}}
{"type":"result","result":"done"}
`
	reader := strings.NewReader(input)
	parser := NewJSONLinesParser(reader)

	// First event
	event1, err := parser.Next()
	if err != nil {
		t.Fatalf("First Next() returned error: %v", err)
	}
	if event1.Type != "system" {
		t.Errorf("First event type = %q, want 'system'", event1.Type)
	}

	// Second event
	event2, err := parser.Next()
	if err != nil {
		t.Fatalf("Second Next() returned error: %v", err)
	}
	if event2.Type != "assistant" {
		t.Errorf("Second event type = %q, want 'assistant'", event2.Type)
	}

	// Third event
	event3, err := parser.Next()
	if err != nil {
		t.Fatalf("Third Next() returned error: %v", err)
	}
	if event3.Type != "result" {
		t.Errorf("Third event type = %q, want 'result'", event3.Type)
	}

	// EOF
	_, err = parser.Next()
	if err != io.EOF {
		t.Errorf("Fourth Next() should return io.EOF, got %v", err)
	}
}

func TestJSONLinesParser_Next_SkipsEmptyLines(t *testing.T) {
	input := `
{"type":"system"}

{"type":"result"}

`
	reader := strings.NewReader(input)
	parser := NewJSONLinesParser(reader)

	// First event
	event1, err := parser.Next()
	if err != nil {
		t.Fatalf("First Next() returned error: %v", err)
	}
	if event1.Type != "system" {
		t.Errorf("First event type = %q, want 'system'", event1.Type)
	}

	// Second event
	event2, err := parser.Next()
	if err != nil {
		t.Fatalf("Second Next() returned error: %v", err)
	}
	if event2.Type != "result" {
		t.Errorf("Second event type = %q, want 'result'", event2.Type)
	}
}

func TestJSONLinesParser_Next_SkipsMalformedJSON(t *testing.T) {
	input := `{"type":"system"}
{invalid json}
{"type":"result"}
`
	reader := strings.NewReader(input)
	parser := NewJSONLinesParser(reader)

	// First event
	event1, err := parser.Next()
	if err != nil {
		t.Fatalf("First Next() returned error: %v", err)
	}
	if event1.Type != "system" {
		t.Errorf("First event type = %q, want 'system'", event1.Type)
	}

	// Second event (should skip malformed line)
	event2, err := parser.Next()
	if err != nil {
		t.Fatalf("Second Next() returned error: %v", err)
	}
	if event2.Type != "result" {
		t.Errorf("Second event type = %q, want 'result'", event2.Type)
	}
}

func TestJSONLinesParser_Next_EmptyInput(t *testing.T) {
	reader := strings.NewReader("")
	parser := NewJSONLinesParser(reader)

	_, err := parser.Next()
	if err != io.EOF {
		t.Errorf("Next() on empty input should return io.EOF, got %v", err)
	}
}

func TestStreamAccumulator_GetText_Empty(t *testing.T) {
	sa := NewStreamAccumulator()
	if sa.GetText() != "" {
		t.Errorf("GetText() on new accumulator = %q, want empty", sa.GetText())
	}
}

func TestStreamAccumulator_GetText_AfterMultipleMessages(t *testing.T) {
	sa := NewStreamAccumulator()

	// Simulate multiple assistant messages
	messages := []string{"Hello", ", ", "world", "!"}
	for _, text := range messages {
		event := &StreamEvent{
			Type: "assistant",
			Message: &AssistantMessage{
				Content: []ContentBlock{
					{Type: "text", Text: text},
				},
			},
		}
		if err := sa.ProcessEvent(event); err != nil {
			t.Fatalf("ProcessEvent failed: %v", err)
		}
	}

	if sa.GetText() != "Hello, world!" {
		t.Errorf("GetText() = %q, want 'Hello, world!'", sa.GetText())
	}
}

func TestStreamAccumulator_MultipleThinkingBlocks(t *testing.T) {
	sa := NewStreamAccumulator()

	// First thinking block
	event1 := &StreamEvent{
		Type: "assistant",
		Message: &AssistantMessage{
			Content: []ContentBlock{
				{Type: "thinking", Text: "First thought"},
			},
		},
	}
	sa.ProcessEvent(event1)

	// Second thinking block
	event2 := &StreamEvent{
		Type: "assistant",
		Message: &AssistantMessage{
			Content: []ContentBlock{
				{Type: "thinking", Text: "Second thought"},
			},
		},
	}
	sa.ProcessEvent(event2)

	if len(sa.ThinkingBlocks) != 2 {
		t.Fatalf("expected 2 thinking blocks, got %d", len(sa.ThinkingBlocks))
	}
	if sa.ThinkingBlocks[0] != "First thought" {
		t.Errorf("ThinkingBlocks[0] = %q, want 'First thought'", sa.ThinkingBlocks[0])
	}
	if sa.ThinkingBlocks[1] != "Second thought" {
		t.Errorf("ThinkingBlocks[1] = %q, want 'Second thought'", sa.ThinkingBlocks[1])
	}
}

func TestStreamAccumulator_MixedContent(t *testing.T) {
	sa := NewStreamAccumulator()

	// Message with text and tool use
	event := &StreamEvent{
		Type: "assistant",
		Message: &AssistantMessage{
			Content: []ContentBlock{
				{Type: "text", Text: "Let me read that file."},
				{Type: "tool_use", Name: "Read", ID: "tool_1"},
			},
			Usage: &AssistantUsage{
				InputTokens:  1000,
				OutputTokens: 50,
			},
		},
	}

	err := sa.ProcessEvent(event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}

	if sa.GetText() != "Let me read that file." {
		t.Errorf("GetText() = %q, want 'Let me read that file.'", sa.GetText())
	}
	if sa.CurrentTool != "Read" {
		t.Errorf("CurrentTool = %q, want 'Read'", sa.CurrentTool)
	}
	if sa.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", sa.InputTokens)
	}
}

// Test SSEParser alias for backward compatibility
func TestSSEParserAlias(t *testing.T) {
	input := `{"type":"system"}
`
	reader := strings.NewReader(input)
	parser := NewSSEParser(reader)

	event, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	if event.Type != "system" {
		t.Errorf("Type = %q, want 'system'", event.Type)
	}
}
