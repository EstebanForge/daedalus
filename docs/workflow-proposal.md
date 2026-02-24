# Proposed Workflow: From Idea to Implementation

## Overview

This document defines the recommended workflow for Daedalus — from initial concept through implementation. It is designed to provide clear context for both humans and LLM agents collaborating on the project.

---

## Workflow Types

Daedalus has two connected workflows:
- Runtime onboarding workflow (conditional): runs when `.daedalus/` is missing or onboarding is incomplete.
- Product planning and delivery workflow: JTBD → PRD → Architecture & Design → ADRs → Implementation.

## The Workflow

```
If .daedalus/ is missing:
First-Run Setup → JTBD → PRD → Architecture & Design → ADRs → Implementation

If .daedalus/ exists and onboarding is complete:
JTBD → PRD → Architecture & Design → ADRs → Implementation
```

### 1. First-Run Setup Experience (Onboarding Screens)

**Purpose:** Reduce first-run friction and collect required setup context before planning.

**When shown:** On launch when `.daedalus/` does not exist, or when onboarding state is incomplete.

**Completion rule:** Onboarding is mandatory and resumable. If setup is interrupted or incomplete, Daedalus re-runs onboarding on next launch at the first incomplete step until all required steps are completed.

**Project mode detection:**
- Existing project mode: current directory contains any file or folder other than `.daedalus/`.
- Empty folder mode: no files/folders exist other than `.daedalus/` (or folder is fully empty).

**Required screens:**

#### 1.1 Git Ignore Setup
- Prompt: add `.daedalus/` to `.gitignore`.
- Goal: prevent local runtime/planning artifacts from being committed unintentionally.
- Behavior: explicit Yes/No choice; default recommendation may be shown.
- Screen content must include a short `pros`, `cons`, and `use cases` summary for both options.
- Option A: ignore `.daedalus/` (recommended for local-only planning, cleaner commits, less repo noise).
- Option B: keep `.daedalus/` tracked (useful for shared team planning artifacts and auditability in-repo).

#### 1.2 Existing Project Discovery (existing project mode only)
- Prompt user for a short plain-language description of the project.
- Run an agent-driven repository scan in the background using Agents CLI prompts.
- Scan behavior requirements:
- Read-only analysis only (no repository mutation).
- Show clear UI progress while running (spinner/loader + status text).
- Return a structured summary covering purpose, architecture, stack, key modules, test/lint commands, and active risks.
- If scan fails, show actionable error and allow retry without losing prior onboarding inputs.

#### 1.3 JTBD Capture
- Empty folder mode: user writes JTBD in a text area.
- Existing project mode: Daedalus generates a draft JTBD from user description + repository summary, then user reviews/edits.
- Guidance format:
- `When [situation], I want to [motivation], so I can [expected outcome].`

#### 1.4 Create First PRD
- Prompt for initial PRD name.
- Default: `main`.
- Show target location preview: `.daedalus/prds/<name>/`.

**Auto-seeded docs when existing project mode is used:**
- JTBD document is pre-filled from the scan + user input.
- Architecture & Design document is pre-filled from the scan summary.
- PRD context is seeded from the approved JTBD plus project summary context.

**Defaults outside onboarding:**
- Post-completion automation behavior is configured through defaults and flags, not onboarding screens.
- Sensible defaults: push disabled, auto-PR disabled.
- Configuration/flags should expose these controls directly (for example `completion.push_on_complete`, `completion.auto_pr_on_complete`, plus matching CLI flags).

**Output:** Initial runtime setup plus planning context artifacts needed to start feature/bugfix work.

---

### 2. Jobs-to-be-Done (JTBD)

**Purpose:** Define the problem. Understand what job the product is "hired" to do.

**Output:** A single statement in the format:
```
When [situation], I want to [motivation], so I can [expected outcome].
```

**Example for Daedalus:**
> "When I need to ship a feature to production, I want an AI agent to execute it autonomously with quality gates, so I can deliver faster without manual oversight."

**Why second:** This focuses the project on the actual outcome after setup context is established.

---

### 3. Product Requirements Document (PRD)

**Purpose:** Define what we are building. The contract between stakeholders and engineers.

**Output:** PRD document containing:
- Problem statement (from JTBD)
- Goals and non-goals
- User personas
- Functional requirements
- Non-functional requirements
- User stories
- Acceptance criteria
- Out of scope

