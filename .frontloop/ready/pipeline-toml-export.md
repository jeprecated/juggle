---
title: Export pipeline to TOML file format
priority: low
---

## Goal

Implement serialization of a canonical Pipeline back to TOML. The design doc's round-trip goal requires that pipelines can be saved, not just loaded. This enables:

- `juggle pipeline ... --save pipeline.toml` to persist an inline pipeline
- Future GUI export
- Normalization workflows (load, normalize, save)

## Acceptance Criteria

- `SaveFile(path string, p *Pipeline) error` writes a valid TOML file
- `SaveBytes(p *Pipeline) ([]byte, error)` returns TOML bytes
- Round-trip: `LoadBytes(SaveBytes(p))` produces an equivalent pipeline
- Output is clean, human-readable TOML (not a raw marshal dump)
- Defaults section is included when non-empty
- Tests cover: round-trip fidelity, empty defaults omitted, all node types and flags

## Implementation Notes

- Use `github.com/BurntSushi/toml` encoder or `github.com/pelletier/go-toml` for cleaner output
- Convert Duration fields back to string format for TOML
- The V1 TOML ordering limitation (agents before cmds) applies to export too
- A `--save` flag on the pipeline subcommand is optional for this task; the core is the serialization functions
