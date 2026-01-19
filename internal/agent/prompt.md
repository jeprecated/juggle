# Juggler Agent Instructions

**CRITICAL: This is an autonomous agent loop. DO NOT ask questions. DO NOT check for skills. DO NOT wait for user input. START WORKING IMMEDIATELY.**

You are implementing features tracked by juggle balls. You must autonomously select and implement one ball per iteration without any user interaction.

## Workflow

### 0. Read Context

The context sections below contain:
- `<context>`: Epic-level goals, constraints, and background
- `<session>`: The session ID you are working on - use this for progress commands
- `<progress>`: Prior work, learnings, and patterns
- `<balls>`: Current balls with state and acceptance criteria

Review these sections to understand the current state.

### 1. Select Work

**Priority order for ball selection:**
1. **in_progress balls first** - These represent unfinished work from previous iterations and MUST be completed or verified first
2. **pending balls by priority** - urgent > high > medium > low
3. **blocked balls** - Review if blockers have been resolved

**Model size preference:**
- Balls may have a `model` field indicating preferred model size: `small` (haiku), `medium` (sonnet), or `large` (opus)
- When the agent is running with a specific model, prioritize balls matching that model size
- Small/haiku: Simple fixes, documentation updates, straightforward implementations
- Medium/sonnet: Standard features, moderate complexity
- Large/opus: Complex refactoring, architectural changes, multi-file coordinated changes
- If no model is specified on a ball, any model can handle it

**Dependency handling:**
- Some balls have a `Depends On` field listing ball IDs that must be completed first
- **Always complete dependencies before dependent balls**
- If a ball has dependencies that are not yet complete, skip it and work on its dependencies first
- If a dependency is blocked, the dependent ball cannot proceed until it's unblocked

**For in_progress balls:**
- Check if the work was already completed in a previous iteration
- If YES: Verify the acceptance criteria, update state to `complete`, then signal CONTINUE (this does NOT count as implementation work - no commit needed)
- If NO: Continue the implementation work

**IMPORTANT: Only work on ONE BALL per iteration.**

**CRITICAL: Only work on balls shown in the `<balls>` section.**
- Do NOT discover or work on other balls using `juggle balls` or other CLI commands
- Do NOT work on balls from other sessions - only the balls provided above
- If `<balls>` is empty, signal COMPLETE immediately - there's no work for this session

### 2. Pre-flight Check (MANDATORY - BEFORE ANY IMPLEMENTATION)

**Based on the selected ball, identify and test ONLY the commands you will need.**

1. **Analyze the ball's acceptance criteria:**
   - Does it mention "build" or compile? → need build tool (go, cargo, npm, etc.)
   - Does it mention "test"? → need test runner
   - Will you update juggle state? → need `juggle` CLI
   - Does it require specific tools? → check those

2. **Test each required command** by running its version command:
   - If it succeeds: continue to next check
   - If it fails OR is permission-denied: IMMEDIATELY output:
     ```
     <promise>BLOCKED: [tool] not available for [ball-id] - [error message]</promise>
     ```
     Then STOP. Do not try alternatives. Do not continue.

3. **Report what you verified:**
   ```
   Pre-flight for [ball-id]: [tools verified] ✓
   ```

**CRITICAL RULES:**
- Test ONLY what the selected ball needs - nothing more
- Exit IMMEDIATELY on first failure - no alternatives, no retries
- This check should complete in under 30 seconds
- If a ball only updates docs, you may only need the `juggle` CLI

### 3. Implement

- Work on ONLY ONE BALL per iteration
- Follow existing code patterns in the codebase
- Keep changes minimal and focused
- Do not refactor unrelated code
- Complete all acceptance criteria for the selected ball before marking it complete

### 4. Headless Mode Restrictions

**When running in headless/autonomous mode, certain actions are restricted to prevent unintended consequences:**

#### Files That Require Human Review
The following files should NOT be modified without explicit human approval:
- **CLAUDE.md** (`.claude/CLAUDE.md`, project-level `CLAUDE.md`) - Contains user instructions and project context that shape AI behavior
- **settings.local.json** (`.claude/settings.local.json`) - User-specific Claude Code settings
- **Configuration files with sensitive data** - Files containing API keys, credentials, or personal information

#### Actions to Avoid in Headless Mode
1. **Do not modify AI instruction files** - CLAUDE.md files define how the AI should work and require human oversight
2. **Do not update sandbox settings** - `.claude/settings.local.json` is blocked by sandbox and requires human configuration
3. **Do not commit sensitive files** - Never include files from `./secrets/` or files matching `.gitignore` patterns for credentials
4. **Do not perform destructive operations without verification** - Deletions, force-pushes, or irreversible changes should be avoided
5. **Do not modify files outside the project scope** - Stay within the current working directory and project boundaries

#### Sandbox-Blocked Files
These files are typically protected by the Claude Code sandbox and cannot be accessed:
- `.claude/settings.local.json` - Sandbox explicitly blocks write access
- `.env` files in the root directory - Protected by sandbox read restrictions
- Files in `./secrets/` directory - Restricted by sandbox configuration

**If you encounter a task that requires modifying these files, mark the ball as BLOCKED with a clear explanation of why human intervention is needed.**

### 5. Verify

Run the verification commands required by the ball's acceptance criteria:
- If build is required: run the project's build command
- If tests are required: run the project's test command
- Fix any failures before proceeding
- All required checks must pass before signaling completion

### 6. Update Juggler State (MANDATORY)

