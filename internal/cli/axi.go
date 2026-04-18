package cli

import "os"

// OutputFormat controls how agent-facing commands format their output.
type OutputFormat string

const (
	FormatText OutputFormat = "text"
	FormatToon OutputFormat = "toon"
	FormatJSON OutputFormat = "json"
)

// outputFormat determines the output format from the --format flag,
// falling back to the JUGGLE_FORMAT environment variable, then "text".
func outputFormat() OutputFormat {
	if flags.format != "" {
		return OutputFormat(flags.format)
	}
	if env := os.Getenv("JUGGLE_FORMAT"); env != "" {
		return OutputFormat(env)
	}
	return FormatText
}

// agentFormat is like outputFormat but defaults to "toon" instead of "text".
// Use for commands designed to be called by agents (e.g. trigger).
func agentFormat() OutputFormat {
	if flags.format != "" {
		return OutputFormat(flags.format)
	}
	if env := os.Getenv("JUGGLE_FORMAT"); env != "" {
		return OutputFormat(env)
	}
	return FormatToon
}
