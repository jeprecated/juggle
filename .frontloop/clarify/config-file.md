---
title: Config file (juggle.toml)
priority: low
---

## Goal

Support an optional `juggle.toml` in the project root (and `~/.config/juggle/config.toml` for global defaults) for persistent flag defaults. CLI flags override file values. Must be verbose about when a config file is being applied.

## Acceptance Criteria

- Reads `juggle.toml` from cwd if present
- Falls back to `~/.config/juggle/config.toml` for global defaults
- CLI flags override config file values
- When a config file is loaded, print to stderr: "using config: ./juggle.toml"
- When verbose, list which values came from config vs flags
- Supports all flag equivalents: iterations, delay, model, provider, trust, etc.
- `--no-config` flag to skip config file loading
- No config file required — bare `juggle` works exactly as today
- Tests verify override precedence and verbose output

## Design Decisions

- TOML format (matches Go ecosystem, simple to parse)
- Config is purely for flag defaults, not for phase/pipeline definitions (those come later if needed)
