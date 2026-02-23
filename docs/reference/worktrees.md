# Daedalus Worktree Mode (v1)

## Status
Implemented in the current scaffold.

## Goal
Allow isolated PRD execution in a dedicated Git worktree while preserving PRD artifacts in the main project `.daedalus/prds/` directory.

## Naming and paths
- Base path: `.daedalus/worktrees/<prd-name>/`
- Branch name convention: `daedalus/<prd-name>`

## Lifecycle
1. Setup on run start (`--worktree` or `[worktree].enabled = true`):
- verify current project is a Git repository
- use existing managed worktree when it already exists
- otherwise create branch `daedalus/<prd-name>` and add a worktree at `.daedalus/worktrees/<prd-name>/`

2. Run:
- execute provider, quality commands, and story commit from the worktree directory
- persist PRD artifacts under `.daedalus/prds/<name>/` in the main project root

3. Teardown:
- manual/explicit operator action only (no auto-delete in v1)

## Safety rules
- Fail fast when worktree mode is requested outside a Git repository.
- Fail fast when the expected worktree path exists but is not a Git-managed worktree.
- Do not auto-delete worktrees or branches.

## CLI/TUI integration
- Global flag: `--worktree` or `--worktree=<bool>`
- Run flag: `daedalus run [name] --worktree` or `--worktree=<bool>`
- Config toggle: `[worktree] enabled = true|false`
- Environment override: `DAEDALUS_WORKTREE`

## Failure behavior
- Worktree setup failure stops run start and returns actionable error.
- Loop failures inside a worktree follow standard story/loop failure semantics.
