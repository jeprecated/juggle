//go:build linux

package cli

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// openTTYKeypress opens /dev/tty in raw mode for single-byte keypress reads.
// Returns the tty file and a cleanup function that restores the terminal and closes the file.
// Returns an error if stdin is not a TTY or /dev/tty cannot be opened.
func openTTYKeypress() (*os.File, func(), error) {
	if !isStdinTTY() {
		return nil, func() {}, fmt.Errorf("stdin is not a TTY")
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err != nil {
		return nil, func() {}, fmt.Errorf("opening /dev/tty: %w", err)
	}

	oldState, err := makeRaw(tty.Fd())
	if err != nil {
		tty.Close()
		return nil, func() {}, fmt.Errorf("raw mode: %w", err)
	}

	cleanup := func() {
		restoreTerminal(tty.Fd(), oldState)
		tty.Close()
	}
	return tty, cleanup, nil
}

// isStdinTTY reports whether os.Stdin is connected to a terminal.
func isStdinTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// makeRaw puts fd into raw mode (no echo, no line buffering).
// Returns the previous termios state so it can be restored.
func makeRaw(fd uintptr) (*syscall.Termios, error) {
	var oldState syscall.Termios
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&oldState))); errno != 0 {
		return nil, errno
	}

	newState := oldState
	// cfmakeraw: disable echo, canonical mode, signals, and output processing
	newState.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	newState.Oflag &^= syscall.OPOST
	newState.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	newState.Cflag &^= syscall.CSIZE | syscall.PARENB
	newState.Cflag |= syscall.CS8
	newState.Cc[syscall.VMIN] = 1
	newState.Cc[syscall.VTIME] = 0

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&newState))); errno != 0 {
		return nil, errno
	}
	return &oldState, nil
}

// restoreTerminal restores fd to the given termios state.
func restoreTerminal(fd uintptr, state *syscall.Termios) {
	syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(state))) //nolint:errcheck
}
