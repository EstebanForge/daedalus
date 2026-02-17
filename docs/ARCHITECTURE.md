# Daedalus Architecture

## System shape
Three-layer local-first architecture:
- Interface layer: CLI + TUI + plugin entry.
- Core layer: PRD service, loop manager, quality and git services.
- Adapter layer: provider modules (Codex first, Claude/Gemini ready).

## Directory layout
Global config (Linux/XDG):
- `~/.config/daedalus/config.toml`
or if `XDG_CONFIG_HOME` is set:
- `$XDG_CONFIG_HOME/daedalus/config.toml`

Project runtime state:
`.daedalus/`
- `prds/<name>/prd.md`
- `prds/<name>/prd.json`
- `prds/<name>/progress.md`
- `prds/<name>/agent.log`
- `prds/<name>/events.jsonl`
- `worktrees/<name>/` (optional)

## Components

### App controller
Responsibilities:
- Process lifecycle.
- Dependency wiring.
- Command routing.

### PRD service
Responsibilities:
- Load/save/validate PRD schema.
- Select next story.
- Persist transitions atomically.

Contract:
- `loadPRD(path) -> PRD`
- `savePRD(path, prd) -> error`
- `getNextStory(prd) -> UserStory?`
- `markInProgress(prd, storyID) -> PRD`
- `markPassed(prd, storyID) -> PRD`

### Loop manager
Responsibilities:
- Execute iteration lifecycle.
- Handle retries/backoff.
- Implement pause/stop semantics.

Default retry policy:
- `maxRetries = 3`
- `retryDelays = [0s, 5s, 15s]`
- Configurable per global config.

Loop:
1. Load active PRD.
2. Select story.
3. Set `inProgress=true`.
4. Build prompt/context.
5. Run provider iteration via adapter.
6. Run quality commands.
7. Commit changes.
8. Mark `passes=true`, `inProgress=false`.
9. Append progress and events.

### Provider adapter
Responsibilities:
- Isolate provider SDK/CLI usage.
- Stream normalized events.
- Map SDK failures to typed core errors.

Contract:
- `RunIteration(ctx, request) -> (<-chan Event, IterationResult, error)`
- `Capabilities() -> ProviderCapabilities`
- `Name() -> string`

`IterationRequest`:
- `workDir: string`
- `prompt: string`
- `contextFiles: []string`
- `approvalPolicy: string`
- `sandboxPolicy: string`
- `model: string`
- `metadata: map[string]string`

`ProviderCapabilities`:
- `streaming: bool`
- `toolCalls: bool`
- `sandboxControl: bool`
- `approvalModes: []string`
- `maxContextHint: int` (optional advisory)

Normalized events:
- `iteration_started`
- `assistant_text`
- `tool_started`
- `tool_finished`
- `command_output`
- `iteration_finished`
- `error`

Provider-specific event detail must be optional metadata only. Core logic cannot depend on provider-specific fields.

### Provider registry
Responsibilities:
- Resolve provider by key (`codex`, `claude`, `gemini`).
- Return configured provider instance or explicit configuration errors.
- Keep provider selection out of loop/business logic.

### Quality service
Responsibilities:
- Execute check commands.
- Capture outputs/exit codes.
- Return structured report.

### Git service
Responsibilities:
- Check repository cleanliness.
- Stage and commit story changes.
- Manage optional branch/worktree lifecycle.

### Event bus
Responsibilities:
- Ordered event fanout to TUI and logs.
- Buffered delivery with backpressure handling.

## State models

Story states:
- `pending -> in_progress -> passed`
- Manual reset path only via explicit user action.

Loop states:
- `ready -> running`
- `running -> paused | stopped | error | completed`
- `paused -> running`

## Failure strategy
- Retry transient adapter failures with bounded backoff.
- Never mark story passed when checks fail.
- Persist artifacts before and after each transition.
- Keep `inProgress` sticky for safe recovery.
- Retry settings are user-configurable with safe defaults.

## Security model
- Sanitize user/config/PRD input.
- Do not interpolate model text directly into shell commands.
- Require explicit capability checks for destructive git actions.
- Keep post-completion network hooks opt-in and disabled by default.

## Concurrency model
- One active loop per PRD.
- Optional parallel PRDs only through isolated worktrees.
- Shared structured logger; per-PRD artifact files.

## Provider integration notes
- Codex is v1 implementation target.
- Claude and Gemini are planned modules behind the same provider contract.
- Core packages must never import provider SDK packages directly.
- Provider modules absorb API drift and map native output/errors to normalized events.

## Testing strategy
- Unit: PRD transitions, selection logic, retry policy.
- Integration: full single-story loop in fixture repository per provider.
- Golden: native provider event to normalized event mapping.
- Contract: shared provider test suite all providers must pass.
- Smoke: CLI command and TUI startup.

## Implementation phases
- Phase 1: CLI + PRD service + logs.
- Phase 2: provider contract + registry + quality gates.
- Phase 3: Codex provider implementation + TUI runtime controls and views.
- Phase 4: worktree mode + hardening + docs finalization.
- Phase 5: Claude/Gemini provider modules.
