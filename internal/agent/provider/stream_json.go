package provider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// StreamEvent represents a single SSE event from Claude API
type StreamEvent struct {
	EventType string          `json:"-"` // Parsed from "event:" line
	Data      json.RawMessage `json:"-"` // Parsed from "data:" line
}

// Event type constants
const (
	EventMessageStart       = "message_start"
	EventContentBlockStart  = "content_block_start"
	EventContentBlockDelta  = "content_block_delta"
	EventContentBlockStop   = "content_block_stop"
	EventMessageDelta       = "message_delta"
	EventMessageStop        = "message_stop"
	EventPing               = "ping"
	EventError              = "error"
)

// MessageStart represents the initial message metadata
type MessageStart struct {
	Type    string `json:"type"`
	Message struct {
		ID           string `json:"id"`
		Type         string `json:"type"`
		Role         string `json:"role"`
		Model        string `json:"model"`
		StopReason   string `json:"stop_reason"`
		StopSequence string `json:"stop_sequence"`
		Usage        struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ContentBlockStart represents the start of a content block
type ContentBlockStart struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"` // "text", "tool_use", "thinking"
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"` // For tool_use
	} `json:"content_block"`
}

// ContentBlockDelta represents incremental content
type ContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type       string `json:"type"` // "text_delta", "input_json_delta"
		Text       string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
}

// MessageDelta represents usage updates
type MessageDelta struct {
	Type  string `json:"type"`
	Delta struct {
		StopReason   string `json:"stop_reason,omitempty"`
		StopSequence string `json:"stop_sequence,omitempty"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ContentBlockStop represents block completion
type ContentBlockStop struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// StreamAccumulator accumulates streaming events into usable data
type StreamAccumulator struct {
	// Cumulative metrics
	InputTokens  int
	OutputTokens int
	CacheTokens  int

	// Content tracking
	TextOutput     strings.Builder
	ThinkingBlocks []string
	ActiveTools    map[int]string // index -> tool name
	ActiveBlocks   map[int]string // index -> block type

	// Current state
	CurrentTool     string
	ThinkingActive  bool
	currentThinking strings.Builder
}

// NewStreamAccumulator creates a new accumulator
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{
		ActiveTools:  make(map[int]string),
		ActiveBlocks: make(map[int]string),
	}
}

// ProcessEvent processes a single SSE event
func (sa *StreamAccumulator) ProcessEvent(event *StreamEvent) error {
	switch event.EventType {
	case EventMessageStart:
		var msg MessageStart
		if err := json.Unmarshal(event.Data, &msg); err != nil {
			return fmt.Errorf("failed to parse message_start: %w", err)
		}
		sa.InputTokens = msg.Message.Usage.InputTokens
		sa.OutputTokens = msg.Message.Usage.OutputTokens

	case EventContentBlockStart:
		var block ContentBlockStart
		if err := json.Unmarshal(event.Data, &block); err != nil {
			return fmt.Errorf("failed to parse content_block_start: %w", err)
		}
		sa.ActiveBlocks[block.Index] = block.ContentBlock.Type

		if block.ContentBlock.Type == "tool_use" {
			sa.ActiveTools[block.Index] = block.ContentBlock.Name
			sa.CurrentTool = block.ContentBlock.Name
		} else if block.ContentBlock.Type == "thinking" {
			sa.ThinkingActive = true
			sa.currentThinking.Reset()
		}

	case EventContentBlockDelta:
		var delta ContentBlockDelta
		if err := json.Unmarshal(event.Data, &delta); err != nil {
			return fmt.Errorf("failed to parse content_block_delta: %w", err)
		}

		blockType := sa.ActiveBlocks[delta.Index]
		if blockType == "text" && delta.Delta.Type == "text_delta" {
			sa.TextOutput.WriteString(delta.Delta.Text)
		} else if blockType == "thinking" && delta.Delta.Type == "text_delta" {
			sa.currentThinking.WriteString(delta.Delta.Text)
		}

	case EventContentBlockStop:
		var stop ContentBlockStop
		if err := json.Unmarshal(event.Data, &stop); err != nil {
			return fmt.Errorf("failed to parse content_block_stop: %w", err)
		}

		blockType := sa.ActiveBlocks[stop.Index]
		if blockType == "tool_use" {
			delete(sa.ActiveTools, stop.Index)
			sa.CurrentTool = ""
		} else if blockType == "thinking" {
			sa.ThinkingActive = false
			if sa.currentThinking.Len() > 0 {
				sa.ThinkingBlocks = append(sa.ThinkingBlocks, sa.currentThinking.String())
			}
		}
		delete(sa.ActiveBlocks, stop.Index)

	case EventMessageDelta:
		var delta MessageDelta
		if err := json.Unmarshal(event.Data, &delta); err != nil {
			return fmt.Errorf("failed to parse message_delta: %w", err)
		}
		// Cumulative output tokens
		sa.OutputTokens = delta.Usage.OutputTokens

	case EventMessageStop:
		// Final message, nothing to accumulate

	case EventPing, EventError:
		// Ignore pings and handle errors upstream

	default:
		// Unknown event type, ignore gracefully
	}

	return nil
}

// GetText returns the accumulated text output
func (sa *StreamAccumulator) GetText() string {
	return sa.TextOutput.String()
}

// SSEParser parses Server-Sent Events from a reader
type SSEParser struct {
	scanner *bufio.Scanner
}

// NewSSEParser creates a new SSE parser
func NewSSEParser(r io.Reader) *SSEParser {
	return &SSEParser{
		scanner: bufio.NewScanner(r),
	}
}

// Next reads the next SSE event from the stream
func (p *SSEParser) Next() (*StreamEvent, error) {
	var event StreamEvent
	var dataLines []string

	for p.scanner.Scan() {
		line := p.scanner.Text()

		// Empty line signals end of event
		if line == "" {
			if event.EventType != "" && len(dataLines) > 0 {
				// Join data lines and parse
				event.Data = json.RawMessage(strings.Join(dataLines, "\n"))
				return &event, nil
			}
			// Reset for next event
			event = StreamEvent{}
			dataLines = nil
			continue
		}

		// Parse event field
		if strings.HasPrefix(line, "event:") {
			event.EventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		// Parse data field
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			dataLines = append(dataLines, data)
			continue
		}

		// Ignore other fields (id:, retry:, etc.)
	}

	// Check for scan errors
	if err := p.scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	// End of stream
	return nil, io.EOF
}
