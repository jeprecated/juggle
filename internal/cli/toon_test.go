package cli

import (
	"strings"
	"testing"
)

func TestToonList(t *testing.T) {
	tests := []struct {
		name     string
		colName  string
		fields   []string
		rows     [][]string
		total    int
		want     string
	}{
		{
			name:    "empty rows",
			colName: "sessions",
			fields:  []string{"id", "type"},
			rows:    nil,
			want:    "sessions[0]: none found\n",
		},
		{
			name:    "single row",
			colName: "sessions",
			fields:  []string{"id", "type", "pid"},
			rows:    [][]string{{"cli-dev", "loop", "12345"}},
			want:    "sessions[1]{id,type,pid}:\n  cli-dev,loop,12345\n",
		},
		{
			name:    "multiple rows",
			colName: "tasks",
			fields:  []string{"id", "title", "status"},
			rows: [][]string{
				{"1", "Fix auth bug", "open"},
				{"2", "Add pagination", "closed"},
			},
			want: "tasks[2]{id,title,status}:\n  1,Fix auth bug,open\n  2,Add pagination,closed\n",
		},
		{
			name:    "with total count",
			colName: "runs",
			fields:  []string{"id", "status"},
			rows:    [][]string{{"abc", "done"}, {"def", "done"}},
			total:   10,
			want:    "runs[2]{id,status}:\n  abc,done\n  def,done\ncount: 2 of 10 total\n",
		},
		{
			name:    "total matches rows omitted",
			colName: "runs",
			fields:  []string{"id"},
			rows:    [][]string{{"abc"}},
			total:   1,
			want:    "runs[1]{id}:\n  abc\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			ToonList(&b, tt.colName, tt.fields, tt.rows, tt.total)
			if b.String() != tt.want {
				t.Errorf("ToonList() = %q, want %q", b.String(), tt.want)
			}
		})
	}
}

func TestToonListQuoting(t *testing.T) {
	var b strings.Builder
	ToonList(&b, "items", []string{"id", "desc"},
		[][]string{{"1", "has, comma"}}, 0)
	got := b.String()
	want := "items[1]{id,desc}:\n  1,\"has, comma\"\n"
	if got != want {
		t.Errorf("ToonList quoting = %q, want %q", got, want)
	}
}

func TestToonListTruncation(t *testing.T) {
	longVal := strings.Repeat("x", 100)
	var b strings.Builder
	ToonList(&b, "items", []string{"id", "data"},
		[][]string{{"1", longVal}}, 0)
	got := b.String()

	if !strings.Contains(got, "...(100b)") {
		t.Errorf("ToonList truncation = %q, want truncated value with size annotation", got)
	}
	if strings.Contains(got, longVal) {
		t.Error("ToonList should truncate long values")
	}
}

func TestToonObject(t *testing.T) {
	var b strings.Builder
	ToonObject(&b, []string{"id", "status"}, []string{"cli-dev", "triggered"})
	got := b.String()
	want := "id: cli-dev\nstatus: triggered\n"
	if got != want {
		t.Errorf("ToonObject() = %q, want %q", got, want)
	}
}

func TestToonError(t *testing.T) {
	var b strings.Builder
	ToonError(&b, "not_found", "session \"cli-dev\" not found")
	got := b.String()
	if !strings.Contains(got, "error: session \"cli-dev\" not found") {
		t.Errorf("ToonError() = %q, want error message", got)
	}
	if !strings.Contains(got, "code: not_found") {
		t.Errorf("ToonError() = %q, want code field", got)
	}
}

func TestToonHelp(t *testing.T) {
	var b strings.Builder
	ToonHelp(&b, []string{
		"juggle trigger <id> \"message\" to send a trigger",
		"juggle log for run history",
	})
	got := b.String()
	want := "help[2]:\n  juggle trigger <id> \"message\" to send a trigger\n  juggle log for run history\n"
	if got != want {
		t.Errorf("ToonHelp() = %q, want %q", got, want)
	}
}

func TestToonHelpEmpty(t *testing.T) {
	var b strings.Builder
	ToonHelp(&b, nil)
	if b.String() != "" {
		t.Errorf("ToonHelp(nil) = %q, want empty", b.String())
	}
}

func TestToonValueQuoting(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"has, comma", "\"has, comma\""},
		{`has "quote"`, "\"has \"\"quote\"\"\""},
		{"plain text", "plain text"},
	}

	for _, tt := range tests {
		got := toonValue(tt.input)
		if got != tt.want {
			t.Errorf("toonValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToonValueTruncation(t *testing.T) {
	short := "hello"
	if got := toonValue(short); got != short {
		t.Errorf("toonValue(short) = %q, want %q", got, short)
	}

	long := strings.Repeat("a", 100)
	got := toonValue(long)
	if !strings.HasSuffix(got, "...(100b)") {
		t.Errorf("toonValue(long) = %q, want truncated with size", got)
	}
	// Truncated portion should be 57 chars (60-3 for "...")
	prefix := got[:57]
	if prefix != strings.Repeat("a", 57) {
		t.Errorf("toonValue(long) prefix = %q (len %d), want 57 'a's", prefix, len(prefix))
	}
}
