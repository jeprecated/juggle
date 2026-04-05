---
title: Copilot CLI provider
priority: medium
---

## Goal

Add GitHub Copilot CLI (`copilot`) as a new provider, following the same pattern as the existing claude, codex, gemini, and opencode providers.

## Acceptance Criteria

- New `TypeCopilot` constant added to provider.go with `IsValid()` updated
- New `copilot.go` implementing the `Provider` interface (Type, Run, MapModel, MapPermission)
- Provider detection added to `detect.go` so `--provider copilot` works
- CLI flag/config file support for selecting the copilot provider
- Headless and interactive run modes work correctly
- Rate-limit and quota detection handles Copilot-specific error patterns
- Unit tests for the new provider

## Design Decisions

- Command is `copilot` (not `gh copilot` — that's the older suggestion tool)
- Headless prompt mode: `copilot -p "PROMPT"` with `--autopilot --yolo` for unattended execution
- Silent output: `-s` / `--silent` for captured output in headless mode
- Model selection: `--model=MODEL` (default is claude-sonnet-4-5; supports claude, gpt, gemini families)
- Permission mapping: `bypassPermissions` → `--yolo`, `acceptEdits` → `--allow-all-tools`, `plan` → no autonomous flags
- Max turns: `--max-autopilot-continues N` maps to MaxTurns
- Tool restrictions: `--allow-tool TOOL` / `--deny-tool TOOL` map to AllowedTools/DisallowedTools
- JSON output available via `--output-format json` — evaluate for structured parsing

## Implementation Notes

- Use `gemini.go` as reference implementation — similar flag structure
- Auth requires `GH_TOKEN` env var or prior `copilot login`
- Rate limit errors show "rate limit exceeded" — detect and set RateLimited/RetryAfter
- Stdin piping is limited; use `-p` flag for prompt delivery
- Working directory is inherited from cwd automatically

## Completion Summary

- Added `TypeCopilot Type = "copilot"` constant and updated `IsValid()` in provider.go
- Created `copilot.go` implementing the full `Provider` interface with headless and interactive modes
- Headless args: `--autopilot`, permission flag, `-s`, `--model=MODEL`, `--max-autopilot-continues N`, `--allow-tool`/`--deny-tool` per tool, then `-p PROMPT`
- Rate limit detection covers "rate limit", "429", "too many requests", "throttl", and shared quota patterns
- Updated `detect.go`: `BinaryName`, `Get`, and `ValidProviders` all include copilot
- Added unit tests covering all flag-building logic, MapModel, MapPermission, rate limit parsing, and detect/factory functions

### Files Changed

- internal/agent/provider/provider.go (modified — TypeCopilot constant + IsValid)
- internal/agent/provider/detect.go (modified — BinaryName, Get, ValidProviders)
- internal/agent/provider/copilot.go (new)
- internal/agent/provider/copilot_test.go (new)
- internal/agent/provider/provider_test.go (modified — ValidProviders count 5→6)
