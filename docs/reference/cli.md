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

Worktree control:
- Global flag: `--worktree` or `--worktree=<bool>`
- Run flag: `daedalus run [name] --worktree` or `--worktree=<bool>`

## Commands

### `daedalus`
Launch TUI runtime.
This is the default UX entrypoint for operators and new users.
Current v1 scaffold provides a structured terminal dashboard with:
- Header status line (loop state, provider, iteration, elapsed time)
- PRD tab strip with quick switch (`1-9`)
- Stories pane + context pane (dashboard/stories/logs/diff/picker/help/settings)
- Activity/footer lines with runtime hints

Views:
- `dashboard` (`d`)
- `stories` (`u`)
- `logs` (`t` toggle from dashboard)
- `diff` (`d` while in dashboard/logs)
- `picker` (`l`)
- `help` (`?`)
- `settings` (`,`)

Interactive key controls (terminal mode):
- `s` / `run` (run one iteration)
- `p` / `pause` (graceful pause after current iteration boundary)
- `x` / `stop` (graceful stop after current iteration boundary)
- `X` (immediate cancellation of active iteration)
- `t` (toggle dashboard/logs)
- `d` (toggle diff/dashboard)
- `l` (open/close PRD picker)
- `n` (create a new PRD with auto-generated name)
- `Tab` / `Shift+Tab` (cycle PRDs)
- `[` / `]` (cycle provider)
- `1-9` (quick switch PRD tab)
- `j` / `k` (story selection or panel scrolling)
- `e` (open selected PRD markdown in local editor)
- `f` (cycle logs filter)
- `+` / `-` (adjust logs tail)
- `?` / `help`
- `q` / `quit`

Command-mode fallback:
- If not attached to an interactive terminal, Daedalus falls back to line-command mode.
- Force command mode in a terminal by setting `DAEDALUS_TUI_FORCE_COMMAND=1`.
- In command mode, textual commands like `provider <name>`, `providers`, `tail <n>`, `status`, and `validate` are available.

TUI runtime control actions (`start`, `pause`, `stop`) are appended to:
- `.daedalus/prds/<name>/events.jsonl`
- `.daedalus/prds/<name>/agent.log`

### `daedalus run [name] [--provider <name>]`
Run one execution iteration for PRD `name`.

Behavior:
- If `name` omitted and exactly one PRD exists, it is auto-selected.
- If multiple PRDs exist and `name` omitted, command fails.
- Run executes headless in v1 and does not start the TUI.
- Story selection is automatic via PRD state; manual story override is out of scope for v1.
- Provider is resolved from `--provider` or config default.
- Retry config resolved from CLI flags or config defaults.
- Worktree mode can be enabled via `--worktree`, `DAEDALUS_WORKTREE`, or `[worktree].enabled`.
- In worktree mode, execution runs in `.daedalus/worktrees/<name>/` on branch `daedalus/<name>`.
- Quality commands are loaded from `[quality].commands` in config and must all pass.
- Current state: Codex provider runs through `codex exec`; Claude provider runs through `claude -p`.

### `daedalus new [name] [context...]`
Create a PRD scaffold under `.daedalus/prds/<name>/`.

Defaults:
- `name` default: `main`.
- `context...` is reserved for future PRD generation flow.
- In v1 scaffold, additional `context...` arguments are accepted but ignored without warning.

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
In v1, `prd.md` remains human-authored context and is not schema-validated.

Checks:
- project is required
- user stories exist
- ID/title/description are required
- priority >= 1
- acceptance criteria are present
- no duplicate IDs
- no duplicate priorities

### `daedalus edit [name]`
Open `.daedalus/prds/<name>/prd.md` in a local editor.

Behavior:
- If `name` omitted and exactly one PRD exists, it is auto-selected.
- Editor command resolution order: `DAEDALUS_EDITOR`, then `EDITOR`, then fallback `vi`.

### `daedalus plugin run [name]`
Run one headless iteration through the plugin adapter path.

Behavior:
- Uses the same core loop as `daedalus run`.
- Emits a JSON result payload to stdout for host/tool consumption.

### `daedalus help`
Show help output.

### `daedalus version`
Show binary version.
Version strategy in v1:
- default value is `dev`
- build pipelines may override via Go linker flags (for example `-ldflags "-X main.version=v1.2.3"`)
