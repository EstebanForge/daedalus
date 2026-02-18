# Documentation Review — Daedalus

Reviewed: 2026-02-17
Scope: `docs/PRD.md`, `docs/ARCHITECTURE.md`, `docs/DESIGN.md`, `docs/reference/cli.md`, `docs/reference/configuration.md`, `docs/reference/providers.md`

---

## Overall Assessment

The documentation is well-structured and internally consistent. The architecture, provider contract, and data model are solid foundations. The following issues prevent a clean handoff to developers. They are categorized by severity.

---

## BLOCKERS — resolve before development starts

### B-1: Three open design decisions (DESIGN.md:99-103)

These are explicitly listed as "open" and each one directly affects implementation scope:

**Headless vs TUI for `run`**
Does `daedalus run` operate fully headless or does it share/spawn the TUI runtime? This changes the entire `run` command implementation and how the event bus is wired.

**Manual story selection in v1**
No decision recorded. Affects the CLI API (`run [name] --story <id>`?), the PRD service `getNextStory` contract, and the loop manager.

**Auto-commit vs operator confirmation**
No decision recorded. Affects the git service flow, TUI controls, and the loop step sequence in ARCHITECTURE.md.

---

### B-2: Prompt construction not specified (ARCHITECTURE.md loop step 4)

The loop step reads "Build prompt/context" with no further detail. There is no specification for:
- What the prompt sent to the provider contains (story title? description? acceptance criteria? surrounding PRD context?).
- Which files populate `contextFiles` in `IterationRequest` and how they are selected.

This is the primary input to every provider adapter. Without a spec, each developer will invent a different shape.

---

### B-3: `RunIteration` return semantics ambiguous (providers.md:14, ARCHITECTURE.md:74)

The signature `RunIteration(ctx, request) -> (<-chan Event, IterationResult, error)` returns three values simultaneously but the semantics are never specified:
- Is `error` only for pre-start failures (bad config, can't launch), while in-flight failures arrive via the channel?
- Is `IterationResult` only valid after the channel is closed?
- What is the channel's behavior when `ctx` is cancelled — closed immediately? Drained first?

The test requirements reference "cancellation and graceful shutdown behavior" but define no expected behavior.

---

### B-4: `approval_policy` and `sandbox_policy` valid values not defined (configuration.md)

The example config shows `approval_policy = "on-failure"` and `sandbox_policy = "workspace-write"` but neither field lists its full set of valid values, behavior per value, or what happens on an invalid input. These values are passed through to provider adapters. Developers implementing provider modules have no contract to code against.

---

## HIGH — will cause implementation inconsistencies

### H-1: `IterationResult.usage` struct undefined (providers.md:34)

Described only as "optional token/cost structure." No field definition. Every provider implementing the contract will invent a different shape, making the result non-portable.

---

### H-2: `events.jsonl` line schema not specified (PRD.md:99, ARCHITECTURE.md:22)

The file is referenced as a required persistence artifact but its per-line JSON structure is never defined. The normalized event model in providers.md is close but is not explicitly mapped to the on-disk format. At minimum, an example line is needed.

---

### H-3: `progress.md` format not specified

Referenced as a generated artifact in PRD.md, ARCHITECTURE.md, and cli.md but no template, structure, or content rules are defined.

---

### H-4: `prd.md` (human-authored) format not specified

The `prd.json` data model is documented. The companion `prd.md` is only ever described as "human-authored markdown." Unresolved questions:
- Is it free-form prose or a structured template?
- Is it the source that gets converted to JSON, or a generated companion?
- Does `daedalus validate` check it at all?

---

### H-5: Story terminal failure path not defined

PRD FR-002 defines states as `pending -> in_progress -> passed`. There is no `failed` state. ARCHITECTURE.md states "Keep `inProgress` sticky for safe recovery." If quality gates fail on all retries, it is not specified:
- What loop state results (error? stopped?).
- Whether the story remains `in_progress` indefinitely or transitions to a new state.
- How an operator resets a stuck story.

---

## MEDIUM — will cause friction, can be deferred

### M-1: `daedalus new [name] [context...]` — behavior when `context...` is provided

cli.md states `context...` is "reserved for future PRD generation flow." The current behavior is unspecified: silently ignored, warning printed, or error returned? This affects UX and testability of the `new` command.

---

### M-2: `daedalus edit [name]` — reserved command behavior

cli.md marks it "not implemented in v1 scaffold." DESIGN.md lists it as part of the v1 command surface without a caveat. Expected exit code and message when invoked should be specified.

---

### M-3: Version embedding strategy not documented

`daedalus version` is a v1 command. No spec exists for how the version string is set (build-time `ldflags`, embedded constant, generated file, etc.). AGENTS.md has no build step covering this.

---

### M-4: Worktree mode not specified

Mentioned in PRD FR-005, ARCHITECTURE.md concurrency model, and DESIGN.md settings, but there is no specification for how worktrees are created, named, managed, or torn down. Flagged as Phase 4 work but has no reference document for developers to design against.

---

### M-5: Claude and Gemini provider config fields undefined

`configuration.md` shows only `enabled = false` for planned providers. No stub field definitions exist for their eventual config shape. Developers planning the config parser have no guidance.

---

### M-6: Architecture phases vs PRD milestones count mismatch

ARCHITECTURE.md defines 5 implementation phases. PRD.md defines 4 milestones. The split of Claude/Gemini into Phase 5 is reasonable but is not reflected in the PRD milestones table, creating a discrepancy in the delivery plan.

---

## LOW / COSMETIC

### L-1: Commit format non-standard

PRD FR-005 specifies `feat: [US-XXX] - [Story Title]`. Standard Conventional Commits format would be `feat(US-XXX): Story Title`. Confirm whether the current format is intentional.

---

### L-2: Event metadata keys not standardized

Normalized events carry `metadata: map[string]string` described as "optional provider detail." No well-known keys are listed. Even a non-exhaustive list of expected keys (e.g., `model`, `run_id`) would help implementors and log consumers.

---

### L-3: Config resolution layers not marked by implementation status

`configuration.md` documents CLI flags, environment variables, config file, and built-in defaults as the resolution priority. CLI overrides and environment overrides are both marked "planned." It is unclear which layers are implemented in the v1 scaffold and which are pending.

---

## What is solid and ready

- Provider interface contract (modulo B-3, H-1)
- PRD JSON data model and field definitions
- Story state machine (modulo H-5 on terminal failure)
- Directory layout and config file location
- Retry policy defaults and configuration schema
- Error category taxonomy and retry guidance in providers.md
- CLI command surface (modulo B-1 open decisions)
- Security and concurrency model
- Test strategy and per-layer test requirements
- Non-functional requirements (NFRs) coverage

---

## Recommended actions before handoff

Priority order:

1. Resolve the 3 open design decisions (B-1).
2. Write a prompt construction spec covering prompt content and `contextFiles` selection (B-2).
3. Clarify `RunIteration` return semantics: pre-start vs in-flight errors, channel close conditions, and cancellation behavior (B-3).
4. Define valid values and behavior for `approval_policy` and `sandbox_policy` (B-4).
5. Define `IterationResult.usage` field structure (H-1).
6. Add `events.jsonl` per-line JSON schema with a minimal example (H-2).
7. Add `progress.md` format spec or template (H-3).
8. Clarify `prd.md` role: template vs free-form, source vs generated (H-4).
9. Define the story terminal failure path and operator reset mechanism (H-5).
