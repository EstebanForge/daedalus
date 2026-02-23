# Documentation Review — Daedalus

Reviewed: 2026-02-17 (docs) · 2026-02-22 (codebase)
Scope: `docs/PRD.md`, `docs/ARCHITECTURE.md`, `docs/DESIGN.md`, `docs/reference/cli.md`, `docs/reference/configuration.md`, `docs/reference/providers.md`, `internal/` (all Go packages)

---

## Overall Assessment

The documentation is well-structured and internally consistent. The architecture, provider contract, and data model are solid foundations. The following issues prevent a clean handoff to developers. They are categorized by severity.

The 2026-02-22 codebase review added items H-6 through H-9, M-7, and L-4, which cover implementation gaps found by reading the Go source rather than the docs.

---

## BLOCKERS — resolve before development starts

### B-1: Three open design decisions (DESIGN.md:99-103)

Status: resolved on 2026-02-22.

These are explicitly listed as "open" and each one directly affects implementation scope:

**Headless vs TUI for `run`**
Does `daedalus run` operate fully headless or does it share/spawn the TUI runtime? This changes the entire `run` command implementation and how the event bus is wired.

**Manual story selection in v1**
No decision recorded. Affects the CLI API (`run [name] --story <id>`?), the PRD service `getNextStory` contract, and the loop manager.

**Auto-commit vs operator confirmation**
No decision recorded. Affects the git service flow, TUI controls, and the loop step sequence in ARCHITECTURE.md.

---

### B-2: Prompt construction not specified (ARCHITECTURE.md loop step 4)

Status: resolved on 2026-02-22.

The loop step reads "Build prompt/context" with no further detail. There is no specification for:
- What the prompt sent to the provider contains (story title? description? acceptance criteria? surrounding PRD context?).
- Which files populate `contextFiles` in `IterationRequest` and how they are selected.

This is the primary input to every provider adapter. Without a spec, each developer will invent a different shape.

---

### B-3: `RunIteration` return semantics ambiguous (providers.md:14, ARCHITECTURE.md:74)

Status: resolved on 2026-02-22.

