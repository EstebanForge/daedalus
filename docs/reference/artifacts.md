# Daedalus Runtime Artifacts v1

## Location
Per PRD runtime artifacts live under:
- `.daedalus/prds/<name>/`

Files:
- `prd.md`
- `prd.json`
- `progress.md`
- `agent.log`
- `events.jsonl`

## `prd.md` role
- `prd.md` is human-authored narrative context for the project and stories.
- `prd.json` is the executable source of truth for runtime selection and state transitions.
- `daedalus validate [name]` validates `prd.json` schema/consistency in v1.
- `daedalus validate [name]` does not parse `prd.md` structure beyond file presence checks.

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

Example line:
```json
{"type":"tool_started","message":"running go test ./...","timestamp":"2026-02-22T16:45:12Z","iteration":3,"storyID":"US-003","metadata":{"provider":"claude","tool":"shell"}}
```

Rules:
- Event `type` must be one of normalized event types from `docs/reference/providers.md`.
- `metadata` must be informational only; core state transitions must not depend on provider-specific metadata keys.

## `progress.md` format
`progress.md` is append-only and human-readable. Each completed or failed iteration appends one block.

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

## Story terminal failure and reset behavior (v1)
- On terminal iteration failure (all retries exhausted or unrecoverable error), loop state becomes `error`.
- Active story remains `in_progress=true` and `passes=false` (sticky recovery).
- No automatic transition to a `failed` story state in v1.
- Operator recovery paths:
- `daedalus run [name]` resumes the same `in_progress` story.
- Manual reset is explicit user action by editing `prd.json` (`inProgress=false`, `passes=false`) for the story, then re-running.
