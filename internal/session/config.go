package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultConfigPath = ".juggle/config.json"

	// Default values for global configuration fields
	// These are documented here as the canonical source of defaults
	DefaultIterationDelayMinutes = 0  // No delay between agent iterations by default
	DefaultIterationDelayFuzz    = 0  // No variance in delay by default
	DefaultOverloadRetryMinutes  = 10 // Wait 10 minutes before retrying after 529 overload exhaustion

	// EnvConfigHome is the environment variable that overrides the config home directory.
	// When set, all config operations will use this path instead of ~/.juggle.
	// This is primarily used for testing to prevent polluting the user's real config.
	EnvConfigHome = "JUGGLER_CONFIG_HOME"
)

// ConfigOptions holds configurable options for loading global config.
type ConfigOptions struct {
	ConfigHome    string // Override for ~/.juggle directory
	JuggleDirName string // Name of the juggle directory (default: ".juggle")
}

// DefaultConfigOptions returns the default config options.
// If JUGGLER_CONFIG_HOME is set, it will be used instead of the user's home directory.
func DefaultConfigOptions() ConfigOptions {
	configHome := os.Getenv(EnvConfigHome)
	if configHome == "" {
		configHome, _ = os.UserHomeDir()
	}
	return ConfigOptions{
		ConfigHome:    configHome,
		JuggleDirName: ".juggle",
	}
}

// Config holds global juggle configuration stored at ~/.juggle/config.json.
//
// Global configuration includes:
//   - SearchPaths: directories to scan for juggle projects
//   - IterationDelayMinutes/IterationDelayFuzz: pacing between agent runs
//   - OverloadRetryMinutes: wait time after rate limit exhaustion
//   - VCS: preferred version control system (git/jj)
//
// Unknown fields in the config file are preserved to prevent data loss
// when older juggle versions read configs written by newer versions.
//
// Use LoadConfig() to read the config, and config.Save() to write changes.
type Config struct {
	SearchPaths []string `json:"search_paths"`
	// Agent iteration delay settings
	IterationDelayMinutes int `json:"iteration_delay_minutes,omitempty"` // Base delay between iterations in minutes
	IterationDelayFuzz    int `json:"iteration_delay_fuzz,omitempty"`    // Random +/- variance in minutes
	// Overload retry settings (for 529 errors after Claude's built-in retries exhaust)
	OverloadRetryMinutes int `json:"overload_retry_minutes,omitempty"` // Minutes to wait before retrying after 529 overload exhaustion
	// VCS settings
	VCS string `json:"vcs,omitempty"` // Version control system: "git" or "jj"

	// Agent provider settings
	AgentProvider  string            `json:"agent_provider,omitempty"`  // Agent CLI: "claude" or "opencode"
	ModelOverrides map[string]string `json:"model_overrides,omitempty"` // Custom model mappings (e.g., "opus": "anthropic/claude-opus-5")

	// UnknownFields stores any fields from the config file that aren't recognized.
	// These are preserved when saving to avoid data loss.
	UnknownFields map[string]interface{} `json:"-"`
}

// knownConfigFields lists the field names we recognize in config JSON
var knownConfigFields = map[string]bool{
	"search_paths":            true,
	"iteration_delay_minutes": true,
	"iteration_delay_fuzz":    true,
	"overload_retry_minutes":  true,
	"vcs":                     true,
	"agent_provider":          true,
	"model_overrides":         true,
}

// UnmarshalJSON implements custom JSON unmarshaling to capture unknown fields
func (c *Config) UnmarshalJSON(data []byte) error {
	// First, unmarshal into a map to capture all fields
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// Unmarshal known fields using a type alias to avoid recursion
	type configAlias Config
	var alias configAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	// Copy known fields
	c.SearchPaths = alias.SearchPaths
	c.IterationDelayMinutes = alias.IterationDelayMinutes
	c.IterationDelayFuzz = alias.IterationDelayFuzz
	c.OverloadRetryMinutes = alias.OverloadRetryMinutes
	c.VCS = alias.VCS
	c.AgentProvider = alias.AgentProvider
	c.ModelOverrides = alias.ModelOverrides

	// Extract unknown fields
	c.UnknownFields = make(map[string]interface{})
	for key, value := range rawMap {
		if !knownConfigFields[key] {
			c.UnknownFields[key] = value
		}
	}

	return nil
}