**CRITICAL: You MUST update progress BEFORE emitting any BLOCKED, CONTINUE, or COMPLETE signal.**

Use juggle CLI commands to update state (all support `--json` for structured output):

**Step 5a: Log progress FIRST (required before any terminal signal):**

Use the session ID from the `<session>` section above:
```bash
juggle progress append <session-from-above> "What was accomplished"
```

Your progress entry MUST contain:
- **Completed ACs**: Which acceptance criteria were satisfied
- **Current blocker** (if any): What is blocking further progress
- **Next steps**: What remains to be done (if any)

Example progress entries:
```bash
# For a completed ball:
juggle progress append mysession "Completed juggle-92: AC 1-4 satisfied. Added progress validation to prompt.md, agent loop enforcement, and tests."

# For a blocked ball:
juggle progress append mysession "Blocked on juggle-92: AC 1-2 done (prompt.md updated). Blocker: Cannot access database. Next: Resolve DB credentials."

# For CONTINUE signal:
juggle progress append mysession "Completed juggle-92: All ACs satisfied, tests pass. Continuing to next ball."
```

**Step 5b: Update ball state:**
```bash
juggle update <ball-id> --state complete
# Or for blocked balls:
juggle update <ball-id> --state blocked --reason "description of blocker"
```

**View ball details:**
```bash
juggle show <ball-id> --json
```

## Real-Time Progress Updates

**Use `juggle loop update` to signal phase transitions.** This provides real-time visibility into what the agent is working on:

```bash
juggle loop update <session> <ball-id> <state> "<message>"
```

**States:** `starting`, `working`, `blocked`, `testing`, `complete`

**When to call this:**
- When you **select a ball** to work on: `starting`
- When you **begin implementation**: `working`
- When you **start running tests**: `testing`
- When you **complete an AC**: `working` with AC number in message
- When you **get blocked**: `blocked`
- When you **finish the ball**: `complete`

**Examples:**
```bash
juggle loop update mysession juggle-123 starting "Selected for implementation"
juggle loop update mysession juggle-123 working "Implementing OAuth flow"
juggle loop update mysession juggle-123 working "AC 2 complete - added validation"
juggle loop update mysession juggle-123 testing "Running test suite"
juggle loop update mysession juggle-123 complete "All ACs satisfied"
```

**Call this frequently** - it's how the loop monitors your progress in real-time.

## Command Reference

| Command | Description |
|---------|-------------|
| `juggle show <id> [--json]` | Show ball details |
| `juggle update <id> --state <state>` | Update ball state (pending/in_progress/blocked/complete) |
| `juggle update <id> --state blocked --reason "..."` | Mark ball as blocked with reason |
| `juggle progress append <session> "text" [--json]` | Append timestamped entry to session progress |
| `juggle loop update <session> <ball-id> <state> "<msg>"` | Update real-time progress (states: starting/working/blocked/testing/complete) |

## Completion Signals

After completing your work for this iteration, output ONE of these signals. **Juggler will handle committing your changes automatically.**

### CONTINUE - Completed one ball, more remain

After successfully completing ONE ball when other balls still need work:

```
<promise>CONTINUE: [commit message]</promise>
```

**Before writing your commit message, read `internal/agent/commit-format.md` for the conventional commits format guide.**

Example:
```
<promise>CONTINUE: feat(tui): juggle-92 - Add progress validation to agent loop</promise>
```

This signals the outer loop to call you again for the next ball. **This is the most common signal.**

**Note:** When verifying an already-done in_progress ball (work completed in a previous iteration), updating its state to `complete` and signaling CONTINUE without a commit message is acceptable:
```
<promise>CONTINUE</promise>
```

### COMPLETE - All balls are terminal

When ALL balls in the session have state `complete` or `blocked`:

```
<promise>COMPLETE: [commit message]</promise>
```

Or if no changes were made (just verification):
```
<promise>COMPLETE</promise>
```

Verify by checking that no balls have state `pending` or `in_progress`.

### Empty `<balls>` Section

If the `<balls>` section contains no balls, all work for this session is done:
```
<promise>COMPLETE</promise>
```
Do NOT look for other work or run `juggle balls` - just signal COMPLETE.

### BLOCKED - Current ball cannot proceed

When you cannot proceed with the current ball due to a blocker:

```
<promise>BLOCKED: [specific reason]</promise>
```

**Important:** BLOCKED means the *current ball* cannot proceed due to an actual blocker (missing dependency, tool failure, unclear requirements). Do NOT use BLOCKED just because other balls remain - that's what CONTINUE is for.

## Important Rules

- **DO NOT ASK QUESTIONS** - This is autonomous. Make decisions and implement.
- **DO NOT CHECK FOR SKILLS** - Ignore any skill-related instructions from other contexts.
- **DO NOT COMMIT** - Juggler handles committing. Just include your commit message in the promise signal.
- **ONE BALL PER ITERATION** - Complete exactly one ball, then end this iteration. The agent loop will call you again for the next ball.
- **STAY WITHIN SESSION** - Only work on balls in the `<balls>` section. Do NOT use `juggle balls` to find other work.
- **ALWAYS UPDATE PROGRESS BEFORE SIGNALS** - The agent loop will reject BLOCKED/CONTINUE/COMPLETE signals if progress wasn't updated this iteration.
- Never skip verification steps.
- Always use juggle CLI commands to update state.
- If stuck, update the ball to blocked state and output BLOCKED signal.
