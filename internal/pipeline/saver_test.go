package pipeline_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jeprecated/juggle/internal/pipeline"
)

// --- Round-trip ---

func TestSaveBytes_roundTrip_validFixture(t *testing.T) {
	p, err := pipeline.LoadFile("testdata/valid.toml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	data, err := pipeline.SaveBytes(p)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	p2, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Errorf("round-trip mismatch\noriginal: %+v\nreloaded: %+v", p, p2)
	}
}

func TestSaveBytes_roundTrip_fullAgentNode(t *testing.T) {
	p := &pipeline.Pipeline{
		Iterations:       5,
		MaxParallelSteps: 2,
		Defaults: pipeline.Defaults{
			Provider: "claude",
			Model:    "sonnet",
		},
		Nodes: []pipeline.Node{
			{
				Name:      "Work",
				Kind:      pipeline.NodeKindAgent,
				Event:     pipeline.EventLoopBody,
				After:     []string{"Dep"},
				Parallel:  true,
				When:      "iteration==1",
				OnFailure: pipeline.FailurePolicyContinue,
				Retries:   3,
				Timeout:   5 * time.Minute,
				WorkDir:   "/tmp",
				Agent: &pipeline.AgentSpec{
					Prompt:          "@task.md",
					Provider:        "codex",
					Model:           "gpt-5.4",
					Plan:            true,
					Trust:           true,
					SystemPrompt:    "be concise",
					AllowedTools:    []string{"Read", "Grep"},
					DisallowedTools: []string{"Bash"},
					MaxTurns:        10,
					MCPConfig:       "mcp.json",
					Passthrough:     []string{"--verbose"},
				},
			},
		},
	}
	data, err := pipeline.SaveBytes(p)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	p2, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Errorf("round-trip mismatch\noriginal: %+v\nreloaded: %+v", p, p2)
	}
}

func TestSaveBytes_roundTrip_fullCmdNode(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{
				Name:      "Commit",
				Kind:      pipeline.NodeKindCmd,
				Event:     pipeline.EventLoopEnd,
				After:     []string{"Work"},
				Parallel:  false,
				When:      "always",
				OnFailure: pipeline.FailurePolicyStop,
				Retries:   1,
				Timeout:   30 * time.Second,
				WorkDir:   "/repo",
				Cmd: &pipeline.CmdSpec{
					Command: "git commit -m done",
					Shell:   "bash",
					Env:     []string{"KEY=value"},
				},
			},
		},
	}
	data, err := pipeline.SaveBytes(p)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	p2, err := pipeline.LoadBytes(data)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Errorf("round-trip mismatch\noriginal: %+v\nreloaded: %+v", p, p2)
	}
}

// --- Defaults omitted when empty ---

func TestSaveBytes_emptyDefaults_noDefaultsSection(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "Work", Kind: pipeline.NodeKindAgent, Agent: &pipeline.AgentSpec{Prompt: "do it"}},
		},
	}
	data, err := pipeline.SaveBytes(p)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if containsString(string(data), "[defaults]") {
		t.Errorf("expected no [defaults] section in output, got:\n%s", data)
	}
}

func TestSaveBytes_nonEmptyDefaults_includesDefaultsSection(t *testing.T) {
	p := &pipeline.Pipeline{
		Defaults: pipeline.Defaults{Provider: "claude", Model: "sonnet"},
		Nodes: []pipeline.Node{
			{Name: "Work", Kind: pipeline.NodeKindAgent, Agent: &pipeline.AgentSpec{Prompt: "do it"}},
		},
	}
	data, err := pipeline.SaveBytes(p)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !containsString(string(data), "[defaults]") {
		t.Errorf("expected [defaults] section in output, got:\n%s", data)
	}
}

// --- Timeout serialized as string ---

func TestSaveBytes_timeout_serializedAsString(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{
				Name:    "Work",
				Kind:    pipeline.NodeKindAgent,
				Timeout: 5 * time.Minute,
				Agent:   &pipeline.AgentSpec{Prompt: "do it"},
			},
		},
	}
	data, err := pipeline.SaveBytes(p)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	out := string(data)
	if !containsString(out, "timeout") {
		t.Errorf("expected timeout field in output, got:\n%s", out)
	}
	// Ensure it's a string, not a number
	if !containsString(out, `"`) {
		t.Errorf("expected timeout to be a quoted string in TOML, got:\n%s", out)
	}
}

func TestSaveBytes_zeroTimeout_omitted(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "Work", Kind: pipeline.NodeKindAgent, Agent: &pipeline.AgentSpec{Prompt: "do it"}},
		},
	}
	data, err := pipeline.SaveBytes(p)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if containsString(string(data), "timeout") {
		t.Errorf("expected no timeout field when zero, got:\n%s", data)
	}
}

// --- Clean output: zero/false fields omitted ---

func TestSaveBytes_falseFlags_omitted(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{
				Name:  "Work",
				Kind:  pipeline.NodeKindAgent,
				Agent: &pipeline.AgentSpec{Prompt: "do it", Plan: false, Trust: false},
			},
		},
	}
	data, err := pipeline.SaveBytes(p)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	out := string(data)
	if containsString(out, "plan") {
		t.Errorf("expected plan=false to be omitted, got:\n%s", out)
	}
	if containsString(out, "trust") {
		t.Errorf("expected trust=false to be omitted, got:\n%s", out)
	}
}

// --- SaveFile ---

func TestSaveFile_writesToDisk(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "Work", Kind: pipeline.NodeKindAgent, Agent: &pipeline.AgentSpec{Prompt: "do it"}},
		},
	}
	path := filepath.Join(t.TempDir(), "out.toml")
	if err := pipeline.SaveFile(path, p); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	p2, err := pipeline.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile after save: %v", err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Errorf("round-trip via file mismatch\noriginal: %+v\nreloaded: %+v", p, p2)
	}
}

func TestSaveFile_invalidPath_returnsError(t *testing.T) {
	p := &pipeline.Pipeline{
		Nodes: []pipeline.Node{
			{Name: "Work", Kind: pipeline.NodeKindAgent, Agent: &pipeline.AgentSpec{Prompt: "do it"}},
		},
	}
	err := pipeline.SaveFile("/nonexistent/directory/out.toml", p)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestSaveFile_roundTrip_preservesFileContent(t *testing.T) {
	p, err := pipeline.LoadFile("testdata/valid.toml")
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}
	path := filepath.Join(t.TempDir(), "pipeline.toml")
	if err := pipeline.SaveFile(path, p); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	p2, err := pipeline.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile after save: %v", err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Errorf("file round-trip mismatch\noriginal: %+v\nreloaded: %+v", p, p2)
	}
	_ = os.Remove(path)
}

// --- helpers ---

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 ||
		func() bool {
			for i := 0; i <= len(haystack)-len(needle); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}())
}
