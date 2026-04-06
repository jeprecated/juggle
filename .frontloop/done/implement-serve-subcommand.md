---
title: Implement juggle serve subcommand (HTTP API + watch)
priority: high
---

## Goal

Add `juggle serve [folder] [prompts...] --port 8080` subcommand that starts an HTTP API endpoint and watches the same folder. API callers POST a prompt, juggle writes it to a file in the folder (with date+id naming), and the built-in watch picks it up and runs it as an agent session.

## Acceptance Criteria

- `juggle serve ./tasks/ "base prompt" -n 5 --port 8080` starts HTTP server + watch on `./tasks/`
- `POST /prompt.txt`, `POST /prompt.md`, `POST /prompt.json` write files with matching extensions
- File naming: `YYYYMMDD-HHMMSS-<short-id>.<ext>`
- Returns 202 Accepted (empty acknowledgment)
- Watch mode picks up the new file and runs it through the agent lifecycle
- All shared flags inherited from root (iterations, provider, model, hooks, etc.)
- Serve-specific flags: `--port` (default 8080), `--bind` (default 127.0.0.1)
- Watch-specific flags available too: `--workers`, `--dashboard`
- Graceful shutdown stops both HTTP server and watch
- `juggle serve --help` shows all applicable flags

## Design Decisions

- File extension determined by URL path: POST /prompt.txt, /prompt.md, /prompt.json
- Response is a bare 202 Accepted — no body, no filename returned
- Serve runs RunWatch() in a goroutine, HTTP server in main goroutine. All watch machinery reused as-is.
- Localhost-only, no auth.

## Dependencies

- Requires 0001 (watch subcommand refactor) to be completed first. Serve reuses the watch subcommand's flag registration and RunWatch machinery.

## Implementation Notes

- New file: `internal/cli/serve.go` — cobra subcommand, HTTP handler, file writing
- HTTP handler: parse extension from URL path, generate filename with timestamp + short UUID, write request body to file in watch dir, return 202
- Subcommand args mirror watch: `args[0]` = folder, `args[1:]` = prompt content
- Internally builds same Config as watch subcommand, adds HTTP server alongside
- Signal handling: context cancellation stops both HTTP server (Shutdown) and RunWatch (cfg.Shutdown channel)
- No new dependencies beyond stdlib `net/http`

## Completion Summary

- Added `internal/cli/serve.go`: `juggle serve` cobra subcommand with HTTP handler, file writing, and signal handling
- `generateServeFilename(t, id, ext)`: produces `YYYYMMDD-HHMMSS-<id>.<ext>` timestamp filenames
- `newServeHandler(watchDir)`: HTTP handler accepting POST to `/prompt.txt`, `/prompt.md`, `/prompt.json`; returns 202 Accepted with empty body; unsupported paths return 404; non-POST returns 405
- `RunServe(cfg, addr)`: starts RunWatch in goroutine, HTTP server in main goroutine; graceful shutdown via cfg.Shutdown channel
- `runServeCmd`: builds Config from flags and calls RunServe; signal handling mirrors root command
- Serve flags: `--port` (8080), `--bind` (127.0.0.1), `--workers`, `--dashboard`; all shared flags inherited via PersistentFlags
- Added `internal/cli/serve_test.go`: 10 unit tests covering filename format and all HTTP handler behaviors
- Fixed pre-existing 0001 incomplete work: updated `config.go` and `config_test.go` references from `flags.watch`/`flags.workers` to `watchFlags.dirs`/`watchFlags.workers`

### Files Changed

- `internal/cli/serve.go` (new — partial skeleton existed, extended with RunServe, serveCmd, runServeCmd)
- `internal/cli/serve_test.go` (new)
- `internal/cli/config.go` (modified — flags.watch→watchFlags.dirs, flags.workers→watchFlags.workers)
- `internal/cli/config_test.go` (modified — same flag reference updates)
