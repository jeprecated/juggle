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

## Completion Summary

- Added `flake.nix` using `buildGoModule` with `vendorHash = null` (vendored deps)
- Added `vendor/` directory via `go mod vendor` (cobra, toml, doublestar, mousetrap, pflag)
- Generated `flake.lock` pinning nixpkgs and flake-utils inputs
- Verified `nix build` produces working binary
- Verified `nix run . -- --version` works
- Verified all 711 devbox tests still pass

### Files Changed

- `flake.nix` (new)
- `flake.lock` (new)
- `vendor/` (new)
