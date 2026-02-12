package provider

import (
	"bufio"
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

// streamJSONOutput parses JSON Lines format, accumulates metrics, and outputs text.
// This handles the --output-format stream-json flag for real-time token tracking.
func streamJSONOutput(reader io.Reader, buf *strings.Builder, writer io.Writer, accumulator *StreamAccumulator, showThinking bool) {
	parser := NewJSONLinesParser(reader)

	for {
		event, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Scanner errors (like "file already closed") are fatal - break out
			break
		}

		// Process event and update accumulator
		_ = accumulator.ProcessEvent(event)

		// Display real-time feedback based on event type
		switch event.Type {
		case "assistant":
			if event.Message != nil {
				for _, block := range event.Message.Content {
					switch block.Type {
					case "text":
						// Stream text output in real-time
						fmt.Fprint(writer, block.Text)
					case "tool_use":
						fmt.Fprintf(writer, "\n[Tool: %s]\n", block.Name)
					case "thinking":
						if showThinking {
							fmt.Fprintln(writer, "🤔 Thinking...")
							fmt.Fprint(writer, block.Text)
						}
					}
				}
			}

		case "system":
			// Show tool events
			if event.ToolName != "" && (event.Subtype == "tool_use" || strings.HasPrefix(event.Subtype, "tool_")) {
				fmt.Fprintf(writer, "\n[Tool: %s]\n", event.ToolName)
			}
		}
	}

	// Write final accumulated text to buffer
	buf.WriteString(accumulator.GetText())
}
