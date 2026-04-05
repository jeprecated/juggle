package cli

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// promptFrontmatter holds parsed YAML frontmatter fields for prompt files.
type promptFrontmatter struct {
	Aliases []string `yaml:"aliases"`
}

// parseFrontmatter splits YAML frontmatter from prompt file content.
// Frontmatter must be delimited by "---" on its own line at the start of the file.
// Returns the parsed frontmatter and the remaining body content.
// If no frontmatter is present, returns empty frontmatter and the original content unchanged.
func parseFrontmatter(content []byte) (promptFrontmatter, []byte) {
	var fm promptFrontmatter

	if !bytes.HasPrefix(content, []byte("---\n")) {
		return fm, content
	}

	// Find closing ---
	rest := content[4:] // skip opening "---\n"
	idx := bytes.Index(rest, []byte("\n---\n"))
	if idx < 0 {
		// Also check for "---" at end of file
		if !bytes.HasSuffix(rest, []byte("\n---")) {
			return fm, content
		}
		yamlBlock := rest[:len(rest)-4]
		_ = yaml.Unmarshal(yamlBlock, &fm)
		return fm, []byte{}
	}

	yamlBlock := rest[:idx]
	body := rest[idx+5:] // skip "\n---\n"

	_ = yaml.Unmarshal(yamlBlock, &fm)
	return fm, body
}