**Why third:** You cannot design a solution without knowing what you are building.

---

### 4. Architecture & Design Document

**Purpose:** Define how the system will work. Technical blueprint.

**Output:** A single Architecture & Design document (canonical path: `docs/ARCHITECTURE.md`) containing:

#### 4.1 High-Level Architecture
- System boundaries and components
- Layer separation
- Integration points
- Data flow diagrams

#### 4.2 Component Design
- CLI Interface specification
- PRD Service
- Loop Manager
- Quality Gates
- Provider Adapters (Codex, Claude, Gemini, OpenCode, Copilot, Qwen Code, Pi)
- Configuration Model

#### 4.3 API Contracts
- Internal function signatures
- Data structures
- Error handling

#### 4.4 Design Decisions
- Link to relevant ADRs (Architecture Decision Records)
- Trade-offs documented
- Alternatives considered

**Why fourth:** Design is derived from requirements. You must know *what* before you can decide *how*.

---

### 5. Architecture Decision Records (ADRs)

**Purpose:** Document specific architectural decisions with context, rationale, and consequences.

**Output:** Series of markdown files, one per decision.

**ADR Format:**
```markdown
# ADR-N: [Decision Title]

## Status: [Proposed | Accepted | Deprecated | Superseded]

## Context
[Describe the issue or question driving this decision]

## Decision
[What was decided]

## Consequences
- Positive: [List benefits]
- Negative: [List drawbacks]
- Neutral: [Other impacts]

## Related
[Links to related ADRs or documents]
```

**When to write:** Throughout the Architecture & Design phase. Each significant decision gets an ADR.

---

### 6. Implementation

**Purpose:** Write the code.

**Input:** JTBD + PRD + Architecture & Design + ADRs (+ onboarding-generated project context when available).

**Process:**
- LLM agent uses PRD + Architecture & Design as context.
- ADRs provide decision history for difficult trade-offs.
- Quality gates are enforced via the loop mechanism.

---

## Summary Diagrams

Onboarding workflow (runtime, conditional):

```
Missing .daedalus/ OR onboarding incomplete?
  ├─ No  -> Skip onboarding
  └─ Yes -> 1.1 Git Ignore
            -> 1.2 Existing Project Discovery (only if files/folders other than .daedalus/ exist)
            -> 1.3 JTBD Capture/Review
            -> 1.4 Create First PRD
            -> Mark onboarding complete
```

Planning and delivery workflow:

```
JTBD -> PRD -> Architecture & Design -> ADRs -> Implementation
```

---

## Acronyms Reference

| Acronym | Full Name | Purpose |
|---------|-----------|---------|
| JTBD | Jobs-to-be-Done | Requirements framework focusing on user motivations |
| PRD | Product Requirements Document | Definition of what to build |
| ADR | Architecture Decision Record | Document for specific technical decisions |
| API | Application Programming Interface | How components communicate |
| CLI | Command-Line Interface | Text-based user interface |

---

## For LLM Agents

When working on Daedalus:

1. **If `.daedalus/` is missing or onboarding is incomplete, finish onboarding first.**
2. **For existing projects, collect user project description and run the repository scan before finalizing JTBD.**
3. **For empty folders, capture JTBD directly from user input.**
4. **Reference the PRD for requirements; do not implement features outside scope.**
5. **Use Architecture & Design + ADRs to understand decisions before changing implementation.**

---

## Document History

| Version | Date | Notes |
|---------|------|-------|
| 1.0 | 2026-02-24 | Initial proposal |
| 1.1 | 2026-02-24 | Added first-run setup/onboarding screens to workflow |
| 1.2 | 2026-02-24 | Changed onboarding screen 2 to JTBD text capture and made PRD creation JTBD-seeded |
| 1.3 | 2026-02-24 | Added existing-project discovery screen with agent scan and repository-summary doc seeding |
| 1.4 | 2026-02-24 | Made onboarding conditional-first, added completion-resume rules, defined existing-project detection and scan contract, and moved post-completion options to config defaults/flags |
| 1.5 | 2026-02-24 | Standardized to a single canonical Architecture & Design document (`docs/ARCHITECTURE.md`) |

---

*This workflow is designed for Daedalus.*
