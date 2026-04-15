# Journal

## 2026-04-15 — Add `juggle loop` subcommand

Added `juggle loop [prompts] [flags]` as a new cobra subcommand alongside the existing `juggle` bare invocation. This is step one of the ADR-001 consolidation.

### What was already there
The `loopCmd` definition, `registerSharedFlags` helper, and `init()` wiring were already implemented in `internal/cli/juggle.go`. The loop subcommand was fully functional.

### What was added / fixed
- **`internal/cli/loop_test.go`** — 12 new TDD tests covering: subcommand existence, `Use` field, all loop-specific flags (`--iterations`/`-n`, `--delay`, `--resume`, `--continue`), all shared flags, hidden flags (`--fuzz`, `--interactive`, `--show-thinking`, `--provider`), help group headings, runner invocation, `-X` shorthand, and root cmd backward compatibility.
- **`internal/cli/juggle.go`** — Fixed `-X` shorthand for `--extra` in `registerSharedFlags`: changed `StringArrayVar` + manual `.Shorthand = "X"` to `StringArrayVarP`, so pflag's internal shorthands map is correctly populated and `ShorthandLookup("X")` works.

All 5 packages pass (47s for cli due to integration-style tests).
