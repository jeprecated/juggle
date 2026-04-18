package cli

import (
	"fmt"
	"io"
	"os"
)

const stopRequestedMessage = "Stop requested. Stopping after this iteration completes. Press Ctrl+C again to stop now."

func writeStopRequestedMessage(stderr io.Writer, color bool) {
	if stderr == nil {
		stderr = os.Stderr
	}
	if color {
		fmt.Fprintf(stderr, "%s%s%s\n", ansiRed, stopRequestedMessage, ansiReset)
	} else {
		fmt.Fprintln(stderr, stopRequestedMessage)
	}
}

// StartKeypressListener starts a goroutine that reads from r one byte at a time.
// When 'q' or 'Q' is read, onStop is called and a stop message is written to stderr.
// When Enter ('\n' or '\r') is read, onEnter is called (if non-nil).
// If color is true, the stop message is printed in red.
// Returns a wait function that blocks until the goroutine exits; callers
// may discard it if they close r directly (e.g. via ttyCleanup).
// The goroutine exits when r returns an error (e.g. EOF or file close).
func StartKeypressListener(r io.Reader, onStop func(), color bool, stderr io.Writer, onEnter func()) func() {
	goroutineDone := make(chan struct{})

	go func() {
		defer close(goroutineDone)
		buf := make([]byte, 1)
		for {
			n, err := r.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				if buf[0] == 'q' || buf[0] == 'Q' {
					writeStopRequestedMessage(stderr, color)
					onStop()
					return
				}
				if onEnter != nil && (buf[0] == '\n' || buf[0] == '\r') {
					onEnter()
				}
			}
		}
	}()

	return func() {
		<-goroutineDone
	}
}
