package provider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Buffer size constants for scanner operations
const (
	// ScannerInitialBufSize is the initial buffer size for scanning output (64KB)
	ScannerInitialBufSize = 64 * 1024
	// ScannerMaxBufSize is the maximum buffer size for scanning output (1MB)
	ScannerMaxBufSize = 1024 * 1024
)

// streamOutput reads from reader and writes to both buffer and writer.
// This is shared between providers for consistent output handling.
func streamOutput(reader io.Reader, buf *strings.Builder, writer io.Writer) {
	scanner := bufio.NewScanner(reader)
	// Increase scanner buffer for long lines
	scanner.Buffer(make([]byte, ScannerInitialBufSize), ScannerMaxBufSize)

	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		buf.WriteString("\n")
		fmt.Fprintln(writer, line)
	}
}

// streamJSONOutput parses SSE stream-json format, accumulates metrics, and outputs text.
// This handles the --output-format stream-json flag for real-time token tracking.
func streamJSONOutput(reader io.Reader, buf *strings.Builder, writer io.Writer, accumulator *StreamAccumulator, showThinking bool) {
	parser := NewSSEParser(reader)

	for {
		event, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Parse error - log and continue
			fmt.Fprintf(writer, "[stream-json parse error: %v]\n", err)
			continue
		}

		// Process event and update accumulator
		if err := accumulator.ProcessEvent(event); err != nil {
			// Log error but continue processing
			fmt.Fprintf(writer, "[event processing error: %v]\n", err)
			continue
		}

		// Display real-time feedback based on event type
		switch event.EventType {
		case EventContentBlockStart:
			var block ContentBlockStart
			if err := json.Unmarshal(event.Data, &block); err == nil {
				if block.ContentBlock.Type == "tool_use" {
					fmt.Fprintf(writer, "[Tool: %s]\n", block.ContentBlock.Name)
				} else if block.ContentBlock.Type == "thinking" && showThinking {
					fmt.Fprintln(writer, "🤔 Thinking...")
				}
			}

		case EventContentBlockDelta:
			var delta ContentBlockDelta
			if err := json.Unmarshal(event.Data, &delta); err == nil {
				blockType := accumulator.ActiveBlocks[delta.Index]
				if blockType == "text" && delta.Delta.Type == "text_delta" {
					// Stream text output in real-time
					fmt.Fprint(writer, delta.Delta.Text)
				} else if blockType == "thinking" && showThinking && delta.Delta.Type == "text_delta" {
					fmt.Fprint(writer, delta.Delta.Text)
				}
			}
		}
	}

	// Write final accumulated text to buffer
	buf.WriteString(accumulator.GetText())
}
