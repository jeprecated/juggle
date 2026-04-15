package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParseServeAddr(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"8080", "127.0.0.1:8080"},
		{":8080", "127.0.0.1:8080"},
		{"0.0.0.0:8080", "0.0.0.0:8080"},
		{"127.0.0.1:8080", "127.0.0.1:8080"},
	}
	for _, tt := range tests {
		if got := parseServeAddr(tt.input); got != tt.want {
			t.Errorf("parseServeAddr(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestServeHTTPHandler(t *testing.T) {
	t.Run("POST with body returns 202 and writes trigger", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("XDG_STATE_HOME", tmp)

		handler := newServeHandler("test-serve")
		req := httptest.NewRequest(http.MethodPost, "/anything", strings.NewReader("do the thing"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}

		ibDir := inboxDir("test-serve")
		files, _ := os.ReadDir(ibDir)
		if len(files) != 1 {
			t.Fatalf("expected 1 trigger file, got %d", len(files))
		}
		content, _ := os.ReadFile(ibDir + "/" + files[0].Name())
		if string(content) != "do the thing" {
			t.Errorf("unexpected trigger content: %q", string(content))
		}
	})

	t.Run("non-POST returns 400", func(t *testing.T) {
		handler := newServeHandler("test-serve")
		req := httptest.NewRequest(http.MethodGet, "/prompt", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("POST with empty body returns 400", func(t *testing.T) {
		handler := newServeHandler("test-serve")
		req := httptest.NewRequest(http.MethodPost, "/prompt", strings.NewReader(""))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("URL path is ignored", func(t *testing.T) {
		tmp := t.TempDir()
		t.Setenv("XDG_STATE_HOME", tmp)

		handler := newServeHandler("test-serve")
		req := httptest.NewRequest(http.MethodPost, "/whatever/path.exe", strings.NewReader("msg"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}
	})
}
