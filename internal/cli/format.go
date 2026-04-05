package cli

import (
	"fmt"
	"io"
	"os"
	"time"
)

// ANSI escape codes for dim gray (color 245) styling.
const (
	dimOn  = "\033[38;5;245m"
	dimOff = "\033[0m"
)

// LoopFormatter prints iteration headers and status lines to stderr.
// When the writer is a TTY, output uses dim gray ANSI styling.
type LoopFormatter struct {
	w     io.Writer
	isTTY bool
}

// NewLoopFormatter creates a formatter that writes to w.
// TTY detection enables colored output.
func NewLoopFormatter(w io.Writer) *LoopFormatter {
	isTTY := false
	if f, ok := w.(*os.File); ok {
		if info, err := f.Stat(); err == nil {
			isTTY = info.Mode()&os.ModeCharDevice != 0
		}
	}

	return &LoopFormatter{w: w, isTTY: isTTY}
}

// IterationHeader prints a separator line before an iteration starts.
// For watch mode, pass a non-empty filename. Pass a non-empty runLabel to show the run label.
func (f *LoopFormatter) IterationHeader(iteration, max int, filename, runLabel string) {
	label := fmt.Sprintf("Iteration %d/%s", iteration, maxStr(max))
	if filename != "" {
		label += fmt.Sprintf(" [%s]", filename)
	}
	if runLabel != "" {
		label += fmt.Sprintf(" · %s", runLabel)
	}
	line := fmt.Sprintf("── %s ──", label)

	if f.isTTY {
		fmt.Fprintf(f.w, "%s%s%s\n", dimOn, line, dimOff)
	} else {
		fmt.Fprintln(f.w, line)
	}
}

// PhaseAgentHeader prints a dim separator before a phase agent runs.
// phase is one of: pre, before, after, post.
func (f *LoopFormatter) PhaseAgentHeader(phase string) {
	title := string(phase[0]-'a'+'A') + phase[1:]
	line := fmt.Sprintf("── Agent %s ──", title)
	if f.isTTY {
		fmt.Fprintf(f.w, "%s%s%s\n", dimOn, line, dimOff)
	} else {
		fmt.Fprintln(f.w, line)
	}
}

// CmdHookMarker prints a dim marker line before a cmd hook runs.
// hookName is "cmd-before" or "cmd-after".
func (f *LoopFormatter) CmdHookMarker(hookName, cmd string) {
	line := fmt.Sprintf("  %s: %s", hookName, cmd)
	if f.isTTY {
		fmt.Fprintf(f.w, "%s%s%s\n", dimOn, line, dimOff)
	} else {
		fmt.Fprintln(f.w, line)
	}
}

// IterationStatus prints timing and token metrics after an iteration completes.
func (f *LoopFormatter) IterationStatus(elapsed time.Duration, inputTokens, outputTokens, cacheTokens int) {
	var timing string
	if elapsed < time.Second {
		timing = fmt.Sprintf("%dms", elapsed.Milliseconds())
	} else {
		timing = fmt.Sprintf("%ds", int(elapsed.Seconds()))
	}

	status := fmt.Sprintf("  %s", timing)
	if inputTokens > 0 || outputTokens > 0 {
		tok := fmt.Sprintf(" | %d in / %d out", inputTokens, outputTokens)
		if cacheTokens > 0 {
			tok += fmt.Sprintf(" (%d cached)", cacheTokens)
		}
		status += tok
	}

	if f.isTTY {
		fmt.Fprintf(f.w, "%s%s%s\n", dimOn, status, dimOff)
	} else {
		fmt.Fprintln(f.w, status)
	}
}
