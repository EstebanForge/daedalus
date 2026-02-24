# Proposed Workflow: From Idea to Implementation

## Overview

This document defines the recommended workflow for Daedalus — from initial concept through implementation. It is designed to provide clear context for both humans and LLM agents collaborating on the project.

---

## The Workflow

```
JTBD → PRD → Architecture & Design → Implementation
```

### 1. Jobs-to-be-Done (JTBD)

**Purpose:** Define the problem. Understand what job the product is "hired" to do.

**Output:** A single statement in the format:
```
When [situation], I want to [motivation], so I can [expected outcome].
```

**Example for Daedalus:**
> "When I need to ship a feature to production, I want an AI agent to execute it autonomously with quality gates, so I can deliver faster without manual oversight."

**Why first:** This focuses the entire project on solving a real problem, not just building features.

---

### 2. Product Requirements Document (PRD)

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

**Why second:** You cannot design a solution without knowing what you are building.

---

### 3. Architecture & Design Document

**Purpose:** Define how the system will work. Technical blueprint.

**Output:** Architecture & Design document containing:

#### 3.1 High-Level Architecture
- System boundaries and components
- Layer separation
- Integration points
- Data flow diagrams

#### 3.2 Component Design
- CLI Interface specification
- PRD Service
- Loop Manager
- Quality Gates
- Provider Adapters (Codex, Claude, Gemini, OpenCode, Copilot, Qwen Code, Pi)
- Configuration Model

#### 3.3 API Contracts
- Internal function signatures
- Data structures
- Error handling

#### 3.4 Design Decisions
- Link to relevant ADRs (Architecture Decision Records)
- Trade-offs documented
- Alternatives considered

**Why third:** Design is derived from requirements. You must know *what* before you can decide *how*.

---

### 4. Architecture Decision Records (ADRs)

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

### 5. Implementation

**Purpose:** Write the code.

**Input:** All of the above documents.

**Process:**
- LLM agent uses PRD + Architecture & Design as context
- ADRs provide decision history for difficult trade-offs
- Quality gates enforced via the loop mechanism

---

## Summary Diagram

```
┌─────────────────────────────────────────────────────────────┐
│  1. JTBD (5 min)                                          │
│     └─> Job Statement                                      │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  2. PRD                                                    │
│     └─> Requirements, User Stories, Acceptance Criteria   │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  3. Architecture & Design                                   │
│     ├─> High-Level Architecture                            │
│     ├─> Component Design                                   │
│     └─> API Contracts                                      │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  4. ADRs (continuous)                                      │
│     └─> Decision records                                   │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  5. Implementation                                         │
│     └─> Code                                              │
└─────────────────────────────────────────────────────────────┘
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

1. **Always start with the JTBD** — it defines the scope
2. **Reference the PRD** for requirements — do not implement features outside scope
3. **Use Architecture & Design** for implementation guidance
4. **Check ADRs** for decision context — understand *why* before changing

---

## Document History

| Version | Date | Notes |
|---------|------|-------|
| 1.0 | 2026-02-24 | Initial proposal |

---

*This workflow is designed for Daedalus.*
