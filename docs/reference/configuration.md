# Daedalus Configuration v1

## Config file location
Linux/XDG:
- `$XDG_CONFIG_HOME/daedalus/config.toml`
- Fallback when `XDG_CONFIG_HOME` is unset: `~/.config/daedalus/config.toml`

Project runtime state remains in:
- `.daedalus/`

## Format
TOML only.

## Resolution priority
1. CLI flags
2. Environment variables
3. `config.toml`
4. Built-in defaults

## Example
```toml
[provider]
default = "codex"

[worktree]
enabled = false

[worktree]
enabled = false

[retry]
max_retries = 3
delays = ["0s", "5s", "15s"]

[quality]
commands = ["go test ./..."]

[ui]
theme = "auto"

[quality]
commands = ["go test ./..."]

[ui]
theme = "auto"

[providers.codex]
model = "default"
approval_policy = "on-failure"
sandbox_policy = "workspace-write"

[providers.claude]
enabled = true
model = "sonnet"
approval_policy = "on-failure"
sandbox_policy = "workspace-write"

[providers.gemini]
enabled = true
model = "gemini-2.0-flash"
# IMPORTANT: Use API key, NOT OAuth. OAuth tokens may result in account bans.

[providers.opencode]
enabled = false
model = "claude-sonnet-4-20250514"

[providers.copilot]
enabled = false
model = "gpt-5"

[providers.qwen]
enabled = false
model = "qwen3-coder-plus"
# Supports Qwen OAuth (free tier) or API key via DASHSCOPE_API_KEY

[providers.pi]
enabled = false
model = "claude-sonnet-4-20250514"
provider = "anthropic"
# Pi supports multiple providers via --provider flag (openai, anthropic, gemini)
```

## Fields

### `[provider]`
- `default: string`
  - Default provider key.
  - v1 default: `"codex"`.

### `[worktree]`
- `enabled: bool`
  - Enables worktree mode for `run`/TUI loop execution.
  - v1 default: `false`.
  - When enabled, runs execute in `.daedalus/worktrees/<prd-name>/` and artifacts still persist under `.daedalus/prds/<prd-name>/`.

### `[worktree]`
- `enabled: bool`
  - Enables worktree mode for `run`/TUI loop execution.
  - v1 default: `false`.
  - When enabled, runs execute in `.daedalus/worktrees/<prd-name>/` and artifacts still persist under `.daedalus/prds/<prd-name>/`.

### `[retry]`
- `max_retries: int`
  - Retry attempts after initial failure.
  - v1 default: `3`.
- `delays: []string`
  - Duration strings for retry wait schedule.
  - v1 default: `["0s", "5s", "15s"]`.
  - If retries exceed delay entries, use the last delay repeatedly.

### `[quality]`
- `commands: []string`
  - Ordered quality gate commands executed after provider iteration and before story completion.
  - Any non-zero exit code fails the run iteration.
  - v1 default: `["go test ./..."]`.

### `[ui]`
- `theme: string`
  - Controls TUI color palette selection.
  - Valid values: `auto`, `dark`, `light`.
  - v1 default: `"auto"`.
  - `auto` attempts OS theme detection (macOS/Windows/Linux desktop hints). If detection is unavailable, Daedalus falls back to `dark`.

### `[quality]`
- `commands: []string`
  - Ordered quality gate commands executed after provider iteration and before story completion.
  - Any non-zero exit code fails the run iteration.
  - v1 default: `["go test ./..."]`.

### `[ui]`
- `theme: string`
  - Controls TUI color palette selection.
  - Valid values: `auto`, `dark`, `light`.
  - v1 default: `"auto"`.
  - `auto` attempts OS theme detection (macOS/Windows/Linux desktop hints). If detection is unavailable, Daedalus falls back to `dark`.

### `[providers.<key>]`
Provider-specific settings. Core reads only normalized fields and passes provider-specific values through module boundaries.

<<<<<<< Updated upstream
`[providers.codex]`:
- `model: string`
  - Optional model name. Default: `"default"`.
- `approval_policy: string`
  - Daedalus policy mapped to Codex approval mode.
  - Allowed: `on-failure`, `on-request`, `never`, `yolo`.
- `sandbox_policy: string`
  - Sandbox policy for model-generated commands.

`[providers.claude]`:
||||||| Stash base
Common normalized fields:
=======
Common normalized fields:
>>>>>>> Stashed changes
- `enabled: bool`
<<<<<<< Updated upstream
  - Enables/disables Claude provider configuration.
  - Default: `true`.
||||||| Stash base
  - Enables/disables Claude provider configuration.
  - Default: `false`.
=======
>>>>>>> Stashed changes
- `model: string`
- `approval_policy: string`
- `sandbox_policy: string`

