# Daedalus Architecture & Design

## System shape
Three-layer local-first architecture:
- Interface layer: CLI + TUI + plugin entry.
- Core layer: onboarding manager, PRD service, loop manager, quality and git services.
- Adapter layer: provider modules (Codex, Claude, Gemini, OpenCode, Copilot, Qwen Code, Pi CLI).

## Directory layout
Global config (Linux/XDG):
- `~/.config/daedalus/config.toml`
- `$XDG_CONFIG_HOME/daedalus/config.toml` when `XDG_CONFIG_HOME` is set.

Project runtime state:
- `.daedalus/`
- `onboarding/state.json` (onboarding progress + completion marker)
- `prds/<name>/prd.md`
- `prds/<name>/prd.json`
- `prds/<name>/progress.md`
- `prds/<name>/agent.log`
- `prds/<name>/events.jsonl`
- `prds/<name>/project-summary.md` (existing-project scan output)
- `prds/<name>/jtbd.md` (captured/reviewed JTBD)
- `prds/<name>/architecture-design.md` (scan-seeded architecture context)
- `worktrees/<name>/` (optional)

Artifact schemas and templates are defined in:
- `docs/reference/artifacts.md`

## Components

### App controller
Responsibilities:
- Process lifecycle.
- Dependency wiring.
- Command routing.
- Enforce startup ordering: onboarding first when required.

### Onboarding manager
Responsibilities:
- Detect whether onboarding is required.
- Resume onboarding at first incomplete step.
- Differentiate empty folder mode vs existing project mode.
- Run project discovery scan for existing projects.
- Seed planning context artifacts for JTBD/PRD/Architecture docs.

Startup contract:
1. Check `.daedalus/` existence and `onboarding/state.json` completion marker.
2. If onboarding complete, continue to normal runtime.
3. If onboarding required, execute screen flow:
- Git ignore decision.
- Existing project discovery (if directory has any file/folder other than `.daedalus/`).
- JTBD capture/review.
- Initial PRD creation.
4. Persist step completion atomically after each successful screen.

Existing project scan contract:
- Trigger: any file/folder besides `.daedalus/` in current working directory.
- Execution: agent-driven scan via Agents CLI prompts.
- Mode: read-only.
- UX: background execution with loader/spinner and status text.
- Failure: actionable error + retry without discarding previous onboarding answers.

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
- `contextFiles` selection order:
- required: `.daedalus/prds/<name>/prd.md`, `.daedalus/prds/<name>/prd.json`, `.daedalus/prds/<name>/progress.md`
- optional: `.daedalus/prds/<name>/project-summary.md`, `.daedalus/prds/<name>/jtbd.md`, `.daedalus/prds/<name>/architecture-design.md`
- optional: repository-local guidance files (`AGENTS.md`, `README.md`) when present
- `contextFiles` must be unique and stable in ordering.

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

### Document templates
Responsibilities:
- Provide a canonical, provider-agnostic structure for all planning artifacts.
- Ensure deterministic document output regardless of which LLM backend is used.

Sources: `internal/templates/` (embedded via `//go:embed`).

Templates:
- `project-summary.md` — repository scan output (LLM fills in placeholders).
- `jtbd.md` — Jobs-to-be-Done (human-authored, LLM-drafted in existing-project mode).
- `architecture-design.md` — architecture context (human-authored, scan-seeded).
- `prd.md` — PRD narrative document (created with every new PRD).

Rules:
- All code that generates or seeds these documents must use `internal/templates`.
- Headings in templates are the single source of truth; changing them requires
  updating the template file and its tests.

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

Rules:
- All configured commands run in declared order.
- Any non-zero exit code sets `QualityReport.passed=false`.
- Loop must not mark story passed or commit when checks fail.
- Quality outputs persist to `events.jsonl` and `progress.md`.

### Git service
Responsibilities:
- Check repository cleanliness.
- Stage and commit story changes.
- Manage optional branch/worktree lifecycle.

Contract:
- `CommitStory(ctx, workDir, storyID, storyTitle) -> CommitResult`

Rules:
- Commit is allowed only after quality gates pass.
- Commit message format: `feat(US-XXX): Story Title`.
- Commit failures keep story `in_progress` and move loop to `error`.
- No changes may skip commit but must be recorded in `progress.md`.

## UX design

### Product UX intent
- Operator-first.
- High signal, low friction.
- Deterministic controls.

### Onboarding screens
1. Git ignore decision with pros/cons/use-cases.
2. Existing-project discovery (conditional) with background scan progress UI.
3. JTBD capture/review.
4. First PRD creation with path preview.

### Runtime screen model
- Dashboard: loop state, provider, story, quality status, worktree context.
- Stories: ordered stories with state badges and criteria count.
- Logs: streaming events with filtering.
- Settings: effective provider/retry/worktree/theme/quality config.

### Core interaction flows
- Onboarding-first bootstrap.
- Run loop lifecycle.
- Pause/resume.
- Failure/recovery.

### Visual and accessibility rules
- Dense layout; no decorative noise.
- Color indicates state only.
- Stable location for critical status.
- Warnings/errors include next action text.
- Full keyboard navigation.
- No information conveyed by color alone.

## State models

Onboarding states:
- `not_started -> in_progress -> completed`
- `in_progress` tracks per-step completion markers.
- Startup resumes from first incomplete step.

Project mode states:
- `empty_folder`
- `existing_project`

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
- On onboarding scan failure, keep onboarding state and allow retry.

## Security model
- Sanitize user/config/PRD input.
- Do not interpolate model text directly into shell commands.
- Require explicit capability checks for destructive git actions.
- Project discovery scan must be read-only.
- Post-completion network hooks remain opt-in and disabled by default.

## Concurrency model
- One active loop per PRD.
- One active onboarding session per repository.
- Optional parallel PRDs only through isolated worktrees.
- Shared structured logger; per-PRD artifact files.

Worktree lifecycle and safety rules are specified in:
- `docs/reference/worktrees.md`

## Implementation phases
- [x] Phase 1: CLI + PRD service + logs.
- [x] Phase 2: onboarding manager + existing-project discovery scan + context seeding.
- [x] Phase 3: provider contract + registry + quality gates + TUI runtime controls/views.
- [x] Phase 4: worktree mode + TUI polish/hardening (richer event streaming, stronger pause/stop lifecycle controls, visual/interaction refinement) + docs finalization.
- [x] Phase 5: remaining provider implementations (including Gemini), multi-provider hardening, and provider-specific optimizations.

## Provider integration notes
- All providers use CLI-based non-interactive execution (`-p` / `--prompt` patterns).
- All 7 providers are fully implemented: codex, claude, gemini, opencode, copilot, qwen, pi.
- Codex and Claude support sandbox and approval policy configuration.
- Gemini requires API key authentication (not OAuth).
- Copilot uses `--prompt` flag instead of `-p`.
- Core packages must never import provider SDK packages directly.
- Provider modules absorb API drift and map native output/errors to normalized events.
- All providers pass the shared contract test suite in `internal/providers/contract_test.go`.

## Testing strategy
- Unit: onboarding transitions, PRD transitions, selection logic, retry policy.
- Integration: single-story loop in fixture repository per provider.
- Golden: native provider event to normalized event mapping.
- Contract: shared provider test suite all providers must pass.
- Smoke: CLI command and TUI startup.
