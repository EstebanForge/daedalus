# Daedalus PRD

## Product
Daedalus

## One-liner
Codex-native autonomous delivery loop with onboarding-first setup, project discovery, PRD-driven execution, and strict quality gates.

## Problem
Large implementation sessions degrade as context grows. Teams lose determinism, quality drifts, and commit history becomes noisy. We need a repeatable story-at-a-time loop with explicit operator control.

## Goal
Build an autonomous delivery orchestrator supporting Codex, Claude, Gemini, OpenCode, Copilot, Qwen Code, and Pi CLI through their non-interactive CLI modes.
- Executes one user story per fresh iteration.
- Persists memory and progress between iterations.
- Enforces checks before story completion.
- Exposes live state and controls in a terminal UI.
- Bootstraps new and existing repositories with mandatory onboarding before execution.

## Non-goals (v1)
- Cloud/distributed execution.
- Multi-operator concurrency in one runtime.
- Web UI.
- Automatic architecture generation from scratch prompts without user review.

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
- Onboarding is mandatory when runtime state is missing/incomplete.

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
- Terminal failures keep the story `in_progress` for explicit operator recovery; no automatic `failed` story state in v1.

### FR-003 Provider runner abstraction
System must run agent providers through a stable adapter boundary.

Acceptance:
- Each iteration starts a fresh provider session/run.
- Adapter streams normalized events to loop and TUI.
- Adapter errors are typed and retry-aware.
- Core loop is provider-agnostic.
- Default provider is configurable and defaults to `codex`.
- All seven providers (Codex, Claude, Gemini, OpenCode, Copilot, Qwen Code, Pi) integrate via CLI adapters:
  - Claude: `claude -p`
  - Gemini: `gemini -p` (API key, not OAuth)
  - Codex: `codex exec`
  - OpenCode: `opencode -p`
  - Copilot: `copilot -p` / `copilot --prompt`
  - Qwen Code: `qwen -p`
  - Pi: `pi -p`

### FR-004 Quality gates
System must execute configured checks before completion.

Acceptance:
- Multiple commands allowed.
- Any failing command blocks completion.
- Failure output persists to event log and progress report.

### FR-005 Git flow
System must support story-scoped commits.

Acceptance:
- Commit format: `feat(US-XXX): Story Title`.
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
- Restart resumes from persisted state with no story loss.

### FR-008 Plugin mode
System must run as standalone CLI and as plugin integration.

Acceptance:
- Shared core engine, separate entry adapters.
- Plugin path can invoke loop actions without TUI dependency.
- Initial adapter command: `daedalus plugin run [name]`.

### FR-009 First-run onboarding and project discovery
System must run onboarding before planning/execution when `.daedalus/` is missing or onboarding is incomplete.

Acceptance:
- Onboarding is mandatory and resumable; interrupted setup resumes at first incomplete step.
- Existing project mode is detected when current directory contains any file/folder other than `.daedalus/`.
- Empty folder mode is detected when no file/folder exists other than `.daedalus/`.
- Screen 1 asks whether to add `.daedalus/` to `.gitignore`, with `pros`, `cons`, and `use cases` for both choices.
- Existing project mode prompts user for a plain-language project description.
- Existing project mode runs an agent-driven read-only repository scan in background with visible progress UI (spinner/loader + status).
- Scan output includes purpose, architecture, stack, key modules, test/lint commands, and active risks.
- Scan failures show actionable errors and allow retry without losing prior onboarding inputs.

### FR-010 Scan-seeded planning docs
System must seed planning docs from onboarding outputs.

Acceptance:
- Empty folder mode captures JTBD from direct user input.
- Existing project mode drafts JTBD from user description + scan summary, with user review/edit.
- Existing project mode pre-fills Architecture & Design doc context from scan summary.
- PRD context is seeded from approved JTBD + project summary context.

### FR-011 Completion behavior defaults and flags
Post-completion automation must be configured via defaults and flags, not onboarding screens.

Acceptance:
- Sensible defaults are disabled: push disabled, auto-PR disabled.
- Configuration and CLI flags define/override completion behavior.
- Onboarding does not block on post-completion automation choices.

## Non-functional requirements
- Security: sanitize/validate inputs; avoid unsafe shell interpolation.
- Reliability: bounded retries with backoff for transient failures.
- Performance: event streaming remains responsive with large outputs.
- Portability: Linux/macOS first; Windows later.
- Observability: structured logs and deterministic event schema.
- Extensibility: adding a new provider does not require core loop refactoring.
- Configurability: retry/provider/completion behavior are configurable through TOML config and CLI flags.

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

### M1 Foundation ✓
- [x] CLI skeleton: `new`, `list`, `status`, `validate`, `run`.
- [x] PRD service + schema validation.
- [x] Durable state layout.
- [x] Onboarding trigger/resume scaffold.

### M2 Onboarding and context seeding ✓
- [x] Git-ignore decision screen.
- [x] Existing-project detection + background scan + progress UI.
- [x] JTBD capture/review flow.
- [x] Seed JTBD/Architecture context docs.

### M3 Loop core + TUI ✓
- [x] Story picker and transition engine.
- [x] Codex provider adapter integration.
- [x] Dashboard + story list + logs + settings.
- [x] Runtime controls (start/pause/stop/resume).

### M4 Hardening ✓
- [x] Quality gate pipeline.
- [x] Git commit + optional worktree mode.
- [x] TUI polish and hardening:
  - richer live event streaming
  - stronger pause/stop lifecycle controls
  - visual/interaction refinement
- [x] Tests and docs completion.

### M5 Multi-provider hardening ✓
- [x] Gemini provider module behind shared provider contract.
- [x] Multi-provider contract conformance and regression hardening.
- [x] All 7 providers implemented: codex, claude, gemini, opencode, copilot, qwen, pi.
- [x] Shared contract test suite covering all providers.

## Risks
- Provider API/CLI and policy changes.
- Repo-specific check pipelines can be slow/flaky.
- Ambiguous PRDs can cause poor story decomposition.
- Multi-provider behavior drift (different tool/event semantics).
- Poor repository scan quality can seed incorrect planning context.

## Mitigations
- Strict adapter boundary around provider integrations (SDK/CLI).
- Configurable retries/timeouts/circuit-breakers.
- PRD validation rules + clear authoring guidance.
- Shared provider contract tests and normalized event golden tests.
- Require explicit user review/edit for generated JTBD and seeded context.

## Todo list
- [x] Define product scope.
- [x] Define v1 requirements.
- [x] Define UX/TUI direction.
- [x] Define architecture and component boundaries.
- [x] Define milestones and risk controls.
- [x] Finalize MVP command contract with exact CLI syntax.
- [x] Start implementation scaffold.
- [x] Implement onboarding + existing-project discovery flow.
- [x] Complete Gemini provider module implementation.
- [x] Complete all provider module implementations (OpenCode, Copilot, Qwen Code, Pi).
- [x] Complete TUI polish and hardening scope.
