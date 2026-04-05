package cli

import (
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	t.Run("file without frontmatter returns empty aliases and full content as body", func(t *testing.T) {
		content := []byte("just some prompt text\nno frontmatter here")
		fm, body := parseFrontmatter(content)
		if len(fm.Aliases) != 0 {
			t.Errorf("expected no aliases, got %v", fm.Aliases)
		}
		if string(body) != string(content) {
			t.Errorf("expected body to equal original content, got %q", body)
		}
	})

	t.Run("file with frontmatter returns aliases and stripped body", func(t *testing.T) {
		content := []byte("---\naliases: [subagents, sub]\n---\nThis is the prompt body.")
		fm, body := parseFrontmatter(content)
		if len(fm.Aliases) != 2 || fm.Aliases[0] != "subagents" || fm.Aliases[1] != "sub" {
			t.Errorf("expected [subagents sub], got %v", fm.Aliases)
		}
		if string(body) != "This is the prompt body." {
			t.Errorf("expected body without frontmatter, got %q", body)
		}
	})

	t.Run("frontmatter without aliases field returns empty aliases", func(t *testing.T) {
		content := []byte("---\ntitle: My Prompt\n---\nPrompt body here.")
		fm, body := parseFrontmatter(content)
		if len(fm.Aliases) != 0 {
			t.Errorf("expected no aliases, got %v", fm.Aliases)
		}
		if string(body) != "Prompt body here." {
			t.Errorf("expected stripped body, got %q", body)
		}
	})

	t.Run("frontmatter block with trailing newline after closing ---", func(t *testing.T) {
		content := []byte("---\naliases: [foo]\n---\n\nBody after blank line.")
		fm, body := parseFrontmatter(content)
		if len(fm.Aliases) != 1 || fm.Aliases[0] != "foo" {
			t.Errorf("expected [foo], got %v", fm.Aliases)
		}
		if string(body) != "\nBody after blank line." {
			t.Errorf("expected body with leading newline preserved, got %q", body)
		}
	})

	t.Run("content starting with --- but no closing --- is treated as no frontmatter", func(t *testing.T) {
		content := []byte("---\nthis is not closed\nno separator")
		fm, body := parseFrontmatter(content)
		if len(fm.Aliases) != 0 {
			t.Errorf("expected no aliases, got %v", fm.Aliases)
		}
		if string(body) != string(content) {
			t.Errorf("expected full content as body, got %q", body)
		}
	})

	t.Run("empty file returns empty aliases and empty body", func(t *testing.T) {
		content := []byte("")
		fm, body := parseFrontmatter(content)
		if len(fm.Aliases) != 0 {
			t.Errorf("expected no aliases, got %v", fm.Aliases)
		}
		if len(body) != 0 {
			t.Errorf("expected empty body, got %q", body)
		}
	})
}
