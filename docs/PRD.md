# Daedalus PRD

## Product
Daedalus

## One-liner
Codex-native autonomous delivery loop with a TUI, PRD-driven execution, and strict quality gates.

## Problem
Large implementation sessions degrade as context grows. Teams lose determinism, quality drifts, and commit history becomes noisy. We need a repeatable story-at-a-time loop with explicit operator control.

## Goal
Build a Codex plugin-capable orchestrator that:
- Executes one user story per fresh iteration.
- Persists memory and progress between iterations.
- Enforces checks before story completion.
- Exposes live state and controls in a terminal UI.

## Non-goals (v1)
- Cloud/distributed execution.
- Multi-operator concurrency in one runtime.
- Web UI.
- Automatic architecture generation from scratch prompts.

## Users
- Senior engineers running autonomous implementation loops.
- Tech leads who want auditable, story-bound commits.
- Solo builders shipping scoped milestones quickly.

## Principles
- One story per iteration.
- One commit per completed story.
- No hidden state.
- Pause/resume/stop must be instant and safe.
- Minimal irreversible actions by default.

## Success metrics
- >= 90% of stories complete without manual intervention post-setup.
- 100% completed stories pass configured checks.
- <= 5s resume after process restart.
- <= 1 minute initial setup for a repo.

## Functional requirements

### FR-001 PRD lifecycle
System must create, edit, validate, and track PRDs under `.daedalus/prds/<name>/`.

Acceptance:
- `daedalus new [name]` scaffolds `prd.md` and `prd.json`.
- `daedalus validate [name]` validates schema and story ordering.
- `daedalus status [name]` shows completion + active story.

### FR-002 Story state machine
System must persist story lifecycle transitions.

Acceptance:
- States: `pending -> in_progress -> passed`.
- Resume prefers existing `in_progress` story.
- If no in-progress story, pick lowest priority number with `passes=false`.

### FR-003 Provider runner abstraction
System must run agent providers through a stable adapter boundary.

Acceptance:
- Each iteration starts a fresh provider session/run.
- Adapter streams normalized events to loop and TUI.
- Adapter errors are typed and retry-aware.
- Core loop is provider-agnostic.
- Default provider is configurable and defaults to `codex`.

### FR-004 Quality gates
System must execute configured checks before completion.

Acceptance:
- Multiple commands allowed.
- Any failing command blocks completion.
- Failure output persists to event log and progress report.

### FR-005 Git flow
System must support story-scoped commits.

Acceptance:
- Commit format: `feat: [US-XXX] - [Story Title]`.
- Commit only after checks pass.
- Optional worktree/branch isolation mode.

### FR-006 TUI operations
System must provide operator control and observability.

Acceptance:
- Views: dashboard, stories, logs, settings.
- Keys: start, pause, stop, resume, switch PRD.
- Show iteration, active story, branch/worktree, check status.

### FR-007 Persistence and audit
System must persist all operational artifacts.

Acceptance:
- Files per PRD: `prd.md`, `prd.json`, `progress.md`, `agent.log`, `events.jsonl`.
- Restart resumes from persisted state, no story loss.

### FR-008 Plugin mode
System must run as standalone CLI and as Codex plugin integration.

Acceptance:
- Shared core engine, separate entry adapters.
- Plugin path can invoke loop actions without TUI dependency.

## Non-functional requirements
- Security: sanitize/validate inputs; avoid unsafe shell interpolation.
- Reliability: bounded retries with backoff for transient failures.
- Performance: event streaming remains responsive with large outputs.
- Portability: Linux/macOS first; Windows later.
- Observability: structured logs and deterministic event schema.
- Extensibility: adding a new provider does not require core loop refactoring.
- Configurability: retry and provider selection are configurable through TOML config and CLI flags.

## Data model

`prd.json`
- `project: string`
- `description: string`
- `userStories: UserStory[]`

`UserStory`
- `id: string` (`US-001`)
- `title: string`
- `description: string`
- `acceptanceCriteria: string[]`
- `priority: int`
- `passes: bool`
- `inProgress: bool` (optional)

## Milestones

### M1 Foundation
- CLI skeleton: `new`, `list`, `status`, `validate`, `run`.
- PRD service + schema validation.
- Durable state layout.

### M2 Loop core
- Story picker and transition engine.
- Codex SDK adapter integration.
- Event stream and structured logs.

### M3 TUI
- Dashboard + story list + logs + settings.
- Runtime controls (start/pause/stop/resume).
- Error and retry visibility.

### M4 Hardening
- Quality gate pipeline.
- Git commit + optional worktree mode.
- Tests and docs completion.

## Risks
- SDK API/event changes.
- Repo-specific check pipelines can be slow/flaky.
- Ambiguous PRDs can cause poor story decomposition.
- Multi-provider behavior drift (different tool/event semantics).

## Mitigations
- Strict adapter boundary around SDK.
- Configurable retries/timeouts/circuit-breakers.
- PRD validation rules + clear authoring guidance.
- Shared provider contract tests and normalized event golden tests.

## Todo list
- [x] Define product scope.
- [x] Define v1 requirements.
- [x] Define UX/TUI direction.
- [x] Define architecture and component boundaries.
- [x] Define milestones and risk controls.
- [x] Finalize MVP command contract with exact CLI syntax.
- [x] Start implementation scaffold.
