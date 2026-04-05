package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenNushellCompletion(t *testing.T) {
	t.Run("includes extern for root command", func(t *testing.T) {
		cmd := &cobra.Command{Use: "testcmd", Short: "test"}
		var buf bytes.Buffer
		genNushellCompletion(cmd, &buf)
		if !strings.Contains(buf.String(), `extern "testcmd"`) {
			t.Errorf("expected extern block for testcmd, got:\n%s", buf.String())
		}
	})

	t.Run("bool flag has no type annotation", func(t *testing.T) {
		cmd := &cobra.Command{Use: "testcmd"}
		cmd.Flags().Bool("verbose", false, "be verbose")
		var buf bytes.Buffer
		genNushellCompletion(cmd, &buf)
		got := buf.String()
		if strings.Contains(got, "--verbose: ") {
			t.Errorf("bool flag should not have type annotation, got:\n%s", got)
		}
		if !strings.Contains(got, "--verbose") {
			t.Errorf("expected --verbose in output, got:\n%s", got)
		}
	})

	t.Run("string flag has string type annotation", func(t *testing.T) {
		cmd := &cobra.Command{Use: "testcmd"}
		cmd.Flags().String("model", "sonnet", "model name")
		var buf bytes.Buffer
		genNushellCompletion(cmd, &buf)
		if !strings.Contains(buf.String(), "--model: string") {
			t.Errorf("expected --model: string, got:\n%s", buf.String())
		}
	})

	t.Run("int flag has int type annotation", func(t *testing.T) {
		cmd := &cobra.Command{Use: "testcmd"}
		cmd.Flags().Int("count", 0, "iteration count")
		var buf bytes.Buffer
		genNushellCompletion(cmd, &buf)
		if !strings.Contains(buf.String(), "--count: int") {
			t.Errorf("expected --count: int, got:\n%s", buf.String())
		}
	})

	t.Run("flag with shorthand includes short form", func(t *testing.T) {
		cmd := &cobra.Command{Use: "testcmd"}
		cmd.Flags().StringP("model", "m", "sonnet", "model name")
		var buf bytes.Buffer
		genNushellCompletion(cmd, &buf)
		if !strings.Contains(buf.String(), "--model(-m)") {
			t.Errorf("expected --model(-m), got:\n%s", buf.String())
		}
	})

	t.Run("always includes help flag", func(t *testing.T) {
		cmd := &cobra.Command{Use: "testcmd"}
		var buf bytes.Buffer
		genNushellCompletion(cmd, &buf)
		if !strings.Contains(buf.String(), "--help(-h)") {
			t.Errorf("expected --help(-h) in output, got:\n%s", buf.String())
		}
	})

	t.Run("includes extern for subcommands", func(t *testing.T) {
		root := &cobra.Command{Use: "testcmd"}
		sub := &cobra.Command{Use: "subcmd", Short: "sub"}
		root.AddCommand(sub)
		var buf bytes.Buffer
		genNushellCompletion(root, &buf)
		if !strings.Contains(buf.String(), `extern "testcmd subcmd"`) {
			t.Errorf("expected extern for subcommand, got:\n%s", buf.String())
		}
	})

	t.Run("hidden flags are excluded", func(t *testing.T) {
		cmd := &cobra.Command{Use: "testcmd"}
		cmd.Flags().Bool("secret", false, "hidden flag")
		_ = cmd.Flags().MarkHidden("secret")
		var buf bytes.Buffer
		genNushellCompletion(cmd, &buf)
		if strings.Contains(buf.String(), "--secret") {
			t.Errorf("hidden flag should not appear in completion, got:\n%s", buf.String())
		}
	})
}
