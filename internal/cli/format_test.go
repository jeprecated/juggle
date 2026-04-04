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
		f.IterationHeader(1, 10, "")
		got := buf.String()
		if !strings.Contains(got, "Iteration 1/10") {
			t.Errorf("got %q, want iteration count", got)
		}
		if !strings.Contains(got, "──") {
			t.Error("missing separator dashes")
		}
	})

	t.Run("unlimited iterations", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationHeader(3, 0, "")
		if !strings.Contains(buf.String(), "Iteration 3/unlimited") {
			t.Errorf("got %q, want unlimited", buf.String())
		}
	})

	t.Run("watch mode includes filename", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		f.IterationHeader(1, 5, "task.md")
		got := buf.String()
		if !strings.Contains(got, "[task.md]") {
			t.Errorf("got %q, want filename in brackets", got)
		}
	})

	t.Run("non-TTY produces plain text", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewLoopFormatter(&buf)
		if f.isTTY {
			t.Skip("test requires non-TTY writer")
		}
		f.IterationHeader(1, 10, "")
		got := buf.String()
		// Should not contain ANSI escape sequences
		if strings.Contains(got, "\033[") {
			t.Errorf("non-TTY output contains ANSI codes: %q", got)
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
