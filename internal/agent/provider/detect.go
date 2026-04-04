package provider

import (
	"os/exec"
)

// Detect determines the provider type based on config settings.
// Resolution order (highest to lowest priority):
//  1. CLI flag override (if set)
//  2. Project config (if set)
//  3. Global config (if set)
//  4. Default: claude
func Detect(cliOverride, projectProvider, globalProvider string) Type {
	// CLI flag has highest priority
	if cliOverride != "" {
		t := Type(cliOverride)
		if t.IsValid() {
			return t
		}
	}

	// Project config overrides global
	if projectProvider != "" {
		t := Type(projectProvider)
		if t.IsValid() {
			return t
		}
	}

	// Global config
	if globalProvider != "" {
		t := Type(globalProvider)
		if t.IsValid() {
			return t
		}
	}

	// Default to Claude
	return TypeClaude
}

// IsAvailable checks if a provider's binary is available in PATH
func IsAvailable(p Type) bool {
	binary := BinaryName(p)
	if binary == "" {
		return false
	}
	_, err := exec.LookPath(binary)
	return err == nil
}

// BinaryName returns the executable name for a provider
func BinaryName(p Type) string {
	switch p {
	case TypeClaude:
		return "claude"
	case TypeOpenCode:
		return "opencode"
	case TypeCodex:
		return "codex"
	default:
		return ""
	}
}

// Get returns the appropriate provider implementation for the given type
func Get(providerType Type) Provider {
	switch providerType {
	case TypeOpenCode:
		return NewOpenCodeProvider()
	case TypeCodex:
		return NewCodexProvider()
	case TypeClaude:
		fallthrough
	default:
		return NewClaudeProvider()
	}
}

// GetWithDetection returns a provider using the detection logic
func GetWithDetection(cliOverride, projectProvider, globalProvider string) Provider {
	providerType := Detect(cliOverride, projectProvider, globalProvider)
	return Get(providerType)
}

// ModelOverrides allows custom model mappings from config
type ModelOverrides map[string]string

// ApplyModelOverrides returns the model string, applying overrides if configured
// Priority: override > provider default mapping
func ApplyModelOverrides(canonical string, overrides ModelOverrides, p Provider) string {
	// Check for override first
	if overrides != nil {
		if override, ok := overrides[canonical]; ok {
			return override
		}
	}

	// Fall back to provider's default mapping
	return p.MapModel(canonical)
}

// ValidProviders returns the list of valid provider type strings
func ValidProviders() []string {
	return []string{
		string(TypeClaude),
		string(TypeOpenCode),
		string(TypeCodex),
	}
}
