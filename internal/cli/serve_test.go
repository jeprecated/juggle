package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateServeFilename(t *testing.T) {
	t.Run("format is YYYYMMDD-HHMMSS-id.ext", func(t *testing.T) {
		ts := time.Date(2024, 3, 15, 14, 30, 5, 0, time.UTC)
		name := generateServeFilename(ts, "abc123", ".md")
		if name != "20240315-143005-abc123.md" {
			t.Errorf("unexpected filename: %q", name)
		}
	})

	t.Run("supports txt extension", func(t *testing.T) {
		ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		name := generateServeFilename(ts, "xyz", ".txt")
		if name != "20240101-000000-xyz.txt" {
			t.Errorf("unexpected filename: %q", name)
		}
	})

	t.Run("pads single-digit time components", func(t *testing.T) {
		ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		name := generateServeFilename(ts, "id", ".json")
		if name != "20240102-030405-id.json" {
			t.Errorf("unexpected filename: %q", name)
		}
	})
}

func TestServeHTTPHandler(t *testing.T) {
	t.Run("POST /prompt.md writes file and returns 202", func(t *testing.T) {
		dir := t.TempDir()
		handler := newServeHandler(dir)

		req := httptest.NewRequest(http.MethodPost, "/prompt.md", strings.NewReader("do the thing"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}

		files, _ := os.ReadDir(dir)
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if ext := filepath.Ext(files[0].Name()); ext != ".md" {
			t.Errorf("expected .md extension, got %q", ext)
		}
		content, _ := os.ReadFile(filepath.Join(dir, files[0].Name()))
		if string(content) != "do the thing" {
			t.Errorf("unexpected file content: %q", string(content))
		}
	})

	t.Run("POST /prompt.txt writes .txt file", func(t *testing.T) {
		dir := t.TempDir()
		handler := newServeHandler(dir)

		req := httptest.NewRequest(http.MethodPost, "/prompt.txt", strings.NewReader("content"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}
		files, _ := os.ReadDir(dir)
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if ext := filepath.Ext(files[0].Name()); ext != ".txt" {
			t.Errorf("expected .txt extension, got %q", ext)
		}
	})

	t.Run("POST /prompt.json writes .json file", func(t *testing.T) {
		dir := t.TempDir()
		handler := newServeHandler(dir)

		req := httptest.NewRequest(http.MethodPost, "/prompt.json", strings.NewReader(`{"task":"go"}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Errorf("expected 202, got %d", rec.Code)
		}
		files, _ := os.ReadDir(dir)
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if ext := filepath.Ext(files[0].Name()); ext != ".json" {
			t.Errorf("expected .json extension, got %q", ext)
		}
	})

	t.Run("unsupported path returns 404", func(t *testing.T) {
		dir := t.TempDir()
		handler := newServeHandler(dir)

		req := httptest.NewRequest(http.MethodPost, "/anything.exe", strings.NewReader("x"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("GET returns 405", func(t *testing.T) {
		dir := t.TempDir()
		handler := newServeHandler(dir)

		req := httptest.NewRequest(http.MethodGet, "/prompt.md", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("filename has YYYYMMDD-HHMMSS-id structure", func(t *testing.T) {
		dir := t.TempDir()
		handler := newServeHandler(dir)

		req := httptest.NewRequest(http.MethodPost, "/prompt.md", strings.NewReader("x"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		files, _ := os.ReadDir(dir)
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		name := strings.TrimSuffix(files[0].Name(), ".md")
		parts := strings.SplitN(name, "-", 3)
		if len(parts) != 3 {
			t.Errorf("filename %q should have 3 dash-separated parts", files[0].Name())
			return
		}
		if len(parts[0]) != 8 {
			t.Errorf("date part %q should be 8 chars (YYYYMMDD)", parts[0])
		}
		if len(parts[1]) != 6 {
			t.Errorf("time part %q should be 6 chars (HHMMSS)", parts[1])
		}
		if len(parts[2]) == 0 {
			t.Errorf("id part should not be empty")
		}
	})

	t.Run("response body is empty", func(t *testing.T) {
		dir := t.TempDir()
		handler := newServeHandler(dir)

		req := httptest.NewRequest(http.MethodPost, "/prompt.md", strings.NewReader("x"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if body := rec.Body.String(); body != "" {
			t.Errorf("expected empty response body, got %q", body)
		}
	})
}
