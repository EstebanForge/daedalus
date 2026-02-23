# Daedalus Design

## Product UX intent
Operator-first. High signal. Low friction. Deterministic controls.

## Interaction model
- Primary: TUI runtime.
- Secondary: CLI commands for scripting/headless use.
- Tertiary: plugin integration entrypoints for Codex host flows.

## Screen model

### Dashboard
Shows:
- Loop state: `ready`, `running`, `paused`, `stopped`, `error`, `completed`.
- Active PRD and current story.
- Iteration counter and limits.
- Quality gate status.
- Branch/worktree context.

### Stories
Shows:
- Ordered user stories by priority.
- State badges: pending/in-progress/passed.
- Acceptance criteria counts.
- Last update timestamp.

### Logs
Shows:
- Streaming agent events.
- Tool/command outputs.
- Retry, failure, and stop reasons.
- Filter controls by event type.

### Settings
Controls:
- Quality commands.
- Retry/backoff policy.
- Auto-commit toggle.
- Worktree mode and setup command.
- Completion hooks (disabled by default).

## Core flows

### Flow A: bootstrap
1. User runs `daedalus new [name]`.
2. Tool scaffolds PRD files.
3. User authors/refines stories.
4. Tool validates to runnable JSON.

### Flow B: run loop
1. User starts run from CLI or TUI.
2. System resolves story candidate.
3. Provider iteration executes with scoped prompt/context through provider adapters.
4. Claude provider path runs via `claude -p` (CLI-only integration surface).
5. Quality commands run.
6. On success: commit + mark passed + append progress.
7. Repeat until complete or operator interruption.

### Flow C: pause/resume
1. Pause request marks graceful halt.
2. Current iteration completes.
3. Resume continues from persisted state.

### Flow D: failure/recovery
1. Transient failure triggers retry policy.
2. Terminal failure enters error state.
3. Operator can inspect logs and resume safely.

## Command surface (v1)
- `daedalus` (opens TUI)
- `daedalus run [name]`
- `daedalus plugin run [name]` (plugin/headless adapter entrypoint)
- `daedalus new [name] [context...]`
- `daedalus edit [name]` (opens `.daedalus/prds/<name>/prd.md` in local editor)
- `daedalus list`
- `daedalus status [name]`
- `daedalus validate [name]`

## Keyboard map (draft)
- `s`: start/resume
- `p`: pause
- `x`: stop
- `xx`: immediate stop (cancel active iteration)
- `t`: toggle dashboard/log view
- `n`: PRD picker
- `f`: set logs event filter
- `tail`: set logs tail length
- `,`: settings
- `?`: help
- `q`: quit

## Visual rules
- Dense layout; no decorative noise.
- Color for state only.
- Stable placement for critical status indicators.
- Warnings/errors always include next action text.

## Accessibility
- Full keyboard navigation.
- Works in low-color terminals.
- No information conveyed by color alone.

## Resolved design decisions (v1)
- Invoking `daedalus` with no subcommand opens the TUI by default.
- `daedalus run` is headless by default and does not launch TUI.
- Manual story selection is out of scope for v1. Story choice is automatic via PRD state machine.
- Auto-commit is enabled by default after quality gates pass; no confirmation prompt in headless mode.
