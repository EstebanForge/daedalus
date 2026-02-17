# AGENTS.md

This file is the operating guide for agents working in the Daedalus repository.

## Project Intent
- Build Daedalus as a Go-first autonomous delivery CLI/TUI.
- Keep core engine provider-agnostic.
- Ship Codex first, keep Claude/Gemini integration-ready without refactoring core.

## Tech Stack
- Language: Go (module: `github.com/EstebanForge/daedalus`)
- Config: TOML
- Global config path:
- `$XDG_CONFIG_HOME/daedalus/config.toml`
- Fallback: `~/.config/daedalus/config.toml`
- Project runtime state: `.daedalus/`

## Current Architecture
- `cmd/daedalus/main.go`: binary entrypoint
- `internal/app`: CLI routing and runtime option resolution
- `internal/config`: TOML config defaults/load/validate
- `internal/prd`: PRD model, validation, storage
- `internal/loop`: story execution loop + retry policy
- `internal/providers`: provider contract, errors, registry, provider modules
- `internal/project`: `.daedalus/` path conventions
- `docs/`: all planning/architecture/design/reference markdown

## Non-Negotiable Design Rules
- Keep core runtime provider-agnostic.
- Do not import provider SDKs in core packages.
- All provider behavior must go through `internal/providers` contract.
- Provider-specific details must stay in optional metadata, never core branch logic.
- Preserve backward-compatible CLI behavior unless explicitly instructed otherwise.
- Keep changes minimal, explicit, testable.

## Provider Model
- Provider keys:
- `codex` (default)
- `claude` (planned)
- `gemini` (planned)
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
- `internal/agent` is legacy scaffold and should be phased out in favor of `internal/providers`.
- Prefer adding tests with new behavior, especially for config parsing, provider resolution, and retry handling.
