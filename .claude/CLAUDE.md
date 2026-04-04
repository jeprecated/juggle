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
```

## Architecture

**Juggle** is a minimal agent loop runner. It takes prompt content as positional arguments, runs an AI agent in a loop, and stops. No task storage, no project footprint, no opinions about what's in the prompt.

### Package Structure

- `cmd/juggle/main.go` — entry point
- `internal/cli/juggle.go` — CLI flags, Config, Run(), RunLoop(), rate limiting, prompt building
- `internal/cli/resolve.go` — @file resolution for positional args
- `internal/cli/watch.go` — watch mode: ScanWatchDir(), RunWatch(), runWatchTask()
- `internal/agent/runner.go` — Runner interface, ProviderRunner, MockRunner
- `internal/agent/provider/` — claude and opencode provider implementations

### Key patterns

- **Config struct with dependency injection** — `Config.Runner` field allows injecting MockRunner for tests
- **No global state** — all configuration flows through Config
- **Provider abstraction** — `provider.Provider` interface wraps Claude Code and OpenCode CLIs
