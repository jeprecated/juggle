package cli

import (
	"fmt"
	"io"
	"strings"
)

const toonTruncateLen = 60

// ToonList writes a TOON-formatted list to w.
//
//	name:    collection name (e.g. "sessions", "runs")
//	fields:  ordered field names for the schema header
//	rows:    slice of string slices, one per row, matching fields order
//	total:   pre-computed total count (0 = omit aggregate line)
//
// When rows is empty, writes a definitive empty state message.
func ToonList(w io.Writer, name string, fields []string, rows [][]string, total int) {
	if len(rows) == 0 {
		fmt.Fprintf(w, "%s[0]: none found\n", name)
		return
	}
	fmt.Fprintf(w, "%s[%d]{%s}:\n", name, len(rows), strings.Join(fields, ","))
	for _, row := range rows {
		fmt.Fprintf(w, "  %s\n", toonRow(fields, row))
	}
	if total > 0 && total != len(rows) {
		fmt.Fprintf(w, "count: %d of %d total\n", len(rows), total)
	}
}

// ToonObject writes a single TOON object to w.
func ToonObject(w io.Writer, fields []string, values []string) {
	for i, f := range fields {
		if i < len(values) {
			fmt.Fprintf(w, "%s: %s\n", f, toonValue(values[i]))
		}
	}
}

// ToonError writes a structured error in TOON format to w.
func ToonError(w io.Writer, code, message string) {
	fmt.Fprintf(w, "error: %s\n", message)
	fmt.Fprintf(w, "code: %s\n", code)
}

// ToonHelp writes contextual help hints.
func ToonHelp(w io.Writer, hints []string) {
	if len(hints) == 0 {
		return
	}
	fmt.Fprintf(w, "help[%d]:\n", len(hints))
	for _, h := range hints {
		fmt.Fprintf(w, "  %s\n", h)
	}
}

// toonRow formats a single TOON row, applying truncation and quoting.
func toonRow(fields []string, values []string) string {
	n := len(fields)
	if len(values) < n {
		n = len(values)
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = toonValue(values[i])
	}
	return strings.Join(parts, ",")
}

// toonValue formats a single value for TOON output.
// Values containing commas are double-quoted.
// Values longer than toonTruncateLen are truncated with a size annotation.
func toonValue(s string) string {
	if len(s) > toonTruncateLen {
		truncated := s[:toonTruncateLen-3] + "..."
		return fmt.Sprintf("%s(%db)", truncated, len(s))
	}
	if strings.Contains(s, ",") || strings.Contains(s, "\"") {
		escaped := strings.ReplaceAll(s, "\"", "\"\"")
		return "\"" + escaped + "\""
	}
	return s
}
