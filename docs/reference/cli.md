# Daedalus CLI v1

## Usage
`daedalus [command] [flags]`

## Runtime startup behavior

### Startup behavior
- `daedalus` launches the TUI runtime.
- If `.daedalus/` is missing or onboarding state is incomplete, Daedalus starts onboarding before normal runtime.
- Onboarding resumes at the first incomplete step.
- Existing-project discovery runs when any file/folder other than `.daedalus/` exists in the current directory.
- If not in an interactive terminal, Daedalus falls back to command mode.

## Global flags (implemented)
- `--config <path>`
- `--provider <name>`
- `--worktree` or `--worktree=<bool>`
- `--max-retries <n>`
- `--retry-delays <csv-duration>`

Provider selection:
- Supported keys: `codex`, `claude`, `gemini`, `opencode`, `copilot`, `qwen`, `pi`
- Default: `codex`

Retry defaults:
- `--max-retries 3`
- `--retry-delays 0s,5s,15s`

Worktree control:
- Global: `--worktree` or `--worktree=<bool>`
- Run command: `daedalus run [name] --worktree` or `--worktree=<bool>`

## Planned global flags (proposal alignment)
- `--push-on-complete` / `--push-on-complete=<bool>`
- `--auto-pr-on-complete` / `--auto-pr-on-complete=<bool>`

These map to post-completion defaults and are intentionally outside onboarding.

## Commands

### `daedalus`
Launch TUI runtime.

Current scaffold provides:
- Header status line (loop state, provider, iteration, elapsed time)
- PRD tab strip (`1-9`)
- Stories pane + context pane (dashboard/stories/logs/diff/picker/help/settings)
- Activity/footer lines

Views:
- `dashboard` (`d`)
- `stories` (`u`)
- `logs` (`t` toggle)
- `diff` (`d` while in dashboard/logs)
- `picker` (`l`)
- `help` (`?`)
- `settings` (`,`) 

Interactive key controls:
- `s` / `run`
- `p` / `pause`
- `x` / `stop`
- `X` (immediate cancellation)
- `t` (dashboard/logs)
- `d` (diff/dashboard)
- `l` (PRD picker)
- `n` (new PRD with auto-generated name)
- `Tab` / `Shift+Tab` (cycle PRDs)
- `[` / `]` (cycle provider)
- `1-9` (switch PRD tab)
- `j` / `k` (navigate)
- `e` (open PRD markdown in editor)
- `f` (cycle logs filter)
- `+` / `-` (logs tail)
- `?` / `help`
- `q` / `quit`

Command mode fallback:
- If not attached to an interactive terminal, Daedalus falls back to line-command mode.
- Force command mode with `DAEDALUS_TUI_FORCE_COMMAND=1`.

### `daedalus run [name] [--provider <name>]`
Run one execution iteration for PRD `name`.

Behavior:
- If `name` omitted and exactly one PRD exists, it is auto-selected.
- If multiple PRDs exist and `name` omitted, command fails.
- Run executes headless and does not start the TUI.
- Story selection is automatic via PRD state.
- Provider is resolved from `--provider` or config default.
- Retry config is resolved from CLI flags, env vars, or config defaults.
- Worktree mode can be enabled via `--worktree`, `DAEDALUS_WORKTREE`, or `[worktree].enabled`.
- In worktree mode, execution runs in `.daedalus/worktrees/<name>/` on branch `daedalus/<name>`.
- Quality commands are loaded from `[quality].commands` and all must pass.

### `daedalus new [name] [context...]`
Create a PRD scaffold under `.daedalus/prds/<name>/`.

Defaults:
- `name` default: `main`
- `context...` is accepted but ignored in current scaffold

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
- story ID/title/description are required
- priority >= 1
- acceptance criteria are present
- no duplicate IDs
- no duplicate priorities

### `daedalus edit [name]`
Open `.daedalus/prds/<name>/prd.md` in a local editor.

Editor resolution order:
- `DAEDALUS_EDITOR`
- `EDITOR`
- fallback `vi`

### `daedalus plugin run [name]`
Run one headless iteration through plugin adapter path.

Behavior:
- Uses the same core loop as `daedalus run`
- Emits JSON result payload to stdout

### `daedalus help`
Show help output.

### `daedalus version`
Show binary version.

Version strategy:
- default value: `dev`
- build pipelines may override with linker flags
