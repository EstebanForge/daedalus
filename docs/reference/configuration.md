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
enabled = false

[providers.gemini]
enabled = false
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