// MarshalJSON implements custom JSON marshaling to preserve unknown fields
func (c *Config) MarshalJSON() ([]byte, error) {
	// Start with unknown fields
	result := make(map[string]interface{})
	for key, value := range c.UnknownFields {
		result[key] = value
	}

	// Add known fields (they take precedence over unknown fields with same name)
	result["search_paths"] = c.SearchPaths
	if c.IterationDelayMinutes != 0 {
		result["iteration_delay_minutes"] = c.IterationDelayMinutes
	}
	if c.IterationDelayFuzz != 0 {
		result["iteration_delay_fuzz"] = c.IterationDelayFuzz
	}
	if c.OverloadRetryMinutes != 0 {
		result["overload_retry_minutes"] = c.OverloadRetryMinutes
	}
	if c.VCS != "" {
		result["vcs"] = c.VCS
	}
	if c.AgentProvider != "" {
		result["agent_provider"] = c.AgentProvider
	}
	if len(c.ModelOverrides) > 0 {
		result["model_overrides"] = c.ModelOverrides
	}

	return json.Marshal(result)
}

// GetUnknownFields returns the list of unrecognized field names
func (c *Config) GetUnknownFields() []string {
	keys := make([]string, 0, len(c.UnknownFields))
	for key := range c.UnknownFields {
		keys = append(keys, key)
	}
	return keys
}

// DefaultConfig returns a configuration with common project locations
// DefaultConfig returns a configuration with empty search paths
// Projects are added automatically when balls are created
func DefaultConfig() *Config {
	return &Config{
		SearchPaths:           []string{},
		IterationDelayMinutes: DefaultIterationDelayMinutes,
		IterationDelayFuzz:    DefaultIterationDelayFuzz,
		OverloadRetryMinutes:  DefaultOverloadRetryMinutes,
		UnknownFields:         make(map[string]interface{}),
	}
}

// LoadConfig loads configuration from ~/.juggle/config.json
func LoadConfig() (*Config, error) {
	return LoadConfigWithOptions(DefaultConfigOptions())
}

// LoadConfigWithOptions loads configuration with custom options.
// If the config file doesn't exist, creates a default config and saves it.
// Reading an existing config does NOT automatically write it back.
func LoadConfigWithOptions(opts ConfigOptions) (*Config, error) {
	if opts.ConfigHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		opts.ConfigHome = home
	}

	configPath := filepath.Join(opts.ConfigHome, opts.JuggleDirName, "config.json")

	// If config doesn't exist, create with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if err := config.SaveWithOptions(opts); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Ensure UnknownFields map is initialized
	if config.UnknownFields == nil {
		config.UnknownFields = make(map[string]interface{})
	}

	return &config, nil
}

// Save persists the configuration to disk
func (c *Config) Save() error {
	return c.SaveWithOptions(DefaultConfigOptions())
}

// SaveWithOptions persists the configuration with custom options
func (c *Config) SaveWithOptions(opts ConfigOptions) error {
	if opts.ConfigHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		opts.ConfigHome = home
	}

	configPath := filepath.Join(opts.ConfigHome, opts.JuggleDirName, "config.json")
	configDir := filepath.Dir(configPath)

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// AddSearchPath adds a new search path if it doesn't already exist
func (c *Config) AddSearchPath(path string) bool {
	for _, existing := range c.SearchPaths {
		if existing == path {
			return false // Already exists
		}
	}
	c.SearchPaths = append(c.SearchPaths, path)
	return true
}

// RemoveSearchPath removes a search path
func (c *Config) RemoveSearchPath(path string) bool {
	for i, existing := range c.SearchPaths {
		if existing == path {
			c.SearchPaths = append(c.SearchPaths[:i], c.SearchPaths[i+1:]...)
			return true
		}
	}
	return false
}

// SetIterationDelay sets the delay between agent iterations.
// delayMinutes is the base delay in minutes, fuzz is the +/- variance in minutes.
func (c *Config) SetIterationDelay(delayMinutes, fuzz int) {
	c.IterationDelayMinutes = delayMinutes
	c.IterationDelayFuzz = fuzz
}

// GetIterationDelay returns the delay settings (delayMinutes, fuzz).
// Returns (0, 0) if not configured.
func (c *Config) GetIterationDelay() (delayMinutes, fuzz int) {
	return c.IterationDelayMinutes, c.IterationDelayFuzz
}

// HasIterationDelay returns true if iteration delay is configured.
func (c *Config) HasIterationDelay() bool {
	return c.IterationDelayMinutes > 0
}

// ClearIterationDelay removes the iteration delay configuration.
func (c *Config) ClearIterationDelay() {
	c.IterationDelayMinutes = 0
	c.IterationDelayFuzz = 0
}

