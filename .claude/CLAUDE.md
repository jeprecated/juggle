# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test Commands

### Building

```bash
devbox shell
go build -o juggle ./cmd/juggle
```

### Testing

```bash
# All tests (quiet)
devbox run test-quiet

# All tests (verbose)
go test -v ./...

# Integration tests only
go test -v ./internal/integration_test/...

# Single test
go test -v ./internal/cli/... -run TestRunLoop
```

### Development

```bash
go mod tidy
go fmt ./...
go vet ./...

# Lint (must pass before pushing)
devbox run -- golangci-lint run

# Race detector (must pass before pushing)
devbox run -- go test -race ./...
```

## Architecture

**Juggle** is a minimal agent loop runner. It takes prompt content as positional arguments, runs an AI agent in a loop, and stops. No task storage, no project footprint, no opinions about what's in the prompt.

### Package Structure

- `cmd/juggle/main.go` — entry point (version set via ldflags)
- `internal/cli/juggle.go` — CLI flags, Config, Run(), RunLoop(), rate limiting, prompt building
- `internal/cli/resolve.go` — @file resolution for positional args
- `internal/cli/watch.go` — single-directory watch mode: ScanWatchDir(), RunWatch(), runWatchTask()
- `internal/cli/watch_glob.go` — glob-pattern watch mode with parallel workers
- `internal/cli/dashboard.go` — TUI dashboard for watch mode worker overview
- `internal/cli/hooks.go` — shell command lifecycle hooks (--cmd-before, --cmd-after, --stop-when)
- `internal/cli/session_hooks.go` — agent-internal hooks (Claude-specific EVENT:CMD)
- `internal/cli/phase_agent.go` — multi-phase agent sessions (--agent-pre/before/after/post)
- `internal/cli/log.go` — JSONL logging (per-iteration + summary entries)
- `internal/cli/config.go` — juggle.toml config file loading
- `internal/cli/help.go` — grouped --help output with flag categories
- `internal/cli/color.go` — ANSI color helpers (respects NO_COLOR)
- `internal/cli/format.go` — iteration header/status formatting
- `internal/cli/complete.go` — shell completion subcommand (bash, zsh, fish)
- `internal/cli/nushell.go` — Nushell completion generation
- `internal/cli/powershell.go` — PowerShell completion generation
- `internal/cli/juggle_env.go` — environment variable setup for spawned agents
- `internal/agent/runner.go` — Runner interface, ProviderRunner, MockRunner
- `internal/agent/provider/` — provider implementations: claude, opencode, codex, gemini, custom

### Key patterns

- **Config struct with dependency injection** — `Config.Runner` field allows injecting MockRunner for tests
- **No global state** — all configuration flows through Config
- **Provider abstraction** — `provider.Provider` interface wraps AI coding CLIs (Claude Code, OpenCode, Codex, Gemini CLI, or any custom command)
