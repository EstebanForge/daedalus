# AGENTS.md

This file is the operating guide for agents working in the Daedalus repository.

## Project Intent
- Build Daedalus as a Go-first autonomous delivery CLI/TUI.
- Keep core engine provider-agnostic via ACP (Agent Client Protocol).
- All providers use ACP for standardized communication.

## Tech Stack
- Language: Go (module: `github.com/EstebanForge/daedalus`)
- Config: TOML
- Global config path:
- `$XDG_CONFIG_HOME/daedalus/config.toml`
- Fallback: `~/.config/daedalus/config.toml`
- Project runtime state: `.daedalus/`

## Current Architecture
- `cmd/daedalus/main.go`: binary entrypoint
- `internal/app`: CLI routing, TUI, onboarding TUI, runtime option resolution
- `internal/config`: TOML config defaults/load/validate
- `internal/onboarding`: onboarding state machine and manager
- `internal/prd`: PRD model, validation, storage
- `internal/loop`: story execution loop + retry policy
- `internal/providers`: ACP provider runtime, provider contract, errors, and registry for all 7 provider keys
- `internal/quality`: quality gate runner (executes configured check commands)
- `internal/git`: story-scoped commit service
- `internal/worktree`: git worktree lifecycle manager
- `internal/project`: `.daedalus/` path conventions
- `internal/templates`: embedded canonical markdown templates for all planning artifacts
- `docs/`: all planning/architecture/design/reference markdown

## Non-Negotiable Design Rules
- Keep core runtime provider-agnostic via ACP.
- Do not import provider SDKs in core packages.
- All provider behavior must go through `internal/providers` contract.
- Provider-specific details must stay in optional metadata, never core branch logic.
- ACP is the only supported transport. CLI-based providers are no longer supported.
- Keep changes minimal, explicit, testable.

## Provider Model
- Provider keys:
- `codex` (default)
- `claude`
- `gemini`
- `opencode`
- `copilot`
- `qwen`
- `pi`
- Registry resolves provider by key and returns explicit configuration errors.

## Runtime Defaults
- Default provider: `codex`
- Retry defaults:
- `max_retries = 3`
- `delays = ["0s", "5s", "15s"]`

## CLI Contract (v1)
- `daedalus new [name]`
- `daedalus list`
- `daedalus status [name]`
- `daedalus validate [name]`
- `daedalus run [name] [--provider <name>] [--max-retries <n>] [--retry-delays <csv>]`
- `daedalus help`
- `daedalus version`

## Build, Lint, Test
- Format: `gofmt -w <files>`
- Vet: `go vet ./...`
- Lint: `golangci-lint run ./...`
- Test: `go test ./...`
- Verification is incomplete unless lint, vet, and tests all pass.

## Agent Workflow Protocol
- Search first, code second.
- Update docs if behavior/contracts change.
- Prefer focused patches over broad rewrites.
- Validate before yielding.
- Report what changed, what was verified, and what remains.

## Definition of Done
- Code compiles.
- `go test ./...` passes.
- `go vet ./...` passes.
- `golangci-lint run ./...` passes.
- Docs and CLI help text match implementation.

## Documentation Policy
- Keep planning and architecture docs under `docs/` only.
- Keep CLI/config/provider contract docs in `docs/reference/`.
- Update `docs/reference/configuration.md` when config keys/defaults change.
- Update `docs/reference/providers.md` when provider contract changes.

## Safety Rules
- Never use destructive git commands unless explicitly requested.
- Never revert user changes you did not author.
- Avoid hidden side effects.
- Fail fast on invalid config, invalid provider key, and invalid retry values.

## Implementation Notes
- Prefer adding tests with new behavior, especially for config parsing, provider resolution, and retry handling.
- All seven providers (codex, claude, gemini, opencode, copilot, qwen, pi) are fully implemented in `internal/providers`.
- Shared provider contract tests live in `internal/providers/contract_test.go` — all new providers must pass them.
- All planning artifact generation (project-summary, jtbd, architecture-design, prd) must use `internal/templates` as the canonical structure source. Do not hard-code markdown headings or sections elsewhere.

## Document Templates
Canonical templates for all planning artifacts are embedded in `internal/templates/`:
- `project-summary.md` — LLM fills this in during the onboarding repository scan.
- `jtbd.md` — Jobs-to-be-Done; human-authored, LLM-drafted in existing-project mode.
- `architecture-design.md` — Architecture context; human-authored, scan-seeded in existing-project mode.
- `prd.md` — PRD narrative; created alongside every new prd.json.

Rules:
- The templates are the single source of truth for document structure.
- Any code that generates or seeds these documents must import `internal/templates`.
- Headings must not be changed without updating the corresponding template and its tests.
