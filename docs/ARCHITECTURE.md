# Daedalus Architecture

## System shape
Three-layer local-first architecture:
- Interface layer: CLI + TUI + plugin entry.
- Core layer: PRD service, loop manager, quality and git services.
- Adapter layer: provider modules (Codex, Claude, Gemini, OpenCode, Copilot, Qwen Code, Pi CLI).

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

Artifact schemas and templates are defined in:
- `docs/reference/artifacts.md`

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

Prompt/context construction (v1):
- Prompt is deterministic and built from:
- project name and PRD description
- active story ID, title, description, acceptance criteria, and priority
- explicit completion rule: implement only the active story and satisfy all acceptance criteria
- explicit safety rule: do not execute destructive git operations
- explicit output rule: summarize changes and test/check results
- `contextFiles` are selected in this order:
- required: `.daedalus/prds/<name>/prd.md`, `.daedalus/prds/<name>/prd.json`, `.daedalus/prds/<name>/progress.md`
- optional: repository-local guidance files (for example `AGENTS.md`, `README.md`) when present
- `contextFiles` paths must be unique, repository-relative when possible, and stable in ordering.
- Provider modules consume `prompt` and `contextFiles` as-is and must not silently rewrite story scope.

### Provider adapter
Responsibilities:
- Isolate provider SDK/CLI usage.
- Stream normalized events.
- Map provider-native failures to typed core errors.

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
In v1, started iterations stream normalized events live while the provider process is running. Pre-start failures are returned via `error`; in-flight failures are emitted through `error` events.

### Provider registry
Responsibilities:
- Resolve provider by key (`codex`, `claude`, `gemini`, `opencode`, `copilot`, `qwen`, `pi`).
- Return configured provider instance or explicit configuration errors.
- Keep provider selection out of loop/business logic.

### Quality service
Responsibilities:
- Execute check commands.
- Capture outputs/exit codes.
- Return structured report.

Contract:
- `RunChecks(ctx, workDir, commands) -> QualityReport`

`QualityReport`:
- `passed: bool`
- `results: []CheckResult`

`CheckResult`:
- `command: string`
- `exitCode: int`
- `stdout: string`
- `stderr: string`
- `duration: string`

Rules:
- All configured commands run in declared order.
- Any non-zero exit code sets `QualityReport.passed=false`.
- Loop must not mark story passed or commit when `QualityReport.passed=false`.
- Quality outputs must be persisted to `events.jsonl` and `progress.md`.

### Git service
Responsibilities:
- Check repository cleanliness.
- Stage and commit story changes.
- Manage optional branch/worktree lifecycle.

Contract:
- `CommitStory(ctx, workDir, storyID, storyTitle) -> CommitResult`

`CommitResult`:
- `committed: bool`
- `commitSHA: string` (empty when `committed=false`)
- `message: string`

Rules:
- Commit is allowed only after quality gates pass.
- Commit message format (v1): `feat(US-XXX): Story Title`.
- Commit failures must keep story as `in_progress` and move loop to `error`.
- When no changes exist, runtime may skip commit but must record this in `progress.md`.

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
- On terminal failure, loop enters `error` and current story remains `in_progress` until explicit operator reset.

## Security model
- Sanitize user/config/PRD input.
- Do not interpolate model text directly into shell commands.
- Require explicit capability checks for destructive git actions.
- Keep post-completion network hooks opt-in and disabled by default.

## Concurrency model
- One active loop per PRD.
- Optional parallel PRDs only through isolated worktrees.
- Shared structured logger; per-PRD artifact files.

Worktree lifecycle and safety rules are specified in:
- `docs/reference/worktrees.md`

## Provider integration notes
- All seven providers use CLI-based execution with the `-p` / `--print` flag pattern for non-interactive mode.
- Codex is v1 implementation target.
- Claude integration is CLI-only through the `claude` binary; no Claude SDK dependency in Daedalus core or adapter contracts.
- Claude OAuth is not a dependency for Daedalus integration; authentication is delegated to the local Claude CLI session/token setup.
- Gemini CLI uses `-p/--prompt` for non-interactive mode. **Requires API key** (not OAuth) to comply with Google ToS.
- OpenCode CLI uses `-p` or direct prompt for non-interactive mode.
- Copilot CLI uses `-p/--prompt` for non-interactive mode.
- Qwen Code uses `-p` for headless mode. Supports Qwen OAuth (free tier) or API keys. **API key required for CI/automation.**
- Pi CLI uses `-p` for non-interactive mode. Supports multiple providers via `--provider` flag.
- Core packages must never import provider SDK packages directly.
- Provider modules absorb API drift and map native output/errors to normalized events.

## Testing strategy
- Unit: PRD transitions, selection logic, retry policy.
- Integration: full single-story loop in fixture repository per provider.
- Golden: native provider event to normalized event mapping.
- Contract: shared provider test suite all providers must pass.
- Smoke: CLI command and TUI startup.

Minimum required unit coverage before M1 handoff:
- `internal/prd`: load/save/validate/select/transition behavior
- `internal/config`: load, defaults, fallback resolution, and validation rules
- `internal/loop`: retry/backoff and terminal failure handling
- `internal/providers`: registry resolution and provider error mapping

## Implementation phases
- Phase 1: CLI + PRD service + logs.
- Phase 2: provider contract + registry + quality gates.
- Phase 3: All seven provider implementations (Codex, Claude, Gemini, OpenCode, Copilot, Qwen Code, Pi) + TUI runtime controls and views.
- Phase 4: worktree mode + hardening + docs finalization.
- Phase 5: Multi-provider hardening + provider-specific optimizations.
