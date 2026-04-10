---
title: Per-node provider dispatch in pipeline executor
priority: high
---

## Goal

Allow pipeline nodes to use different providers (claude, codex, gemini, opencode, custom). Currently the executor takes a single `cfg.Runner` and all agent nodes use it. The design doc and parser both support per-node `--provider` but the executor ignores it.

## Acceptance Criteria

- Each agent node's `Provider` field is used to select the correct runner at execution time
- If a node omits `Provider`, it falls back to pipeline `Defaults.Provider`, then to the CLI-level default
- Different nodes in the same pipeline can use different providers (e.g., claude for setup, codex for implementation)
- Tests cover: mixed providers, fallback to defaults, unknown provider error

## Implementation Notes

- The executor needs a way to resolve providers at runtime, not just hold a single Runner
- Options: (a) accept a `map[string]agent.Runner` keyed by provider name, or (b) accept a `RunnerFactory func(provider string) (agent.Runner, error)`
- Option (b) is more flexible and avoids pre-constructing runners that may not be needed
- The CLI wiring task should construct this factory from the same provider setup logic used today
