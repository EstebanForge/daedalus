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

[retry]
max_retries = 3
delays = ["0s", "5s", "15s"]

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

### `[retry]`
- `max_retries: int`
  - Retry attempts after initial failure.
  - v1 default: `3`.
- `delays: []string`
  - Duration strings for retry wait schedule.
  - v1 default: `["0s", "5s", "15s"]`.
  - If retries exceed delay entries, use the last delay repeatedly.

### `[providers.<key>]`
Provider-specific settings. Core reads only normalized fields and passes provider-specific values through module boundaries.

`[providers.codex]`:
- `model: string`
  - Optional model name. Default: `"default"`.
- `approval_policy: string`
  - Daedalus policy mapped to Codex approval mode.
  - Allowed: `on-failure`, `on-request`, `never`, `yolo`.
- `sandbox_policy: string`
  - Sandbox policy for model-generated commands.

`[providers.claude]`:
- `enabled: bool`
  - Enables/disables Claude provider configuration.
  - Default: `true`.
- `model: string`
  - Optional Claude model alias/name passed to `claude --model`.
  - Default: empty (CLI default model).
- `approval_policy: string`
  - Daedalus policy mapped to Claude `--permission-mode`.
  - Allowed: `on-failure`, `on-request`, `never`.
- `sandbox_policy: string`
  - Currently supported: `workspace-write`.

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
- `--provider <name>`
- `--max-retries <n>`
- `--retry-delays <csv-duration>`

## Environment overrides (planned)
- `DAEDALUS_PROVIDER`
- `DAEDALUS_MAX_RETRIES`
- `DAEDALUS_RETRY_DELAYS`
- `DAEDALUS_CONFIG` (explicit config file path)

## Validation rules
- `provider.default` must be a known provider key.
- `retry.max_retries` must be `>= 0`.
- `retry.delays` values must parse as valid durations.
- Empty `retry.delays` with `max_retries > 0` is invalid.
