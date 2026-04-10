package pipeline_test

import (
	"testing"

	"github.com/ohare93/juggle/internal/pipeline"
)

func TestEvalStructured_IterationOperators(t *testing.T) {
	tests := []struct {
		expr      string
		iteration int
		want      bool
	}{
		{"iteration==3", 3, true},
		{"iteration==3", 2, false},
		{"iteration!=3", 2, true},
		{"iteration!=3", 3, false},
		{"iteration>2", 3, true},
		{"iteration>3", 3, false},
		{"iteration>=3", 3, true},
		{"iteration>=2", 3, true},
		{"iteration>=4", 3, false},
		{"iteration<5", 3, true},
		{"iteration<3", 3, false},
		{"iteration<=3", 3, true},
		{"iteration<=2", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			wctx := pipeline.WhenContext{Iteration: tt.iteration}
			got, matched, err := pipeline.EvalStructured(tt.expr, wctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !matched {
				t.Fatal("expected expression to be matched as structured")
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalStructured_IterationWithSpaces(t *testing.T) {
	wctx := pipeline.WhenContext{Iteration: 3}
	got, matched, err := pipeline.EvalStructured("iteration == 3", wctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Fatal("expected matched")
	}
	if !got {
		t.Error("expected true")
	}
}

func TestEvalStructured_Success(t *testing.T) {
	tests := []struct {
		expr        string
		prevSuccess bool
		want        bool
	}{
		{"success==true", true, true},
		{"success==true", false, false},
		{"success==false", false, true},
		{"success==false", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			wctx := pipeline.WhenContext{PrevSuccess: tt.prevSuccess}
			got, matched, err := pipeline.EvalStructured(tt.expr, wctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !matched {
				t.Fatal("expected matched")
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalStructured_ExitCode(t *testing.T) {
	tests := []struct {
		expr     string
		exitCode int
		want     bool
	}{
		{"exit_code==0", 0, true},
		{"exit_code==0", 1, false},
		{"exit_code==1", 1, true},
		{"exit_code==2", 2, true},
		{"exit_code==2", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			wctx := pipeline.WhenContext{PrevExitCode: tt.exitCode}
			got, matched, err := pipeline.EvalStructured(tt.expr, wctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !matched {
				t.Fatal("expected matched")
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalStructured_Fallback_ShellExpressions(t *testing.T) {
	exprs := []string{
		"true",
		"false",
		"[ -f foo.txt ]",
		"test -d /tmp",
		"echo hello",
		"exit 0",
	}

	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			_, matched, err := pipeline.EvalStructured(expr, pipeline.WhenContext{})
			if err != nil {
				t.Fatalf("unexpected error for shell expression %q: %v", expr, err)
			}
			if matched {
				t.Errorf("expected shell expression %q to not match structured grammar", expr)
			}
		})
	}
}

func TestEvalStructured_ParseErrors(t *testing.T) {
	exprs := []string{
		"iteration==abc",
		"iteration>",
		"iteration ==",
		"success==maybe",
		"success==",
		"exit_code==abc",
		"exit_code==",
	}

	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			_, _, err := pipeline.EvalStructured(expr, pipeline.WhenContext{})
			if err == nil {
				t.Errorf("expected parse error for %q", expr)
			}
		})
	}
}
