package templates_test

import (
	"strings"
	"testing"

	"github.com/EstebanForge/daedalus/internal/templates"
)

// requiredSections checks that every expected heading is present in the
// template content exactly once.
func requiredSections(t *testing.T, name, content string, headings []string) {
	t.Helper()
	for _, h := range headings {
		count := strings.Count(content, h)
		if count == 0 {
			t.Errorf("%s: required heading %q not found", name, h)
		} else if count > 1 {
			t.Errorf("%s: heading %q appears %d times (want exactly 1)", name, h, count)
		}
	}
}

func TestProjectSummaryTemplate(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(templates.ProjectSummary) == "" {
		t.Fatal("ProjectSummary template is empty")
	}
	requiredSections(t, "ProjectSummary", templates.ProjectSummary, []string{
		"# Project Summary",
		"## Purpose",
		"## Architecture",
		"## Tech Stack",
		"## Key Modules",
		"## Test and Lint Commands",
		"## Active Risks",
	})
}

func TestJTBDTemplate(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(templates.JTBD) == "" {
		t.Fatal("JTBD template is empty")
	}
	requiredSections(t, "JTBD", templates.JTBD, []string{
		"# Jobs-to-be-Done",
		"## Primary Job",
		"## Context",
		"## Constraints",
	})
	// Primary Job section must contain the canonical JTBD format hint.
	if !strings.Contains(templates.JTBD, "When [situation]") {
		t.Error("JTBD template: Primary Job section missing canonical format hint")
	}
}

func TestArchitectureDesignTemplate(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(templates.ArchitectureDesign) == "" {
		t.Fatal("ArchitectureDesign template is empty")
	}
	requiredSections(t, "ArchitectureDesign", templates.ArchitectureDesign, []string{
		"# Architecture & Design",
		"## Overview",
		"## Components",
		"## Data Flow",
		"## Key Decisions",
		"## Open Questions",
	})
}

func TestPRDTemplate(t *testing.T) {
	t.Parallel()
	if strings.TrimSpace(templates.PRD) == "" {
		t.Fatal("PRD template is empty")
	}
	requiredSections(t, "PRD", templates.PRD, []string{
		"# PRD:",
		"## Problem Statement",
		"## Goals",
		"## Non-Goals",
		"## User Stories",
		"## Risks and Mitigations",
	})
	// The [Project Name] placeholder must be present so callers can substitute it.
	if !strings.Contains(templates.PRD, "[Project Name]") {
		t.Error("PRD template: [Project Name] placeholder not found")
	}
}

// TestProjectSummaryPromptStructure verifies that the scan prompt embeds the
// template so the LLM receives the exact section headings it must produce.
func TestProjectSummaryTemplateHasPlaceholders(t *testing.T) {
	t.Parallel()
	// Every section must have a [...] placeholder so the LLM knows what to fill.
	sections := []string{
		"## Purpose",
		"## Architecture",
		"## Tech Stack",
		"## Key Modules",
		"## Test and Lint Commands",
		"## Active Risks",
	}
	for _, section := range sections {
		idx := strings.Index(templates.ProjectSummary, section)
		if idx < 0 {
			t.Errorf("section %q not found", section)
			continue
		}
		// The content after the heading (up to the next heading) must contain a placeholder.
		after := templates.ProjectSummary[idx+len(section):]
		nextHeading := strings.Index(after, "\n## ")
		var sectionBody string
		if nextHeading >= 0 {
			sectionBody = after[:nextHeading]
		} else {
			sectionBody = after
		}
		if !strings.Contains(sectionBody, "[") || !strings.Contains(sectionBody, "]") {
			t.Errorf("section %q is missing a [...] placeholder in its body", section)
		}
	}
}

// TestAllTemplatesStartWithH1 verifies each template begins with a level-1 heading.
func TestAllTemplatesStartWithH1(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
	}{
		{"ProjectSummary", templates.ProjectSummary},
		{"JTBD", templates.JTBD},
		{"ArchitectureDesign", templates.ArchitectureDesign},
		{"PRD", templates.PRD},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			trimmed := strings.TrimSpace(c.content)
			if !strings.HasPrefix(trimmed, "# ") {
				t.Errorf("%s: expected content to start with '# ', got: %.40q", c.name, trimmed)
			}
		})
	}
}
