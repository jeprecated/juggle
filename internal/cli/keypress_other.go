//go:build !linux

package cli

import (
	"fmt"
	"os"
)

// openTTYKeypress is not supported on non-Linux platforms.
func openTTYKeypress() (*os.File, func(), error) {
	return nil, func() {}, fmt.Errorf("keypress listener not supported on this platform")
}

// isStdinTTY reports whether os.Stdin is connected to a terminal.
func isStdinTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
