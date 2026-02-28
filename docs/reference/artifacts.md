# Daedalus Runtime Artifacts v1

## Locations

Current scaffold runtime artifacts per PRD:
- `.daedalus/prds/<name>/`

Workflow-extension onboarding artifacts:
- `.daedalus/onboarding/state.json`

ACP runtime cache:
- `.daedalus/acp-sessions.json`

## Files

### Core PRD runtime files (implemented)
- `prd.md`
- `prd.json`
- `progress.md`
- `agent.log`
- `events.jsonl`

### Onboarding/context files (implemented)
- `.daedalus/onboarding/state.json`
- `.daedalus/prds/<name>/project-summary.md`
- `.daedalus/prds/<name>/jtbd.md`
- `.daedalus/prds/<name>/architecture-design.md`

### ACP session cache (implemented)
- `.daedalus/acp-sessions.json`

Purpose:
- Zero-config best-effort session resume across process restarts.
- Keyed by provider + working directory.

Behavior:
- Cache is advisory; providers may reject stale session IDs.
- On resume failure, runtime falls back to creating a fresh ACP session.

All three markdown files above are generated from canonical templates embedded in
`internal/templates/`. The templates define fixed section headings that must be
preserved regardless of which LLM provider produced the content. This ensures
deterministic structure across all seven provider backends.

Template source files:
- `internal/templates/project-summary.md`
- `internal/templates/jtbd.md`
- `internal/templates/architecture-design.md`
- `internal/templates/prd.md`

## `prd.md` role
- Narrative planning context for project/stories.
- `prd.json` remains executable source of truth for runtime selection and state transitions.
- `daedalus validate [name]` validates `prd.json` schema/consistency in v1.

## `events.jsonl` schema
One JSON object per line, append-only, ordered by emission time.

Required fields:
- `type: string`
- `message: string`
- `timestamp: string` (RFC3339)
- `iteration: int` (`>= 1`)

Optional fields:
- `storyID: string`
- `metadata: object<string,string>`

Example:
```json
{"type":"tool_started","message":"running go test ./...","timestamp":"2026-02-22T16:45:12Z","iteration":3,"storyID":"US-003","metadata":{"provider":"claude","tool":"shell"}}
```

Rules:
- `type` must be one of normalized event types from `docs/reference/providers.md`.
- `metadata` is informational only.

## `progress.md` format
Append-only, human-readable.

Template:
```md
## Iteration <n> - <story-id> - <status>
Date: <UTC RFC3339 timestamp>
Provider: <provider-key>

Summary:
<provider summary or operator note>

Checks:
- <command>: PASS|FAIL (exit=<code>)

Commit:
- <commit sha or "not committed">

Retry:
- attempt <current>/<max>
- delay used: <duration>
```

`<status>` values:
- `passed`
- `failed`
- `error`
- `cancelled`

## `onboarding/state.json` format (implemented)
Tracks onboarding completion and resumption.

Suggested shape:
```json
{
  "completed": false,
  "project_mode": "existing_project",
  "steps": {
    "git_ignore": true,
    "project_discovery": true,
    "jtbd": false,
    "create_prd": false
  },
  "updated_at": "2026-02-24T20:55:00Z"
}
```

Rules:
- Startup runs onboarding when file is missing or `completed=false`.
- Resume starts from first incomplete step.

## Existing-project scan outputs (implemented)
- `project-summary.md` contains structured repository summary sections:
- `purpose`
- `architecture`
- `tech_stack`
- `key_modules`
- `test_lint_commands`
- `active_risks`

Seed usage:
- `jtbd.md` draft from user project description + scan summary.
- `architecture-design.md` seeded from scan summary.
- `prd.md` context seeded from approved JTBD + project summary.

## Story terminal failure and reset behavior (v1)
- On terminal iteration failure, loop state becomes `error`.
- Active story remains `in_progress=true`, `passes=false`.
- No automatic transition to a `failed` story state in v1.
- Recovery:
- `daedalus run [name]` resumes current `in_progress` story.
- Manual reset via explicit edit in `prd.json`.
