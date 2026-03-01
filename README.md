# Daedalus

Daedalus is a Go-first autonomous delivery CLI/TUI. It runs one PRD story at a time, enforces quality gates, and integrates all providers through ACP (Agent Client Protocol).

## Highlights

- Provider-agnostic core loop (`codex`, `claude`, `gemini`, `opencode`, `copilot`, `qwen`, `pi`).
- ACP session reuse with persisted cache in `.daedalus/acp-sessions.json`.
- Onboarding flow for new and existing repositories.
- Quality-gated story completion with structured artifacts and logs.
- Runtime observability commands:
  - `daedalus doctor [provider...]`
  - `daedalus sessions [list|status] [provider]`

## Build

```bash
make build
```

Binary output:

- `bin/daedalus`

## Quick Start

```bash
# launch TUI (runs onboarding when required)
daedalus

# create and inspect PRD
daedalus new main
daedalus status main
daedalus validate main

# run one iteration
daedalus run main --provider codex

# ACP diagnostics
daedalus doctor
daedalus sessions list
daedalus sessions status codex
```

## Config

Daedalus reads TOML config from:

- `$XDG_CONFIG_HOME/daedalus/config.toml`
- fallback: `~/.config/daedalus/config.toml`

Project runtime state is stored under:

- `.daedalus/`

See:

- `docs/reference/configuration.md`
- `docs/reference/cli.md`
- `docs/reference/providers.md`
- `docs/reference/artifacts.md`

## Verification

```bash
make check
```

