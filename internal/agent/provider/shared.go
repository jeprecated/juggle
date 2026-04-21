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

// streamJSONOutput parses JSON Lines format, accumulates metrics, and outputs text.
// This handles the --output-format stream-json flag for real-time token tracking.
func streamJSONOutput(reader io.Reader, buf *strings.Builder, writer io.Writer, accumulator *StreamAccumulator, showThinking bool, verbose bool) {
	parser := NewJSONLinesParser(reader)

	var lastTool string // tracks last tool name for dedup

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
						lastTool = "" // reset dedup on text output
					case "tool_use":
						if block.Name != lastTool {
							if verbose {
								if summary := formatToolInput(block.Name, block.Input); summary != "" {
									fmt.Fprintf(writer, "\n[Tool: %s] %s\n", block.Name, summary)
								} else {
									fmt.Fprintf(writer, "\n[Tool: %s]\n", block.Name)
								}
							} else {
								fmt.Fprintf(writer, "\n[Tool: %s]\n", block.Name)
							}
							lastTool = block.Name
						}
					case "thinking":
						if showThinking {
							fmt.Fprintln(writer, "🤔 Thinking...")
							fmt.Fprint(writer, block.Text)
						}
					}
				}
			}

		case "result":
			if event.Result != "" {
				fmt.Fprintln(writer, event.Result)
				buf.WriteString(event.Result)
				buf.WriteString("\n")
			}

		case "system":
			// Show tool events (deduplicated)
			if event.ToolName != "" && (event.Subtype == "tool_use" || strings.HasPrefix(event.Subtype, "tool_")) {
				if event.ToolName != lastTool {
					fmt.Fprintf(writer, "\n[Tool: %s]\n", event.ToolName)
					lastTool = event.ToolName
				}
			}
		}
	}

	// Write final accumulated text to buffer
	buf.WriteString(accumulator.GetText())
}

// formatToolInput returns a compact summary of tool input for verbose output.
func formatToolInput(toolName string, input any) string {
	m, ok := input.(map[string]interface{})
	if !ok || len(m) == 0 {
		return ""
	}

	switch toolName {
	case "Bash":
		if cmd, ok := m["command"].(string); ok {
			if len(cmd) > 120 {
				return cmd[:120] + "..."
			}
			return cmd
		}
	case "Read":
		if fp, ok := m["file_path"].(string); ok {
			return fp
		}
	case "Write":
		if fp, ok := m["file_path"].(string); ok {
			return fp
		}
	case "Edit":
		if fp, ok := m["file_path"].(string); ok {
			return fp
		}
	case "Grep":
		pat, _ := m["pattern"].(string)
		path, _ := m["path"].(string)
		glob, _ := m["glob"].(string)
		var parts []string
		if pat != "" {
			parts = append(parts, fmt.Sprintf("%q", pat))
		}
		if glob != "" {
			parts = append(parts, "in "+glob)
		} else if path != "" {
			parts = append(parts, "in "+path)
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	case "Glob":
		pat, _ := m["pattern"].(string)
		path, _ := m["path"].(string)
		if pat != "" {
			if path != "" {
				return pat + " in " + path
			}
			return pat
		}
	default:
		b, err := json.Marshal(m)
		if err != nil {
			return ""
		}
		s := string(b)
		if len(s) > 100 {
			return s[:100] + "..."
		}
		return s
	}

	return ""
}