// SetVCS sets the global VCS preference.
// Valid values are "git", "jj", or "" (empty for auto-detect).
func (c *Config) SetVCS(vcs string) error {
	if vcs != "" && vcs != "git" && vcs != "jj" {
		return fmt.Errorf("invalid VCS type: %s (must be 'git' or 'jj')", vcs)
	}
	c.VCS = vcs
	return nil
}

// GetVCS returns the global VCS preference.
func (c *Config) GetVCS() string {
	return c.VCS
}

// ClearVCS removes the VCS preference, enabling auto-detection.
func (c *Config) ClearVCS() {
	c.VCS = ""
}

// EnsureProjectInSearchPaths ensures a project directory is in the search paths
// This is called when creating balls to automatically track the project
func EnsureProjectInSearchPaths(projectDir string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Add the path if not already present
	if config.AddSearchPath(projectDir) {
		if err := config.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	return nil
}

// ProjectConfig holds per-project configuration stored in .juggle/config.json.
//
// Project configuration includes:
//   - DefaultAcceptanceCriteria: repo-level ACs applied to all balls
//   - ACTemplates: optional AC suggestions shown during ball creation
//   - VCS: project-specific VCS preference (overrides global)
//   - AgentProvider: project-specific agent CLI (overrides global)
//   - ModelOverrides: project-specific model mappings (merged with global)
//   - RunAliases: named command aliases for `juggle worktree run`
//
// These settings apply to all balls and sessions within the project.
type ProjectConfig struct {
	DefaultAcceptanceCriteria []string          `json:"default_acceptance_criteria,omitempty"` // Repo-level ACs applied to all sessions
	ACTemplates               []string          `json:"ac_templates,omitempty"`                // Optional AC templates shown during ball creation
	VCS                       string            `json:"vcs,omitempty"`                         // Version control system: "git" or "jj"
	AgentProvider             string            `json:"agent_provider,omitempty"`              // Agent CLI: "claude" or "opencode"
	ModelOverrides            map[string]string `json:"model_overrides,omitempty"`             // Custom model mappings
	RunAliases                map[string]string `json:"run_aliases,omitempty"`                 // Named command aliases for worktree run
}

// DefaultProjectConfig returns a new project config with initial values
func DefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{}
}

// LoadProjectConfig loads the project configuration from projectDir/.juggle/config.json
func LoadProjectConfig(projectDir string) (*ProjectConfig, error) {
	configPath := filepath.Join(projectDir, projectStorePath, "config.json")

	// If config doesn't exist, create with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultProjectConfig()
		if err := SaveProjectConfig(projectDir, config); err != nil {
			return nil, fmt.Errorf("failed to create default project config: %w", err)
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project config file: %w", err)
	}

	var config ProjectConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project config: %w", err)
	}

	return &config, nil
}

// SaveProjectConfig saves the project configuration to projectDir/.juggle/config.json
func SaveProjectConfig(projectDir string, config *ProjectConfig) error {
	configDir := filepath.Join(projectDir, projectStorePath)
	configPath := filepath.Join(configDir, "config.json")

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create project config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write project config file: %w", err)
	}

	return nil
}

// SetDefaultAcceptanceCriteria sets repo-level acceptance criteria
func (c *ProjectConfig) SetDefaultAcceptanceCriteria(criteria []string) {
	c.DefaultAcceptanceCriteria = criteria
}

// HasDefaultAcceptanceCriteria returns true if the project has default ACs
func (c *ProjectConfig) HasDefaultAcceptanceCriteria() bool {
	return len(c.DefaultAcceptanceCriteria) > 0
}

// SetACTemplates sets the optional AC templates for ball creation
func (c *ProjectConfig) SetACTemplates(templates []string) {
	c.ACTemplates = templates
}

// HasACTemplates returns true if the project has AC templates
func (c *ProjectConfig) HasACTemplates() bool {
	return len(c.ACTemplates) > 0
}

// GetACTemplates returns the AC templates
func (c *ProjectConfig) GetACTemplates() []string {
	return c.ACTemplates
}

// SetVCS sets the project VCS preference.
// Valid values are "git", "jj", or "" (empty for inherit from global/auto-detect).
func (c *ProjectConfig) SetVCS(vcs string) error {
	if vcs != "" && vcs != "git" && vcs != "jj" {
		return fmt.Errorf("invalid VCS type: %s (must be 'git' or 'jj')", vcs)
	}
	c.VCS = vcs
	return nil
}

