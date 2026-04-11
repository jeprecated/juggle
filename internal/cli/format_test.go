package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestIterationHeader(t *testing.T) {
	t.Run("bounded iterations", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationHeader(1, 10, "", "")
		got := buf.String()
		if !strings.Contains(got, "Iteration 1/10") {
			t.Errorf("got %q, want iteration count", got)
		}
		if !strings.Contains(got, "──") {
			t.Error("missing separator dashes")
		}
		if !strings.HasSuffix(got, "\n\n") {
			t.Errorf("got %q, want blank line after header", got)
		}
	})

	t.Run("infinite iterations", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationHeader(3, 0, "", "")
		if !strings.Contains(buf.String(), "Iteration 3/∞") {
			t.Errorf("got %q, want infinity symbol", buf.String())
		}
	})

	t.Run("watch mode includes filename", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationHeader(1, 5, "task.md", "")
		got := buf.String()
		if !strings.Contains(got, "[task.md]") {
			t.Errorf("got %q, want filename in brackets", got)
		}
	})

	t.Run("shows label when set", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationHeader(1, 10, "", "refactor auth")
		got := buf.String()
		if !strings.Contains(got, "refactor auth") {
			t.Errorf("got %q, want label in header", got)
		}
	})

	t.Run("label omitted when empty", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationHeader(1, 10, "", "")
		got := buf.String()
		if strings.Contains(got, "·") {
			t.Errorf("got %q, want no label marker when label is empty", got)
		}
	})

	t.Run("non-TTY produces plain text", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		if f.isTTY {
			t.Skip("test requires non-TTY writer")
		}
		f.IterationHeader(1, 10, "", "")
		got := buf.String()
		// Should not contain ANSI escape sequences
		if strings.Contains(got, "\033[") {
			t.Errorf("non-TTY output contains ANSI codes: %q", got)
		}
	})

	t.Run("subsequent iterations are separated by a blank line", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationHeader(1, 3, "", "")
		f.IterationHeader(2, 3, "", "")
		got := buf.String()
		if !strings.Contains(got, "\n\n── Iteration 2/3 ──") {
			t.Errorf("got %q, want blank line before second header", got)
		}
	})
}

func TestIterationStatus(t *testing.T) {
	t.Run("timing and tokens", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationStatus(12*time.Second, 1523, 892, 1200)
		got := buf.String()
		if !strings.Contains(got, "12s") {
			t.Errorf("got %q, want timing", got)
		}
		if !strings.Contains(got, "1523 in / 892 out") {
			t.Errorf("got %q, want token counts", got)
		}
		if !strings.Contains(got, "(1200 cached)") {
			t.Errorf("got %q, want cache count", got)
		}
	})

	t.Run("no cache tokens omits cache part", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationStatus(5*time.Second, 100, 50, 0)
		got := buf.String()
		if strings.Contains(got, "cached") {
			t.Errorf("got %q, want no cache info", got)
		}
		if !strings.Contains(got, "100 in / 50 out") {
			t.Errorf("got %q, want token counts", got)
		}
	})

	t.Run("no tokens shows only timing", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationStatus(3*time.Second, 0, 0, 0)
		got := buf.String()
		if !strings.Contains(got, "3s") {
			t.Errorf("got %q, want timing", got)
		}
		if strings.Contains(got, "in /") {
			t.Errorf("got %q, want no token info when zero", got)
		}
	})

	t.Run("sub-second shows milliseconds", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationStatus(250*time.Millisecond, 100, 50, 0)
		got := buf.String()
		if !strings.Contains(got, "250ms") {
			t.Errorf("got %q, want milliseconds for sub-second", got)
		}
	})

	t.Run("non-TTY produces plain text", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationStatus(10*time.Second, 500, 200, 100)
		got := buf.String()
		if strings.Contains(got, "\033[") {
			t.Errorf("non-TTY output contains ANSI codes: %q", got)
		}
	})
}

func TestPhaseAgentHeader(t *testing.T) {
	for _, phase := range []string{"pre", "before", "after", "post"} {
		t.Run(phase, func(t *testing.T) {
			var buf bytes.Buffer
			f := NewLoopFormatter(&buf)
			f.PhaseAgentHeader(phase)
			got := buf.String()
			if !strings.Contains(got, "──") {
				t.Errorf("phase=%s: got %q, want separator dashes", phase, got)
			}
			if !strings.Contains(got, "Agent ") {
				t.Errorf("phase=%s: got %q, want 'Agent ' prefix", phase, got)
			}
		})
	}

	t.Run("pre shows Pre", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.PhaseAgentHeader("pre")
		if !strings.Contains(buf.String(), "Agent Pre") {
			t.Errorf("got %q, want 'Agent Pre'", buf.String())
		}
	})

	t.Run("before shows Before", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.PhaseAgentHeader("before")
		if !strings.Contains(buf.String(), "Agent Before") {
			t.Errorf("got %q, want 'Agent Before'", buf.String())
		}
	})

	t.Run("after shows After", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.PhaseAgentHeader("after")
		if !strings.Contains(buf.String(), "Agent After") {
			t.Errorf("got %q, want 'Agent After'", buf.String())
		}
	})

	t.Run("post shows Post", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.PhaseAgentHeader("post")
		if !strings.Contains(buf.String(), "Agent Post") {
			t.Errorf("got %q, want 'Agent Post'", buf.String())
		}
	})

	t.Run("non-TTY produces plain text", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.PhaseAgentHeader("pre")
		if strings.Contains(buf.String(), "\033[") {
			t.Errorf("non-TTY output contains ANSI codes: %q", buf.String())
		}
	})
}

func TestCmdHookMarker(t *testing.T) {
	t.Run("cmd-before shows hook name and command", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.CmdHookMarker("cmd-before", "make lint")
		got := buf.String()
		if !strings.Contains(got, "cmd-before") {
			t.Errorf("got %q, want 'cmd-before'", got)
		}
		if !strings.Contains(got, "make lint") {
			t.Errorf("got %q, want command text", got)
		}
	})

	t.Run("cmd-after shows hook name and command", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.CmdHookMarker("cmd-after", "echo done")
		got := buf.String()
		if !strings.Contains(got, "cmd-after") {
			t.Errorf("got %q, want 'cmd-after'", got)
		}
		if !strings.Contains(got, "echo done") {
			t.Errorf("got %q, want command text", got)
		}
	})

	t.Run("non-TTY produces plain text", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.CmdHookMarker("cmd-before", "true")
		if strings.Contains(buf.String(), "\033[") {
			t.Errorf("non-TTY output contains ANSI codes: %q", buf.String())
		}
	})
}

func TestPollWait(t *testing.T) {
	t.Run("non-TTY prints single line", func(t *testing.T) {
		var buf bytes.Buffer
		shutdown := make(chan struct{})
		got := pollWait(&buf, "Waiting for tasks", 50*time.Millisecond, shutdown)
		if got {
			t.Error("expected false (timeout), got true (shutdown)")
		}
		if !strings.Contains(buf.String(), "Waiting for tasks") {
			t.Errorf("expected message in output, got %q", buf.String())
		}
		lines := strings.Count(buf.String(), "\n")
		if lines != 1 {
			t.Errorf("expected 1 line, got %d", lines)
		}
	})

	t.Run("shutdown interrupts wait", func(t *testing.T) {
		var buf bytes.Buffer
		shutdown := make(chan struct{})
		close(shutdown)
		got := pollWait(&buf, "Waiting", 10*time.Second, shutdown)
		if !got {
			t.Error("expected true (shutdown), got false")
		}
	})
}