<<<<<<< Updated upstream
`[providers.gemini]`:
- `enabled: bool`
  - Enables/disables Gemini provider configuration.
  - Default: `true`.
- `model: string`
  - Optional Gemini model name (e.g., `gemini-2.0-flash`).
  - Default: empty (CLI default model).
- `approval_policy: string`
  - Approval policy for Gemini CLI.
- `api_key: string`
  - **Required.** Google Cloud API key. Not OAuth token.
  - Can also be set via `GEMINI_API_KEY` environment variable.
- **Warning:** Using OAuth tokens (from Google AI Ultra/Pro) with third-party tools violates Google ToS and may result in account bans.

`[providers.opencode]`:
- `enabled: bool`
  - Enables/disables OpenCode provider configuration.
  - Default: `false`.
- `model: string`
  - Optional model name passed to `--model` flag.
  - Default: empty (uses default model).
- `api_key: string`
  - Optional API key for OpenCode (if using custom providers).
  - Can also be set via `OPENCODE_API_KEY` environment variable.

`[providers.copilot]`:
- `enabled: bool`
  - Enables/disables Copilot provider configuration.
  - Default: `false`.
- `model: string`
  - Optional model name.
  - Default: empty (uses default model).
- `agent: string`
  - Optional agent to use (e.g., `general-purpose`, `bash-agent`, `code-review-agent`).

`[providers.qwen]`:
- `enabled: bool`
  - Enables/disables Qwen Code provider configuration.
  - Default: `false`.
- `model: string`
  - Optional model name (e.g., `qwen3-coder-plus`).
  - Default: empty (uses default model).
- `api_key: string`
  - API key for Qwen/Dashscope. Can also set via `DASHSCOPE_API_KEY` environment variable.
- **Note:** Qwen OAuth requires browser — use API key for CI/automation.

`[providers.pi]`:
- `enabled: bool`
  - Enables/disables Pi provider configuration.
  - Default: `false`.
- `model: string`
  - Optional model name.
  - Default: empty (uses default model).
- `provider: string`
  - Provider protocol: `openai`, `anthropic`, `gemini`.
  - Default: `openai`.

## CLI overrides (planned)
||||||| Stash base
## CLI overrides (planned)
=======
`approval_policy` valid values (v1):
- `on-failure`
- `on-request`
- `never`

`sandbox_policy` valid values (v1):
- `workspace-write`

Provider support in v1:
- `codex`: implemented through local Codex CLI (`codex exec`).
- `claude`: implemented through local Claude CLI (`claude -p`).
- `gemini`: planned; implementation pending.

Invalid value behavior:
- Unknown `approval_policy` or unsupported `sandbox_policy` fails fast as provider `configuration_error` when the provider run starts.

## CLI overrides
Implemented global flags:
- `--config <path>`
>>>>>>> Stashed changes
- `--provider <name>`
- `--worktree` or `--worktree=<bool>`
- `--worktree` or `--worktree=<bool>`
- `--max-retries <n>`
- `--retry-delays <csv-duration>`

Run command also supports:
- `daedalus run [name] --worktree`
- `daedalus run [name] --worktree=<bool>`

Boolean values accepted for `--worktree` and `DAEDALUS_WORKTREE`:
- true: `1`, `true`, `yes`, `on`
- false: `0`, `false`, `no`, `off`

## Environment overrides
Implemented:
- `DAEDALUS_CONFIG`
Run command also supports:
- `daedalus run [name] --worktree`
- `daedalus run [name] --worktree=<bool>`

Boolean values accepted for `--worktree` and `DAEDALUS_WORKTREE`:
- true: `1`, `true`, `yes`, `on`
- false: `0`, `false`, `no`, `off`

## Environment overrides
Implemented:
- `DAEDALUS_CONFIG`
- `DAEDALUS_PROVIDER`
- `DAEDALUS_WORKTREE`
- `DAEDALUS_WORKTREE`
- `DAEDALUS_MAX_RETRIES`
- `DAEDALUS_RETRY_DELAYS`
- `DAEDALUS_THEME` (`dark` or `light`; overrides `ui.theme`)
- `DAEDALUS_THEME` (`dark` or `light`; overrides `ui.theme`)

## Validation rules
- `provider.default` must not be empty.
- `provider.default` must not be empty.
- `retry.max_retries` must be `>= 0`.
- `retry.delays` values must parse as valid durations.
- Empty `retry.delays` with `max_retries > 0` is invalid.
- `quality.commands` must contain at least one non-empty command.
- `ui.theme` must be one of `auto`, `dark`, or `light`.
- `quality.commands` must contain at least one non-empty command.
- `ui.theme` must be one of `auto`, `dark`, or `light`.
