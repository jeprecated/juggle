package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/term"
)

// CommitChoice represents the user's choice for commit confirmation
type CommitChoice int

const (
	CommitChoiceNo CommitChoice = iota
	CommitChoiceYes
	CommitChoiceEdit
)

// ConfirmSingleKey displays a yes/no prompt and waits for a single keypress.
// Returns true for 'y'/'Y', false for 'n'/'N', or error on Ctrl+C.
// No Enter key is required - responds immediately to keypress.
func ConfirmSingleKey(prompt string) (bool, error) {
	fmt.Printf("%s (y/n): ", prompt)

	// Get terminal file descriptor
	fd := int(os.Stdin.Fd())

	// Save original terminal state and ensure it's restored
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return false, fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	// Read single byte
	b := make([]byte, 1)
	_, err = os.Stdin.Read(b)
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	// Check key pressed
	key := b[0]

	// Handle Ctrl+C (ASCII 3)
	if key == 3 {
		fmt.Println("\n^C")
		return false, fmt.Errorf("interrupted")
	}

	// Echo the key and newline for valid responses
	if key == 'y' || key == 'Y' {
		fmt.Println("y")
		return true, nil
	} else if key == 'n' || key == 'N' {
		fmt.Println("n")
		return false, nil
	}

	// Invalid key - restore terminal and recurse
	term.Restore(fd, oldState)
	fmt.Println()
	fmt.Println("Invalid key. Please press 'y' or 'n'.")
	return ConfirmSingleKey(prompt)
}

// ConfirmOrEditSingleKey displays a yes/no/edit prompt and waits for a single keypress.
// Returns CommitChoiceYes for 'y'/'Y', CommitChoiceNo for 'n'/'N', CommitChoiceEdit for 'e'/'E'.
// Returns error on Ctrl+C.
func ConfirmOrEditSingleKey(prompt string) (CommitChoice, error) {
	fmt.Printf("%s (y/n/e to edit): ", prompt)

	// Get terminal file descriptor
	fd := int(os.Stdin.Fd())

	// Save original terminal state and ensure it's restored
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return CommitChoiceNo, fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	// Read single byte
	b := make([]byte, 1)
	_, err = os.Stdin.Read(b)
	if err != nil {
		return CommitChoiceNo, fmt.Errorf("failed to read input: %w", err)
	}

	// Check key pressed
	key := b[0]

	// Handle Ctrl+C (ASCII 3)
	if key == 3 {
		fmt.Println("\n^C")
		return CommitChoiceNo, fmt.Errorf("interrupted")
	}

	// Echo the key and newline for valid responses
	if key == 'y' || key == 'Y' {
		fmt.Println("y")
		return CommitChoiceYes, nil
	} else if key == 'n' || key == 'N' {
		fmt.Println("n")
		return CommitChoiceNo, nil
	} else if key == 'e' || key == 'E' {
		fmt.Println("e")
		return CommitChoiceEdit, nil
	}

	// Invalid key - restore terminal and recurse
	term.Restore(fd, oldState)
	fmt.Println()
	fmt.Println("Invalid key. Please press 'y', 'n', or 'e'.")
	return ConfirmOrEditSingleKey(prompt)
}

// EditTextInEditor opens the default editor ($VISUAL or $EDITOR, falling back to vim)
// with the given text and returns the edited text.
func EditTextInEditor(text string) (string, error) {
	// Create temp file for editing
	tmpFile, err := os.CreateTemp("", "commit-msg-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write initial text
	if _, err := tmpFile.WriteString(text); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Get editor from environment
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vim"
	}

	// Resolve editor to absolute path if possible
	editorPath, err := exec.LookPath(filepath.Base(editor))
	if err != nil {
		editorPath = editor
	}

	// Run editor
	cmd := exec.Command(editorPath, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	// Read edited content
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}

	return string(content), nil
}
