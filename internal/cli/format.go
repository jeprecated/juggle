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

// throbberFrames are the ASCII spinner characters cycled during poll waits.
var throbberFrames = [...]byte{'|', '/', '-', '\\'}

// pollWait displays a throbber on w while waiting for delay to elapse.
// On a TTY the spinner animates in-place; on a pipe a single line is printed.
// Returns true if shutdown was signaled during the wait.
func pollWait(w io.Writer, msg string, delay time.Duration, shutdown <-chan struct{}) bool {
	isTTY := false
	if f, ok := w.(*os.File); ok {
		if info, err := f.Stat(); err == nil {
			isTTY = info.Mode()&os.ModeCharDevice != 0
		}
	}

	if !isTTY {
		fmt.Fprintf(w, "%s\n", msg)
		select {
		case <-time.After(delay):
			return false
		case <-shutdown:
			return true
		}
	}

	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	timeout := time.After(delay)
	i := 0

	for {
		fmt.Fprintf(w, "\r%s%s %c%s\033[K", dimOn, msg, throbberFrames[i%len(throbberFrames)], dimOff)

		select {
		case <-tick.C:
			i++
		case <-timeout:
			fmt.Fprint(w, "\r\033[K")
			return false
		case <-shutdown:
			fmt.Fprint(w, "\r\033[K")
			return true
		}
	}
}

// formatCountdown formats a duration for countdown display.
// Seconds are hidden when >= 1 minute remains; shown when < 1 minute.
func formatCountdown(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(d.Seconds())

	if totalSeconds < 60 {
		return fmt.Sprintf("%ds", totalSeconds)
	}

	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}

// pollWaitWithWake is like pollWait but also wakes on the optional wakeCh.
// When countdown is true, msg is treated as a prefix and a dynamic countdown
// (based on remaining time) is appended.
func pollWaitWithWake(w io.Writer, msg string, delay time.Duration, shutdown <-chan struct{}, wake <-chan struct{}, countdown bool) bool {
	isTTY := false
	if f, ok := w.(*os.File); ok {
		if info, err := f.Stat(); err == nil {
			isTTY = info.Mode()&os.ModeCharDevice != 0
		}
	}

	waitDone := func() <-chan struct{} {
		ch := make(chan struct{})
		go func() {
			time.Sleep(delay)
			close(ch)
		}()
		return ch
	}

	if !isTTY {
		displayMsg := msg
		if countdown {
			displayMsg = fmt.Sprintf("%s %s", msg, formatCountdown(delay))
		}
		fmt.Fprintf(w, "%s\n", displayMsg)
		select {
		case <-waitDone():
			return false
		case <-shutdown:
			return true
		case <-wake:
			return false
		}
	}

	start := time.Now()
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	timeout := waitDone()
	i := 0

	for {
		displayMsg := msg
		if countdown {
			remaining := delay - time.Since(start)
			displayMsg = fmt.Sprintf("%s %s", msg, formatCountdown(remaining))
		}
		fmt.Fprintf(w, "\r%s%s %c%s\033[K", dimOn, displayMsg, throbberFrames[i%len(throbberFrames)], dimOff)

		select {
		case <-tick.C:
			i++
		case <-timeout:
			fmt.Fprint(w, "\r\033[K")
			return false
		case <-shutdown:
			fmt.Fprint(w, "\r\033[K")
			return true
		case <-wake:
			fmt.Fprint(w, "\r\033[K")
			return false
		}
	}
}

// LoopFormatter prints iteration headers and status lines to stderr.
// When the writer is a TTY, output uses dim gray ANSI styling.
type LoopFormatter struct {
	w              io.Writer
	isTTY          bool
	iterationCount int
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

	if f.iterationCount > 0 {
		fmt.Fprintln(f.w)
	}
	f.iterationCount++

	if f.isTTY {
		fmt.Fprintf(f.w, "%s%s%s\n\n", ansiBold+ansiCyan, line, ansiReset)
	} else {
		fmt.Fprintf(f.w, "%s\n\n", line)
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
		fmt.Fprintf(f.w, "\n%s%s%s\n", dimOn, status, dimOff)
	} else {
		fmt.Fprintln(f.w, status)
	}
}
