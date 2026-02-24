// Package templates embeds the canonical document templates used when
// generating or prompting for structured planning artifacts.
//
// Every place in the codebase that produces a project-summary, jtbd,
// architecture-design, or prd document must derive its structure from
// these templates so the output is deterministic across all seven
// provider backends.
package templates

import _ "embed"

// ProjectSummary is the canonical template for the repository scan output
// produced during onboarding. LLMs are instructed to fill in the template
// exactly, replacing each [...] placeholder with actual content and keeping
// all section headings unchanged.
//
//go:embed project-summary.md
var ProjectSummary string

// JTBD is the canonical template for the Jobs-to-be-Done document.
// It is used as the initial content when creating jtbd.md, either as a
// blank stub (empty-folder mode) or pre-filled with source material
// derived from the project description and scan summary (existing-project
// mode).
//
//go:embed jtbd.md
var JTBD string

// ArchitectureDesign is the canonical template for the Architecture & Design
// context document. It is used as the initial content when creating
// architecture-design.md, optionally seeded with the repository scan summary.
//
//go:embed architecture-design.md
var ArchitectureDesign string

// PRD is the canonical template for the prd.md narrative document created
// alongside every new prd.json. The placeholder [Project Name] is replaced
// with the actual PRD name at creation time.
//
//go:embed prd.md
var PRD string
