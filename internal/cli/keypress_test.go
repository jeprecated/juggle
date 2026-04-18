package cli

import (
	"bytes"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartKeypressListener_QCallsTrigger(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()

	triggered := make(chan struct{})
	var stderr bytes.Buffer

	cleanup := StartKeypressListener(r, func() { close(triggered) }, false, &stderr, nil)
	defer cleanup()

	io.WriteString(w, "q") //nolint:errcheck

	select {
	case <-triggered:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for trigger on 'q'")
	}
}

func TestStartKeypressListener_UpperQCallsTrigger(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()

	triggered := make(chan struct{})
	var stderr bytes.Buffer

	cleanup := StartKeypressListener(r, func() { close(triggered) }, false, &stderr, nil)
	defer cleanup()

	io.WriteString(w, "Q") //nolint:errcheck

	select {
	case <-triggered:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for trigger on 'Q'")
	}
}

func TestStartKeypressListener_OtherKeyNoTrigger(t *testing.T) {
	r, w := io.Pipe()

	var triggered atomic.Bool
	var stderr bytes.Buffer

	cleanup := StartKeypressListener(r, func() { triggered.Store(true) }, false, &stderr, nil)

	io.WriteString(w, "x") //nolint:errcheck
	w.Close()
	cleanup()

	if triggered.Load() {
		t.Error("expected trigger NOT to be called on 'x' press")
	}
}

func TestStartKeypressListener_PrintsMessageOnQ(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()

	var stderr bytes.Buffer
	done := make(chan struct{})

	cleanup := StartKeypressListener(r, func() { close(done) }, false, &stderr, nil)
	defer cleanup()

	io.WriteString(w, "q") //nolint:errcheck
	<-done

	if !strings.Contains(stderr.String(), stopRequestedMessage) {
		t.Errorf("expected stop message, got: %q", stderr.String())
	}
}

func TestStartKeypressListener_PrintsRedWhenColorEnabled(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()

	var stderr bytes.Buffer
	done := make(chan struct{})

	cleanup := StartKeypressListener(r, func() { close(done) }, true, &stderr, nil)
	defer cleanup()

	io.WriteString(w, "q") //nolint:errcheck
	<-done

	output := stderr.String()
	if !strings.Contains(output, "\033[31m") {
		t.Error("expected red ANSI code when color enabled")
	}
	if !strings.Contains(output, stopRequestedMessage) {
		t.Error("expected stop message")
	}
}

func TestStartKeypressListener_NoColorWhenDisabled(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()

	var stderr bytes.Buffer
	done := make(chan struct{})

	cleanup := StartKeypressListener(r, func() { close(done) }, false, &stderr, nil)
	defer cleanup()

	io.WriteString(w, "q") //nolint:errcheck
	<-done

	output := stderr.String()
	if strings.Contains(output, "\033[") {
		t.Error("expected no ANSI codes when color disabled")
	}
}

func TestStartKeypressListener_CleanupOnEOF(t *testing.T) {
	r, w := io.Pipe()
	var stderr bytes.Buffer

	cleanup := StartKeypressListener(r, func() {}, false, &stderr, nil)

	w.Close() // EOF causes goroutine to exit
	cleanup() // should not hang
}

func TestWriteStopRequestedMessage_NilWriterFallsBack(t *testing.T) {
	writeStopRequestedMessage(nil, false)
}

func TestStartKeypressListener_EnterCallsOnEnter(t *testing.T) {
	r, w := io.Pipe()

	triggered := make(chan struct{})
	var stderr bytes.Buffer

	cleanup := StartKeypressListener(r, func() {}, false, &stderr, func() { close(triggered) })

	io.WriteString(w, "\n") //nolint:errcheck

	select {
	case <-triggered:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for onEnter on newline")
	}
	w.Close()
	cleanup()
}

func TestStartKeypressListener_CarriageReturnCallsOnEnter(t *testing.T) {
	r, w := io.Pipe()

	triggered := make(chan struct{})
	var stderr bytes.Buffer

	cleanup := StartKeypressListener(r, func() {}, false, &stderr, func() { close(triggered) })

	io.WriteString(w, "\r") //nolint:errcheck

	select {
	case <-triggered:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for onEnter on carriage return")
	}
	w.Close()
	cleanup()
}

func TestStartKeypressListener_NilOnEnterIgnored(t *testing.T) {
	r, w := io.Pipe()

	var triggered atomic.Bool
	var stderr bytes.Buffer

	cleanup := StartKeypressListener(r, func() { triggered.Store(true) }, false, &stderr, nil)

	io.WriteString(w, "\n") //nolint:errcheck
	w.Close()
	cleanup()

	if triggered.Load() {
		t.Error("expected onStop NOT to be called on Enter")
	}
}
