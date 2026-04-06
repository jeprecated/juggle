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
