package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenPowerShellCompletion(t *testing.T) {
	t.Run("output contains PowerShell Register-ArgumentCompleter", func(t *testing.T) {
		cmd := &cobra.Command{Use: "testcmd", Short: "test"}
		var buf bytes.Buffer
		genPowerShellCompletion(cmd, &buf)
		if !strings.Contains(buf.String(), "Register-ArgumentCompleter") {
			t.Errorf("expected Register-ArgumentCompleter in output, got:\n%s", buf.String())
		}
	})

	t.Run("output is non-empty", func(t *testing.T) {
		cmd := &cobra.Command{Use: "testcmd", Short: "test"}
		var buf bytes.Buffer
		genPowerShellCompletion(cmd, &buf)
		if buf.Len() == 0 {
			t.Error("expected non-empty PowerShell completion output")
		}
	})

	t.Run("command name appears in output", func(t *testing.T) {
		cmd := &cobra.Command{Use: "mycmd", Short: "test"}
		var buf bytes.Buffer
		genPowerShellCompletion(cmd, &buf)
		if !strings.Contains(buf.String(), "mycmd") {
			t.Errorf("expected command name 'mycmd' in output, got:\n%s", buf.String())
		}
	})
}
