package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPipelineSubcommandIsRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pipeline"})
	if err != nil || cmd == nil {
		t.Fatal("pipeline subcommand not registered on rootCmd")
	}
	if cmd.Name() != "pipeline" {
		t.Errorf("expected command name %q, got %q", "pipeline", cmd.Name())
	}
}

func TestPipelineHelpContainsPipelineFocusedText(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pipeline"})
	if err != nil || cmd == nil {
		t.Skip("pipeline subcommand not found")
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	defer cmd.SetOut(os.Stdout)
	if err := cmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"pipeline", "agent", "cmd"} {
		if !strings.Contains(out, want) {
			t.Errorf("pipeline help output missing %q", want)
		}
	}
}

func TestRootHelpListsPipelineSubcommand(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	defer rootCmd.SetOut(os.Stdout)
	if err := rootCmd.Help(); err != nil {
		t.Fatalf("Help() error: %v", err)
	}
	if !strings.Contains(buf.String(), "pipeline") {
		t.Error("root help should list pipeline subcommand")
	}
}