// GetVCS returns the project VCS preference.
func (c *ProjectConfig) GetVCS() string {
	return c.VCS
}

// ClearVCS removes the project VCS preference.
func (c *ProjectConfig) ClearVCS() {
	c.VCS = ""
}

// SetRunAlias sets a run alias for worktree run commands.
func (c *ProjectConfig) SetRunAlias(name, command string) {
	if c.RunAliases == nil {
		c.RunAliases = make(map[string]string)
	}
	c.RunAliases[name] = command
}

// GetRunAlias returns the command for a run alias, or empty if not found.
func (c *ProjectConfig) GetRunAlias(name string) string {
	if c.RunAliases == nil {
		return ""
	}
	return c.RunAliases[name]
}

// GetRunAliases returns all run aliases.
func (c *ProjectConfig) GetRunAliases() map[string]string {
	return c.RunAliases
}

// DeleteRunAlias removes a run alias.
func (c *ProjectConfig) DeleteRunAlias(name string) bool {
	if c.RunAliases == nil {
		return false
	}
	if _, exists := c.RunAliases[name]; exists {
		delete(c.RunAliases, name)
		return true
	}
	return false
}

// HasRunAliases returns true if any run aliases are defined.
func (c *ProjectConfig) HasRunAliases() bool {
	return len(c.RunAliases) > 0
}

// UpdateProjectAcceptanceCriteria updates the repo-level acceptance criteria
func UpdateProjectAcceptanceCriteria(projectDir string, criteria []string) error {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return err
	}

	config.SetDefaultAcceptanceCriteria(criteria)
	return SaveProjectConfig(projectDir, config)
}

// GetProjectAcceptanceCriteria returns the repo-level acceptance criteria
func GetProjectAcceptanceCriteria(projectDir string) ([]string, error) {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return nil, err
	}

	return config.DefaultAcceptanceCriteria, nil
}

// UpdateProjectACTemplates updates the AC templates for ball creation
func UpdateProjectACTemplates(projectDir string, templates []string) error {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return err
	}

	config.SetACTemplates(templates)
	return SaveProjectConfig(projectDir, config)
}

// GetProjectACTemplates returns the AC templates for ball creation
func GetProjectACTemplates(projectDir string) ([]string, error) {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return nil, err
	}

	return config.ACTemplates, nil
}

// UpdateGlobalIterationDelay updates the iteration delay in global config
func UpdateGlobalIterationDelay(delayMinutes, fuzz int) error {
	return UpdateGlobalIterationDelayWithOptions(DefaultConfigOptions(), delayMinutes, fuzz)
}

// UpdateGlobalIterationDelayWithOptions updates the iteration delay with custom options
func UpdateGlobalIterationDelayWithOptions(opts ConfigOptions, delayMinutes, fuzz int) error {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return err
	}

	config.SetIterationDelay(delayMinutes, fuzz)
	return config.SaveWithOptions(opts)
}

// GetGlobalIterationDelay returns the iteration delay settings from global config
func GetGlobalIterationDelay() (delayMinutes, fuzz int, err error) {
	return GetGlobalIterationDelayWithOptions(DefaultConfigOptions())
}

// GetGlobalIterationDelayWithOptions returns the iteration delay with custom options
func GetGlobalIterationDelayWithOptions(opts ConfigOptions) (delayMinutes, fuzz int, err error) {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return 0, 0, err
	}

	delay, fuzz := config.GetIterationDelay()
	return delay, fuzz, nil
}

// ClearGlobalIterationDelay removes the iteration delay from global config
func ClearGlobalIterationDelay() error {
	return ClearGlobalIterationDelayWithOptions(DefaultConfigOptions())
}

// ClearGlobalIterationDelayWithOptions removes the iteration delay with custom options
func ClearGlobalIterationDelayWithOptions(opts ConfigOptions) error {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return err
	}

	config.ClearIterationDelay()
	return config.SaveWithOptions(opts)
}

// SetOverloadRetryMinutes sets how long to wait before retrying after 529 overload exhaustion.
func (c *Config) SetOverloadRetryMinutes(minutes int) {
	c.OverloadRetryMinutes = minutes
}

// GetOverloadRetryMinutes returns the overload retry interval in minutes.
// Returns the default (10) if not configured or set to 0.
func (c *Config) GetOverloadRetryMinutes() int {
	if c.OverloadRetryMinutes == 0 {
		return DefaultOverloadRetryMinutes
	}
	return c.OverloadRetryMinutes
}

