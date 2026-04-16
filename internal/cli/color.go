package cli

import (
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiRed     = "\033[31m"
	ansiCyan    = "\033[36m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiBlue    = "\033[34m"
	ansiMagenta = "\033[35m"
)

// isColorEnabled reports whether ANSI color should be written to w.
// Color is disabled when NO_COLOR is set or w is not a character device (TTY).
func isColorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// bold wraps s in ANSI bold codes.
func bold(s string) string {
	return ansiBold + s + ansiReset
}

// colorizeHeading returns s styled as a section heading when color is true.
func colorizeHeading(s string, color bool) string {
	if !color {
		return s
	}
	return ansiBold + ansiCyan + s + ansiReset
}

// flagLineRe matches a pflag usage line: leading spaces, optional short flag, long flag.
var flagLineRe = regexp.MustCompile(`(?m)^(\s+)((?:-\w,\s+)?--[\w-]+)`)

// colorizeFlagUsages applies bold to flag names in pflag FlagUsages() output.
func colorizeFlagUsages(text string, color bool) string {
	if !color {
		return text
	}
	return flagLineRe.ReplaceAllStringFunc(text, func(match string) string {
		// Preserve leading whitespace, bold the flag portion.
		loc := flagLineRe.FindStringSubmatchIndex(match)
		if len(loc) < 6 {
			return match
		}
		prefix := match[loc[2]:loc[3]]
		flags := match[loc[4]:loc[5]]
		return prefix + ansiBold + flags + ansiReset
	})
}

// keywordRe matches "juggle", "loop", "queue" as whole words for example coloring.
var keywordRe = regexp.MustCompile(`\b(juggle|loop|queue)\b`)

// colorizeExamples styles example lines: comments in yellow, commands in green
// with "juggle" in magenta, "loop" in cyan, "queue" in blue.
func colorizeExamples(text string, color bool) string {
	if !color {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if strings.HasPrefix(trimmed, "#") {
			lines[i] = indent + ansiYellow + trimmed + ansiReset
		} else {
			lines[i] = indent + colorizeCommandKeywords(trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

// colorizeCommandKeywords colors "juggle" (magenta), "loop" (cyan), "queue" (blue)
// within a command line. All other text is green.
func colorizeCommandKeywords(line string) string {
	var b strings.Builder
	b.WriteString(ansiGreen)
	lastEnd := 0
	for _, m := range keywordRe.FindAllStringSubmatchIndex(line, -1) {
		b.WriteString(line[lastEnd:m[0]])
		keyword := line[m[2]:m[3]]
		var c string
		switch keyword {
		case "juggle":
			c = ansiMagenta
		case "loop":
			c = ansiCyan
		case "queue":
			c = ansiBlue
		}
		b.WriteString(ansiReset)
		b.WriteString(c)
		b.WriteString(keyword)
		b.WriteString(ansiReset)
		b.WriteString(ansiGreen)
		lastEnd = m[3]
	}
	b.WriteString(line[lastEnd:])
	b.WriteString(ansiReset)
	return b.String()
}
