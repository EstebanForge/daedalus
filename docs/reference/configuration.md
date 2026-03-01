# Daedalus Configuration v1

## Config file location
Linux/XDG:
- `$XDG_CONFIG_HOME/daedalus/config.toml`
- Fallback: `~/.config/daedalus/config.toml`

Project runtime state:
- `.daedalus/`

## Format
TOML.

## Resolution priority
1. CLI flags
2. Environment variables
3. `config.toml`
4. Built-in defaults

## Current scaffold example (implemented)
```toml
[provider]
default = "codex"

[worktree]
enabled = false

[retry]
max_retries = 3
delays = ["0s", "5s", "15s"]

[quality]
commands = ["go test ./..."]

[ui]
theme = "auto"

[providers.codex]
enabled = true
model = "default"
approval_policy = "on-failure"
sandbox_policy = "workspace-write"
acp_command = "codex-acp"

[providers.claude]
enabled = false

[providers.gemini]
enabled = false

[providers.opencode]
enabled = false

[providers.copilot]
enabled = false

[providers.qwen]
enabled = false

[providers.pi]
enabled = false
```

## Planned extension example (workflow proposal v1.4)
```toml
[completion]
push_on_complete = false
auto_pr_on_complete = false
```

`[completion]` is documented for workflow alignment but is not implemented in current scaffold.

## Fields (implemented)

### `[provider]`
- `default: string`
  - Default provider key.
  - Default: `"codex"`.

### `[worktree]`
- `enabled: bool`
  - Enables worktree mode for `run`/TUI loop execution.
  - Default: `false`.

### `[retry]`
- `max_retries: int`
  - Retry attempts after initial failure.
  - Default: `3`.
- `delays: []string`
  - Duration strings for retry schedule.
  - Default: `["0s", "5s", "15s"]`.
  - If retries exceed entries, last delay repeats.

### `[quality]`
- `commands: []string`
  - Ordered quality gate commands.
  - Any non-zero exit fails iteration.
  - Default: `["go test ./..."]`.

### `[ui]`
- `theme: string`
  - Valid values: `auto`, `dark`, `light`.
  - Default: `"auto"`.
  - `auto` attempts environment detection; falls back to `dark`.

### `[providers.<key>]`
Supported keys:
- `codex`
- `claude`
- `gemini`
- `opencode`
- `copilot`
- `qwen`
- `pi`

Common fields:
- `enabled: bool`
- `model: string`
- `approval_policy: string` — handled via ACP protocol when supported
- `sandbox_policy: string` — handled via ACP protocol when supported
- `acp_command: string` — optional ACP executable/command override per provider

**Note:** With ACP transport, approval and sandbox policies are handled at the protocol level. Some providers may not support all policy modes. Check `docs/reference/providers.md` for provider-specific capabilities.

Runtime behavior:
- `daedalus run` resolves `model`, `approval_policy`, and `sandbox_policy` from the selected provider config.
- Daedalus validates these values against negotiated provider capabilities before prompting.
- Use `daedalus doctor` to probe ACP binary/initialize/session health and inspect negotiated capability support.

Defaults:
- `providers.codex.enabled = true`
- `providers.codex.model = "default"`
- `providers.codex.approval_policy = "on-failure"`
- `providers.codex.sandbox_policy = "workspace-write"`
- `providers.codex.acp_command = ""` (runtime provider default applies)
- All other providers: `enabled = false`

Policy value notes:
- `approval_policy` values used by scaffold/provider docs: `on-failure`, `on-request`, `never`
- `sandbox_policy` value used by scaffold/provider docs: `workspace-write`

ACP command notes:
- Empty `acp_command` uses provider runtime defaults.
- Set this for any provider when the command/binary differs from runtime defaults.

## Planned fields (not implemented)

### `[completion]`
- `push_on_complete: bool`
  - Default: `false`
- `auto_pr_on_complete: bool`
  - Default: `false`

Purpose:
- Controls post-completion push/PR automation through config and flags.
- Kept out of onboarding screens.

## CLI overrides (implemented)
Global flags:
- `--config <path>`
- `--provider <name>`
- `--worktree` or `--worktree=<bool>`
- `--max-retries <n>`
- `--retry-delays <csv-duration>`

Run command supports:
- `daedalus run [name] --provider <name>`
- `daedalus run [name] --worktree`
- `daedalus run [name] --worktree=<bool>`
- `daedalus run [name] --max-retries <n>`
- `daedalus run [name] --retry-delays <csv-duration>`

Boolean values for `--worktree`:
- true: `1`, `true`, `yes`, `on`
- false: `0`, `false`, `no`, `off`

## CLI overrides (planned)
- `--push-on-complete` or `--push-on-complete=<bool>`
- `--auto-pr-on-complete` or `--auto-pr-on-complete=<bool>`

## Environment overrides (implemented)
- `DAEDALUS_CONFIG`
- `DAEDALUS_PROVIDER`
- `DAEDALUS_WORKTREE`
- `DAEDALUS_MAX_RETRIES`
- `DAEDALUS_RETRY_DELAYS`
- `DAEDALUS_THEME` (`dark` or `light`)

## Environment overrides (planned)
- `DAEDALUS_PUSH_ON_COMPLETE`
- `DAEDALUS_AUTO_PR_ON_COMPLETE`

## Validation rules (implemented)
- `provider.default` must not be empty.
- `retry.max_retries` must be `>= 0`.
- `retry.delays` values must parse as valid durations.
- Empty `retry.delays` with `max_retries > 0` is invalid.
- `quality.commands` must contain at least one non-empty command.
- `ui.theme` must be one of `auto`, `dark`, `light`.
- Selected provider key must resolve to a registered and enabled provider.

## Validation rules (planned)
- `completion.auto_pr_on_complete=true` should require `completion.push_on_complete=true`.