// GetGlobalOverloadRetryMinutes returns the overload retry setting from global config
func GetGlobalOverloadRetryMinutes() (int, error) {
	return GetGlobalOverloadRetryMinutesWithOptions(DefaultConfigOptions())
}

// GetGlobalOverloadRetryMinutesWithOptions returns the overload retry setting with custom options
func GetGlobalOverloadRetryMinutesWithOptions(opts ConfigOptions) (int, error) {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return DefaultOverloadRetryMinutes, err
	}
	return config.GetOverloadRetryMinutes(), nil
}

// UpdateGlobalOverloadRetryMinutes updates the overload retry setting in global config
func UpdateGlobalOverloadRetryMinutes(minutes int) error {
	return UpdateGlobalOverloadRetryMinutesWithOptions(DefaultConfigOptions(), minutes)
}

// UpdateGlobalOverloadRetryMinutesWithOptions updates the overload retry setting with custom options
func UpdateGlobalOverloadRetryMinutesWithOptions(opts ConfigOptions, minutes int) error {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return err
	}

	config.SetOverloadRetryMinutes(minutes)
	return config.SaveWithOptions(opts)
}

// GetGlobalVCS returns the VCS setting from global config
func GetGlobalVCS() (string, error) {
	return GetGlobalVCSWithOptions(DefaultConfigOptions())
}

// GetGlobalVCSWithOptions returns the VCS setting with custom options
func GetGlobalVCSWithOptions(opts ConfigOptions) (string, error) {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return "", err
	}
	return config.GetVCS(), nil
}

// UpdateGlobalVCS updates the VCS setting in global config
func UpdateGlobalVCS(vcs string) error {
	return UpdateGlobalVCSWithOptions(DefaultConfigOptions(), vcs)
}

// UpdateGlobalVCSWithOptions updates the VCS setting with custom options
func UpdateGlobalVCSWithOptions(opts ConfigOptions, vcs string) error {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return err
	}

	if err := config.SetVCS(vcs); err != nil {
		return err
	}
	return config.SaveWithOptions(opts)
}

// ClearGlobalVCS clears the VCS setting from global config
func ClearGlobalVCS() error {
	return ClearGlobalVCSWithOptions(DefaultConfigOptions())
}

// ClearGlobalVCSWithOptions clears the VCS setting with custom options
func ClearGlobalVCSWithOptions(opts ConfigOptions) error {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return err
	}

	config.ClearVCS()
	return config.SaveWithOptions(opts)
}

// GetProjectVCS returns the VCS setting from project config
func GetProjectVCS(projectDir string) (string, error) {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return "", err
	}
	return config.GetVCS(), nil
}

// UpdateProjectVCS updates the VCS setting in project config
func UpdateProjectVCS(projectDir, vcs string) error {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return err
	}

	if err := config.SetVCS(vcs); err != nil {
		return err
	}
	return SaveProjectConfig(projectDir, config)
}

// ClearProjectVCS clears the VCS setting from project config
func ClearProjectVCS(projectDir string) error {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return err
	}

	config.ClearVCS()
	return SaveProjectConfig(projectDir, config)
}

// SetAgentProvider sets the global agent provider preference.
// Valid values are "claude", "opencode", or "" (empty for default).
func (c *Config) SetAgentProvider(provider string) error {
	if provider != "" && provider != "claude" && provider != "opencode" {
		return fmt.Errorf("invalid agent provider: %s (must be 'claude' or 'opencode')", provider)
	}
	c.AgentProvider = provider
	return nil
}

// GetAgentProvider returns the global agent provider preference.
func (c *Config) GetAgentProvider() string {
	return c.AgentProvider
}

// ClearAgentProvider removes the agent provider preference, enabling default (claude).
func (c *Config) ClearAgentProvider() {
	c.AgentProvider = ""
}

// SetModelOverride sets a model override mapping.
func (c *Config) SetModelOverride(canonical, override string) {
	if c.ModelOverrides == nil {
		c.ModelOverrides = make(map[string]string)
	}
	c.ModelOverrides[canonical] = override
}

// GetModelOverride returns the override for a canonical model name, or empty if not set.
func (c *Config) GetModelOverride(canonical string) string {
	if c.ModelOverrides == nil {
		return ""
	}
	return c.ModelOverrides[canonical]
}

// GetModelOverrides returns all model overrides.
func (c *Config) GetModelOverrides() map[string]string {
	return c.ModelOverrides
}

// ClearModelOverrides removes all model overrides.
func (c *Config) ClearModelOverrides() {
	c.ModelOverrides = nil
}