The signature `RunIteration(ctx, request) -> (<-chan Event, IterationResult, error)` returns three values simultaneously but the semantics are never specified:
- Is `error` only for pre-start failures (bad config, can't launch), while in-flight failures arrive via the channel?
- Is `IterationResult` only valid after the channel is closed?
- What is the channel's behavior when `ctx` is cancelled — closed immediately? Drained first?

The test requirements reference "cancellation and graceful shutdown behavior" but define no expected behavior.

---

### B-4: `approval_policy` and `sandbox_policy` valid values not defined (configuration.md)

Status: resolved on 2026-02-22.

The example config shows `approval_policy = "on-failure"` and `sandbox_policy = "workspace-write"` but neither field lists its full set of valid values, behavior per value, or what happens on an invalid input. These values are passed through to provider adapters. Developers implementing provider modules have no contract to code against.

---

## HIGH — will cause implementation inconsistencies

### H-1: `IterationResult.usage` struct undefined (providers.md:34)

Status: resolved on 2026-02-22.

Described only as "optional token/cost structure." No field definition. Every provider implementing the contract will invent a different shape, making the result non-portable.

---

### H-2: `events.jsonl` line schema not specified (PRD.md:99, ARCHITECTURE.md:22)

Status: resolved on 2026-02-22.

The file is referenced as a required persistence artifact but its per-line JSON structure is never defined. The normalized event model in providers.md is close but is not explicitly mapped to the on-disk format. At minimum, an example line is needed.

---

### H-3: `progress.md` format not specified

Status: resolved on 2026-02-22.

Referenced as a generated artifact in PRD.md, ARCHITECTURE.md, and cli.md but no template, structure, or content rules are defined.

---

### H-4: `prd.md` (human-authored) format not specified

Status: resolved on 2026-02-22.

The `prd.json` data model is documented. The companion `prd.md` is only ever described as "human-authored markdown." Unresolved questions:
- Is it free-form prose or a structured template?
- Is it the source that gets converted to JSON, or a generated companion?
- Does `daedalus validate` check it at all?

---

### H-5: Story terminal failure path not defined

Status: resolved on 2026-02-22.

PRD FR-002 defines states as `pending -> in_progress -> passed`. There is no `failed` state. ARCHITECTURE.md states "Keep `inProgress` sticky for safe recovery." If quality gates fail on all retries, it is not specified:
- What loop state results (error? stopped?).
- Whether the story remains `in_progress` indefinitely or transitions to a new state.
- How an operator resets a stuck story.

---

### H-6: Default provider `codex` is not implemented (`internal/providers/codex.go`)

Status: resolved on 2026-02-22.

The configured default provider is `codex`. Codex provider implementation is now available via local `codex exec` integration behind the provider contract.

---

### H-7: Quality gate pipeline (FR-004) has no implementation

Status: resolved on 2026-02-22.

FR-004 requires executing configured check commands before marking a story passed. ARCHITECTURE.md loop step 6 reads "Run quality commands." No quality service struct, interface, or invocation exists anywhere in `internal/`. Loop iterations can currently mark a story passed without executing any checks. Stories cannot be safely considered done until this is implemented.

---

### H-8: Git commit service (FR-005) has no implementation

Status: resolved on 2026-02-22.

FR-005 requires story-scoped commits after checks pass. ARCHITECTURE.md loop step 7 reads "Commit changes." No git service struct, interface, or invocation exists anywhere in `internal/`. The standardized commit format `feat(US-XXX): Story Title` is specified in the PRD and referenced in ARCHITECTURE.md but is never applied. Stories complete without a corresponding commit.

---

### H-9: Test coverage is effectively zero outside the Claude provider

Status: resolved on 2026-02-22.

`internal/providers/claude_test.go` contains 4 unit tests. No tests exist for the PRD service (store, validator, story transitions), config loading and validation, loop manager retry logic, provider registry, or project layout helpers. AGENTS.md defines `go test ./...` passing as a required Definition of Done criterion. Critical behavior — story state transitions, retry backoff, config validation — is entirely untested.

---

## MEDIUM — will cause friction, can be deferred

### M-1: `daedalus new [name] [context...]` — behavior when `context...` is provided

Status: resolved on 2026-02-22.

cli.md states `context...` is "reserved for future PRD generation flow." The current behavior is unspecified: silently ignored, warning printed, or error returned? This affects UX and testability of the `new` command.

---

### M-2: `daedalus edit [name]` — reserved command behavior

Status: resolved on 2026-02-22.

cli.md marks it "not implemented in v1 scaffold." DESIGN.md lists it as part of the v1 command surface without a caveat. Expected exit code and message when invoked should be specified.

---

### M-3: Version embedding strategy not documented

Status: resolved on 2026-02-22.

`daedalus version` is a v1 command. No spec exists for how the version string is set (build-time `ldflags`, embedded constant, generated file, etc.). AGENTS.md has no build step covering this.

---

### M-4: Worktree mode not specified

Status: resolved on 2026-02-22.

Mentioned in PRD FR-005, ARCHITECTURE.md concurrency model, and DESIGN.md settings, but there is no specification for how worktrees are created, named, managed, or torn down. Flagged as Phase 4 work but has no reference document for developers to design against.

---

### M-5: Claude and Gemini provider config fields undefined

Status: resolved on 2026-02-22.

`configuration.md` shows only `enabled = false` for planned providers. No stub field definitions exist for their eventual config shape. Developers planning the config parser have no guidance.

---

### M-6: Architecture phases vs PRD milestones count mismatch

Status: resolved on 2026-02-22.

ARCHITECTURE.md defines 5 implementation phases. PRD.md defines 4 milestones. The split of Claude/Gemini into Phase 5 is reasonable but is not reflected in the PRD milestones table, creating a discrepancy in the delivery plan.

---

### M-7: `internal/agent` package is orphaned legacy dead code

Status: resolved on 2026-02-22.

AGENTS.md explicitly states "`internal/agent` is legacy scaffold and should be phased out in favor of `internal/providers`." The package (`internal/agent/adapter.go`) still exists, compiles, and exports `ErrAdapterNotConfigured`, but is not imported or wired anywhere in the codebase. It should be deleted to eliminate confusion about which adapter abstraction is canonical.

---

## LOW / COSMETIC

### L-1: Commit format non-standard

Status: resolved on 2026-02-22.
Commit format was standardized to Conventional Commits: `feat(US-XXX): Story Title`.

---

### L-2: Event metadata keys not standardized

Status: resolved on 2026-02-22.

Normalized events carry `metadata: map[string]string` described as "optional provider detail." No well-known keys are listed. Even a non-exhaustive list of expected keys (e.g., `model`, `run_id`) would help implementors and log consumers.

---

### L-3: Config resolution layers not marked by implementation status

Status: resolved on 2026-02-22.

`configuration.md` documents CLI flags, environment variables, config file, and built-in defaults as the resolution priority. CLI overrides and environment overrides are both marked "planned." It is unclear which layers are implemented in the v1 scaffold and which are pending.

---

### L-4: Event channel from `RunIteration` not connected to any consumer

Status: resolved on 2026-02-22.

The `RunIteration` contract returns `<-chan Event` for streaming normalized events. No code currently reads from this channel to write `events.jsonl`, feed a TUI stream, or populate `agent.log`. The channel is returned but its consumer does not exist. Until a consumer is wired, all provider events are silently dropped and the persistence artifacts required by FR-007 are never written.

---

## What is solid and ready

- Provider interface contract (modulo B-3, H-1)
- PRD JSON data model and field definitions
- Story state machine (modulo H-5 on terminal failure)
- Directory layout and config file location
- Retry policy defaults and configuration schema
- Error category taxonomy and retry guidance in providers.md
- CLI command surface (including resolved v1 behavior notes)
- Security and concurrency model
- Test strategy and per-layer test requirements
- Non-functional requirements (NFRs) coverage

---

## Recommended actions before handoff

Priority order:

Resolved documentation alignment items:
1. B-1, B-2, B-3, B-4
2. H-1, H-2, H-3, H-4, H-5
3. H-6 implementation completed
4. M-1, M-2, M-3, M-4, M-5, M-6
5. L-1, L-2, L-3

Remaining implementation priorities:
1. Gemini provider module (planned).
2. TUI polish and hardening: richer live event streaming, stronger pause/stop lifecycle controls, and visual/interaction refinement.
