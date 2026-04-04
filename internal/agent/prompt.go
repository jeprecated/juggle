// Package agent provides the agent prompt template and related utilities
// for running AI agents with juggle.
package agent

import (
	_ "embed"
)

//go:embed prompt.md
var PromptTemplate string

//go:embed manual_prompt.md
var ManualPromptTemplate string

//go:embed watch_prompt.md
var WatchPromptTemplate string

// GetPromptTemplate returns the embedded agent prompt template.
func GetPromptTemplate() string {
	return PromptTemplate
}

// GetManualPromptTemplate returns the embedded manual mode prompt template.
func GetManualPromptTemplate() string {
	return ManualPromptTemplate
}

// GetWatchPromptTemplate returns the embedded watch mode prompt template.
func GetWatchPromptTemplate() string {
	return WatchPromptTemplate
}
