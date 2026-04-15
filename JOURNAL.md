# Journal

## 2026-04-15 — Remove `juggle watch` and `juggle serve` subcommands

Removed both legacy subcommands now that `juggle queue` replaces them. TDD approach: wrote `TestWatchSubcommandDoesNotExist` and `TestServeSubcommandDoesNotExist` first (Red), then deleted the cobra registrations and handlers (Green), then updated stale tests (Refactor).

### Removed
- `watchCmd`, `watchFlags` struct, `runWatchSubcmd` from `juggle.go`
- `serveCmd`, `serveSpecificFlags`, `runServeCmd`, `RunServe` from `serve.go` (queue manages its own HTTP goroutine inline)
- `detectShellGlobExpansion` from `watch_glob.go` (only applied to the old positional-arg watch pattern; `queue --watch` uses a flag so shell expansion is moot)
- `TestDetectShellGlobExpansion`, `TestWatchCmdHelpFlagGroupHeadings`, `TestServeHelpShowsServeGroup`, `TestServeHelpShowsWatchModeGroup` — tests for removed code

### Updated
- `help_test.go`: `TestRootHelpListsWatchAndServeSubcommands` → `TestRootHelpListsLoopAndQueueSubcommands`; asserts loop/queue are present and watch/serve are absent
- `nushell_test.go`: completion test now checks for `juggle loop` / `juggle queue` externs and asserts watch/serve are not generated

## 2026-04-15 — Add `juggle loop` subcommand

Added `juggle loop [prompts] [flags]` as a new cobra subcommand alongside the existing `juggle` bare invocation. This is step one of the ADR-001 consolidation.

### What was already there
The `loopCmd` definition, `registerSharedFlags` helper, and `init()` wiring were already implemented in `internal/cli/juggle.go`. The loop subcommand was fully functional.

### What was added / fixed
- **`internal/cli/loop_test.go`** — 12 new TDD tests covering: subcommand existence, `Use` field, all loop-specific flags (`--iterations`/`-n`, `--delay`, `--resume`, `--continue`), all shared flags, hidden flags (`--fuzz`, `--interactive`, `--show-thinking`, `--provider`), help group headings, runner invocation, `-X` shorthand, and root cmd backward compatibility.
- **`internal/cli/juggle.go`** — Fixed `-X` shorthand for `--extra` in `registerSharedFlags`: changed `StringArrayVar` + manual `.Shorthand = "X"` to `StringArrayVarP`, so pflag's internal shorthands map is correctly populated and `ShorthandLookup("X")` works.

All 5 packages pass (47s for cli due to integration-style tests).
