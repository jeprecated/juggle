package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// tomlStringOrList decodes a TOML value that is either a bare string or a list
// of strings. Storing watch = "./tasks" and watch = ["./a", "./b"] are both valid.
type tomlStringOrList []string

func (t *tomlStringOrList) UnmarshalTOML(v interface{}) error {
	switch x := v.(type) {
	case string:
		*t = []string{x}
		return nil
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("watch: expected string element, got %T", item)
			}
			out = append(out, s)
		}
		*t = out
		return nil
	default:
		return fmt.Errorf("watch: expected string or list of strings, got %T", v)
	}
}

// FileConfig holds all config values that can be set in juggle.toml.
// All fields are pointers: nil means the key was absent from the config file.
type FileConfig struct {
	Watch        *tomlStringOrList `toml:"watch"`
	Iterations   *int              `toml:"iterations"`
	Model        *string  `toml:"model"`
	Provider     *string  `toml:"provider"`
	Delay        *int     `toml:"delay"`
	Fuzz         *int     `toml:"fuzz"`
	Trust        *bool    `toml:"trust"`
	Plan         *bool    `toml:"plan"`
	Timeout      *string  `toml:"timeout"`
	MaxWait      *string  `toml:"max_wait"`
	Verbose      *bool    `toml:"verbose"`
	MaxFailures  *int     `toml:"max_failures"`
	CmdBefore    *string  `toml:"cmd_before"`
	CmdAfter     *string  `toml:"cmd_after"`
	StopWhen     *string  `toml:"stop_when"`
	Log          *string  `toml:"log"`
	MaxCost      *float64 `toml:"max_cost"`
	Label        *string  `toml:"label"`
	MaxTurns     *int     `toml:"max_turns"`
	MCPConfig    *string  `toml:"mcp_config"`
	OnFailure    *string  `toml:"on_failure"`
	Retries      *int     `toml:"retries"`
	RetryPrompt  *string  `toml:"retry_prompt"`
	SystemPrompt        *string `toml:"system_prompt"`
	Workers      *int     `toml:"workers"`
	Every        *string  `toml:"every"`
	Now            *bool    `toml:"now"`
	Serve          *string  `toml:"serve"`
	OnTouch        *bool    `toml:"on_touch"`
	Dashboard      *bool    `toml:"dashboard"`
	WorkDir        *string  `toml:"workdir"`
	Channels     *string  `toml:"channels"`
	ExtraArgs    []string `toml:"extra_args"`
	NoLog        *bool    `toml:"no_log"`
	ID           *string  `toml:"id"`
}

// LoadConfig looks for a config file in cwd/juggle.toml, then ~/.config/juggle/config.toml.
// Prints "using config: <path>" to stderr when a file is found.
// Returns (nil, "", nil) when noConfig=true or no file is found.
func LoadConfig(noConfig bool, cwd string, stderr io.Writer) (*FileConfig, string, error) {
	if noConfig {
		return nil, "", nil
	}

	local := filepath.Join(cwd, "juggle.toml")
	if _, err := os.Stat(local); err == nil {
		cfg, err := parseConfigFile(local)
		if err != nil {
			return nil, "", err
		}
		fmt.Fprintf(stderr, "using config: ./%s\n", filepath.Base(local))
		return cfg, local, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", nil
	}
	global := filepath.Join(home, ".config", "juggle", "config.toml")
	if _, err := os.Stat(global); err == nil {
		cfg, err := parseConfigFile(global)
		if err != nil {
			return nil, "", err
		}
		fmt.Fprintf(stderr, "using config: %s\n", global)
		return cfg, global, nil
	}

	return nil, "", nil
}

func parseConfigFile(path string) (*FileConfig, error) {
	var cfg FileConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return &cfg, nil
}

