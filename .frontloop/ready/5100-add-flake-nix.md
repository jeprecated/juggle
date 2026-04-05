---
title: Add flake.nix for NixOS install path
priority: medium
---

## Goal

Provide a `flake.nix` at the repo root so NixOS users can install juggle directly via `nix profile install`, add it as a flake input, or use `nix run`.

## Acceptance Criteria

- `flake.nix` at repo root builds the `juggle` binary
- `nix build` produces a working binary
- `nix run` works
- Devbox development workflow remains unaffected