// GetGlobalAgentProvider returns the agent provider from global config
func GetGlobalAgentProvider() (string, error) {
	return GetGlobalAgentProviderWithOptions(DefaultConfigOptions())
}

// GetGlobalAgentProviderWithOptions returns the agent provider with custom options
func GetGlobalAgentProviderWithOptions(opts ConfigOptions) (string, error) {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return "", err
	}
	return config.GetAgentProvider(), nil
}

// UpdateGlobalAgentProvider updates the agent provider in global config
func UpdateGlobalAgentProvider(provider string) error {
	return UpdateGlobalAgentProviderWithOptions(DefaultConfigOptions(), provider)
}

// UpdateGlobalAgentProviderWithOptions updates the agent provider with custom options
func UpdateGlobalAgentProviderWithOptions(opts ConfigOptions, provider string) error {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return err
	}

	if err := config.SetAgentProvider(provider); err != nil {
		return err
	}
	return config.SaveWithOptions(opts)
}

// ClearGlobalAgentProvider clears the agent provider from global config
func ClearGlobalAgentProvider() error {
	return ClearGlobalAgentProviderWithOptions(DefaultConfigOptions())
}

// ClearGlobalAgentProviderWithOptions clears the agent provider with custom options
func ClearGlobalAgentProviderWithOptions(opts ConfigOptions) error {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return err
	}

	config.ClearAgentProvider()
	return config.SaveWithOptions(opts)
}

// GetGlobalModelOverrides returns the model overrides from global config
func GetGlobalModelOverrides() (map[string]string, error) {
	return GetGlobalModelOverridesWithOptions(DefaultConfigOptions())
}

// GetGlobalModelOverridesWithOptions returns the model overrides with custom options
func GetGlobalModelOverridesWithOptions(opts ConfigOptions) (map[string]string, error) {
	config, err := LoadConfigWithOptions(opts)
	if err != nil {
		return nil, err
	}
	return config.GetModelOverrides(), nil
}

// SetAgentProvider for ProjectConfig sets the project agent provider preference.
func (c *ProjectConfig) SetAgentProvider(provider string) error {
	if provider != "" && provider != "claude" && provider != "opencode" {
		return fmt.Errorf("invalid agent provider: %s (must be 'claude' or 'opencode')", provider)
	}
	c.AgentProvider = provider
	return nil
}

// GetAgentProvider returns the project agent provider preference.
func (c *ProjectConfig) GetAgentProvider() string {
	return c.AgentProvider
}

// ClearAgentProvider removes the project agent provider preference.
func (c *ProjectConfig) ClearAgentProvider() {
	c.AgentProvider = ""
}

// SetModelOverride for ProjectConfig sets a project model override mapping.
func (c *ProjectConfig) SetModelOverride(canonical, override string) {
	if c.ModelOverrides == nil {
		c.ModelOverrides = make(map[string]string)
	}
	c.ModelOverrides[canonical] = override
}

// GetModelOverrides returns the project model overrides.
func (c *ProjectConfig) GetModelOverrides() map[string]string {
	return c.ModelOverrides
}

// GetProjectAgentProvider returns the agent provider from project config
func GetProjectAgentProvider(projectDir string) (string, error) {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return "", err
	}
	return config.GetAgentProvider(), nil
}

// UpdateProjectAgentProvider updates the agent provider in project config
func UpdateProjectAgentProvider(projectDir, provider string) error {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return err
	}

	if err := config.SetAgentProvider(provider); err != nil {
		return err
	}
	return SaveProjectConfig(projectDir, config)
}

// ClearProjectAgentProvider clears the agent provider from project config
func ClearProjectAgentProvider(projectDir string) error {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return err
	}
	config.ClearAgentProvider()
	return SaveProjectConfig(projectDir, config)
}

// GetProjectModelOverrides returns the model overrides from project config
func GetProjectModelOverrides(projectDir string) (map[string]string, error) {
	config, err := LoadProjectConfig(projectDir)
	if err != nil {
		return nil, err
	}
	return config.GetModelOverrides(), nil
}

// MergeModelOverrides merges project overrides with global overrides.
// Project overrides take precedence over global.
func MergeModelOverrides(global, project map[string]string) map[string]string {
	if global == nil && project == nil {
		return nil
	}

	result := make(map[string]string)

	// Copy global overrides
	for k, v := range global {
		result[k] = v
	}

	// Project overrides take precedence
	for k, v := range project {
		result[k] = v
	}

	return result
}
