---
title: Support --command flag to override provider binary
priority: medium
---

## Goal

Add a `--command` flag that overrides the binary name used to invoke the provider, while keeping the provider type and its flag mappings intact. This lets users point juggle at a wrapper script or shell alias (e.g. `cc` which expands to `claude [custom flags]`) without redefining the entire provider.

## Acceptance Criteria

- [ ] Add `--command` flag to CLI (e.g. `juggle --command cc "do stuff"`)
- [ ] When set, override the binary name returned by `BinaryName()` for the selected provider
- [ ] Provider type and flag mappings remain unchanged (e.g. `--provider claude` + `--command cc` uses Claude mappings, invokes `cc`)
- [ ] Works with all providers (not just Claude)
- [ ] If the override binary isn't found on PATH, fail with a clear error
- [ ] Add tests for the override behavior