// ApplyFileConfig applies non-nil config values to the package-level flags struct,
// but only for flags not explicitly set on the CLI (changed returns false for those).
// The mode parameter ("loop" or "queue") controls which keys are applied:
// loop-only keys (iterations, delay) are skipped when mode is "queue", and
// queue-only keys (watch, every, now, serve, on-touch, dashboard, workers) are
// skipped when mode is "loop". Shared keys apply to both modes.
// When verbose=true, prints which values were applied from config to stderr.
func ApplyFileConfig(cfg *FileConfig, changed func(string) bool, verbose bool, stderr io.Writer, mode string) {
	if cfg == nil {
		return
	}

	isLoop := mode == "loop"
	isQueue := mode == "queue"

	var applied []string

	set := func(flagName string, fn func()) {
		if !changed(flagName) {
			fn()
			if verbose {
				applied = append(applied, flagName)
			}
		}
	}

	// Queue-only keys
	if isQueue {
		if cfg.Watch != nil {
			set("watch", func() { queueFlags.watch = []string(*cfg.Watch) })
		}
		if cfg.Workers != nil {
			set("workers", func() { queueFlags.workers = *cfg.Workers })
		}
		if cfg.Every != nil {
			set("every", func() {
				d, err := time.ParseDuration(*cfg.Every)
				if err == nil {
					queueFlags.every = d
				}
			})
		}
		if cfg.Now != nil {
			set("now", func() { queueFlags.now = *cfg.Now })
		}
		if cfg.Serve != nil {
			set("serve", func() { queueFlags.serve = *cfg.Serve })
		}
		if cfg.OnTouch != nil {
			set("on-touch", func() { queueFlags.onTouch = *cfg.OnTouch })
		}
		if cfg.Dashboard != nil {
			set("dashboard", func() { queueFlags.dashboard = *cfg.Dashboard })
		}
	}

	// Loop-only keys
	if isLoop {
		if cfg.Iterations != nil {
			set("iterations", func() { flags.iterations = *cfg.Iterations })
		}
		if cfg.Delay != nil {
			set("delay", func() { flags.delay = *cfg.Delay })
		}
		if cfg.Fuzz != nil {
			set("fuzz", func() { flags.fuzz = *cfg.Fuzz })
		}
	}

	// Shared keys
	if cfg.Model != nil {
		set("model", func() { flags.model = *cfg.Model })
	}
	if cfg.Provider != nil {
		set("provider", func() { flags.provider = *cfg.Provider })
	}
	if cfg.Trust != nil {
		set("trust", func() { flags.trust = *cfg.Trust })
	}
	if cfg.Plan != nil {
		set("plan", func() { flags.plan = *cfg.Plan })
	}
	if cfg.Timeout != nil {
		set("timeout", func() {
			if d, err := time.ParseDuration(*cfg.Timeout); err == nil {
				flags.timeout = d
			}
		})
	}
	if cfg.MaxWait != nil {
		set("max-wait", func() {
			if d, err := time.ParseDuration(*cfg.MaxWait); err == nil {
				flags.maxWait = d
			}
		})
	}
	if cfg.Verbose != nil {
		set("verbose", func() { flags.verbose = *cfg.Verbose })
	}
	if cfg.MaxFailures != nil {
		set("max-failures", func() { flags.maxFailures = *cfg.MaxFailures })
	}
	if cfg.CmdBefore != nil {
		set("cmd-before", func() { flags.cmdBefore = *cfg.CmdBefore })
	}
	if cfg.CmdAfter != nil {
		set("cmd-after", func() { flags.cmdAfter = *cfg.CmdAfter })
	}
	if cfg.StopWhen != nil {
		set("stop-when", func() { flags.stopWhen = *cfg.StopWhen })
	}
	if cfg.Log != nil {
		set("log", func() { flags.log = *cfg.Log })
	}
	if cfg.MaxCost != nil {
		set("max-cost", func() { flags.maxCost = *cfg.MaxCost })
	}
	if cfg.Label != nil {
		set("label", func() { flags.label = *cfg.Label })
	}
	if cfg.MaxTurns != nil {
		set("max-turns", func() { flags.maxTurns = *cfg.MaxTurns })
	}
	if cfg.MCPConfig != nil {
		set("mcp-config", func() { flags.mcpConfig = *cfg.MCPConfig })
	}
	if cfg.OnFailure != nil {
		set("on-failure", func() { flags.onFailure = *cfg.OnFailure })
	}
	if cfg.Retries != nil {
		set("retries", func() { flags.retries = *cfg.Retries })
	}
	if cfg.RetryPrompt != nil {
		set("retry-prompt", func() { flags.retryPrompt = *cfg.RetryPrompt })
	}
	if cfg.SystemPrompt != nil {
		set("system-prompt", func() { flags.systemPrompt = *cfg.SystemPrompt })
	}
	if cfg.WorkDir != nil {
		set("workdir", func() { flags.workdir = *cfg.WorkDir })
	}
	if cfg.Channels != nil {
		set("channels", func() { flags.channels = *cfg.Channels })
	}
	if cfg.ExtraArgs != nil {
		set("extra", func() { flags.extraArgs = cfg.ExtraArgs })
	}
	if cfg.NoLog != nil {
		set("no-log", func() { flags.noLog = *cfg.NoLog })
	}
	if cfg.ID != nil {
		set("id", func() { flags.id = *cfg.ID })
	}

	if verbose && len(applied) > 0 {
		for _, name := range applied {
			fmt.Fprintf(stderr, "  config: %s\n", name)
		}
	}
}
