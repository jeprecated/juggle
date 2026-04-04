package provider

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// StreamEvent represents a JSON Lines event from Claude CLI --output-format stream-json
type StreamEvent struct {
	Type    string `json:"type"`    // "system", "assistant", "result", "user"
	Subtype string `json:"subtype"` // For system events: "init", "tool_use", etc.

	// For assistant messages
	Message *AssistantMessage `json:"message,omitempty"`

	// For result events
	Result    string       `json:"result,omitempty"`
	Usage     *ResultUsage `json:"usage,omitempty"`
	TotalCost float64      `json:"total_cost_usd,omitempty"`

	// For tool events
	ToolName string `json:"tool_name,omitempty"`

	// Raw data for unknown fields
	Raw json.RawMessage `json:"-"`
}

// AssistantMessage represents the message field in assistant events
type AssistantMessage struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Role    string          `json:"role"`
	Content []ContentBlock  `json:"content"`
	Usage   *AssistantUsage `json:"usage,omitempty"`
}

// ContentBlock represents a content block in an assistant message
type ContentBlock struct {
	Type  string `json:"type"` // "text", "tool_use", "thinking"
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`  // For tool_use
	Input any    `json:"input,omitempty"` // For tool_use
}

// AssistantUsage represents usage in an assistant message
type AssistantUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
}

// ResultUsage represents usage in the final result
type ResultUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
}

// StreamAccumulator accumulates JSON Lines events into usable data
type StreamAccumulator struct {
	// Cumulative metrics
	InputTokens  int
	OutputTokens int
	CacheTokens  int

	// Content tracking
	TextOutput     strings.Builder
	ThinkingBlocks []string
	ActiveBlocks   map[int]string // index -> block type

	// Current state
	CurrentTool    string
	ThinkingActive bool
}

// NewStreamAccumulator creates a new accumulator
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{
		ActiveBlocks: make(map[int]string),
	}
}

// ProcessEvent processes a JSON Lines event
func (sa *StreamAccumulator) ProcessEvent(event *StreamEvent) error {
	switch event.Type {
	case "assistant":
		if event.Message != nil {
			// Extract text content
			for _, block := range event.Message.Content {
				switch block.Type {
				case "text":
					sa.TextOutput.WriteString(block.Text)
				case "thinking":
					sa.ThinkingBlocks = append(sa.ThinkingBlocks, block.Text)
				case "tool_use":
					sa.CurrentTool = block.Name
				}
			}
			// Extract usage
			if event.Message.Usage != nil {
				sa.InputTokens = event.Message.Usage.InputTokens
				sa.OutputTokens = event.Message.Usage.OutputTokens
				sa.CacheTokens = event.Message.Usage.CacheReadTokens
			}
		}

	case "result":
		// Final usage from result
		if event.Usage != nil {
			sa.InputTokens = event.Usage.InputTokens
			sa.OutputTokens = event.Usage.OutputTokens
			sa.CacheTokens = event.Usage.CacheReadTokens
		}

	case "system":
		// Handle system events for tool visibility
		if event.Subtype == "tool_use" || strings.HasPrefix(event.Subtype, "tool_") {
			sa.CurrentTool = event.ToolName
		}
	}

	return nil
}

// GetText returns the accumulated text output
func (sa *StreamAccumulator) GetText() string {
	return sa.TextOutput.String()
}

// JSONLinesParser parses JSON Lines format (one JSON object per line)
type JSONLinesParser struct {
	scanner *bufio.Scanner
}

// NewJSONLinesParser creates a new JSON Lines parser
func NewJSONLinesParser(r io.Reader) *JSONLinesParser {
	scanner := bufio.NewScanner(r)
	// Use larger buffer for potentially large JSON lines
	scanner.Buffer(make([]byte, ScannerInitialBufSize), ScannerMaxBufSize)
	return &JSONLinesParser{scanner: scanner}
}

// Next reads and parses the next JSON line
func (p *JSONLinesParser) Next() (*StreamEvent, error) {
	for p.scanner.Scan() {
		line := p.scanner.Text()
		if line == "" {
			continue // Skip empty lines
		}

		var event StreamEvent
		event.Raw = json.RawMessage(line)
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Skip malformed JSON lines
			continue
		}

		return &event, nil
	}

	if err := p.scanner.Err(); err != nil {
		return nil, err
	}

	return nil, io.EOF
}

// Legacy aliases for backward compatibility
type SSEParser = JSONLinesParser

func NewSSEParser(r io.Reader) *SSEParser {
	return NewJSONLinesParser(r)
}
