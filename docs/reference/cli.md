# Daedalus CLI v1

## Usage
`daedalus [command] [flags]`

Provider selection:
- Global flag: `--provider <name>`
- Supported keys: `codex`, `claude`, `gemini`
- Default in v1: `codex`

Retry controls:
- Global flag: `--max-retries <n>`
- Global flag: `--retry-delays <csv-duration>`
- Defaults: `--max-retries 3`, `--retry-delays 0s,5s,15s`

## Commands

### `daedalus`
Launch TUI runtime.

Current state: reserved; not implemented yet.

### `daedalus run [name] [--provider <name>]`
Run one execution iteration for PRD `name`.

Behavior:
- If `name` omitted and exactly one PRD exists, it is auto-selected.
- If multiple PRDs exist and `name` omitted, command fails.
- Provider is resolved from `--provider` or config default.
- Retry config resolved from CLI flags or config defaults.
- Current state: Claude provider runs through the `claude` CLI in print mode; Codex adapter remains pending.

### `daedalus new [name] [context...]`
Create a PRD scaffold under `.daedalus/prds/<name>/`.

Defaults:
- `name` default: `main`.
- `context...` reserved for future PRD generation flow.

Artifacts:
- `prd.md`
- `prd.json`
- `progress.md`

### `daedalus list`
List discovered PRDs with summary counters.

### `daedalus status [name]`
Show story totals and next story for a PRD.

### `daedalus validate [name]`
Validate `prd.json` schema and consistency.

Checks:
- project is required
- user stories exist
- ID/title/description are required
- priority >= 1
- acceptance criteria are present
- no duplicate IDs
- no duplicate priorities

### `daedalus edit [name]`
Reserved command; not implemented in v1 scaffold.

### `daedalus help`
Show help output.

### `daedalus version`
Show binary version.
