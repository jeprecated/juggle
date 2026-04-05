package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestIsColorEnabledReturnsFalseForBuffer(t *testing.T) {
	var buf bytes.Buffer
	if isColorEnabled(&buf) {
		t.Error("expected isColorEnabled to return false for a bytes.Buffer")
	}
}

func TestIsColorEnabledReturnsFalseWhenNOCOLORSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if isColorEnabled(os.Stdout) {
		t.Error("expected isColorEnabled to return false when NO_COLOR is set")
	}
}

func TestBoldWrapsWithANSICodes(t *testing.T) {
	result := bold("hello")
	if !strings.HasPrefix(result, "\033[") {
		t.Error("bold() should start with ANSI escape")
	}
	if !strings.Contains(result, "hello") {
		t.Error("bold() should contain original text")
	}
	if !strings.HasSuffix(result, "\033[0m") {
		t.Error("bold() should end with ANSI reset")
	}
}

func TestColorizeHeadingWithColorTrue(t *testing.T) {
	result := colorizeHeading("Loop Control", true)
	if !strings.Contains(result, "\033[") {
		t.Error("colorizeHeading with color=true should contain ANSI codes")
	}
	if !strings.Contains(result, "Loop Control") {
		t.Error("colorizeHeading should preserve the heading text")
	}
}

func TestColorizeHeadingWithColorFalse(t *testing.T) {
	result := colorizeHeading("Loop Control", false)
	if strings.Contains(result, "\033[") {
		t.Error("colorizeHeading with color=false should not contain ANSI codes")
	}
	if result != "Loop Control" {
		t.Errorf("colorizeHeading with color=false should return plain text, got %q", result)
	}
}

func TestColorizeFlagUsagesWithColorTrue(t *testing.T) {
	input := "      --verbose   enable verbose output\n      --log string   log file path\n"
	result := colorizeFlagUsages(input, true)
	if !strings.Contains(result, "\033[") {
		t.Error("colorizeFlagUsages with color=true should contain ANSI codes")
	}
	if !strings.Contains(result, "verbose") {
		t.Error("colorizeFlagUsages should preserve flag names")
	}
}

func TestColorizeFlagUsagesWithColorFalse(t *testing.T) {
	input := "      --verbose   enable verbose output\n      --log string   log file path\n"
	result := colorizeFlagUsages(input, false)
	if result != input {
		t.Errorf("colorizeFlagUsages with color=false should return unchanged text")
	}
}

func TestColorizeExamplesWithColorTrue(t *testing.T) {
	input := "  # run 10 iterations\n  juggle --iterations 10 'fix bugs'\n"
	result := colorizeExamples(input, true)
	if !strings.Contains(result, "\033[") {
		t.Error("colorizeExamples with color=true should contain ANSI codes")
	}
	if !strings.Contains(result, "juggle") {
		t.Error("colorizeExamples should preserve command text")
	}
}

func TestColorizeExamplesWithColorFalse(t *testing.T) {
	input := "  # run 10 iterations\n  juggle --iterations 10 'fix bugs'\n"
	result := colorizeExamples(input, false)
	if result != input {
		t.Errorf("colorizeExamples with color=false should return unchanged text")
	}
}
