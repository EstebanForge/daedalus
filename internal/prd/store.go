package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/EstebanForge/daedalus/internal/project"
)

type Summary struct {
	Name       string
	Total      int
	Complete   int
	InProgress int
}

type Store struct {
	baseDir string
}

func NewStore(baseDir string) Store {
	return Store{baseDir: baseDir}
}

func (s Store) Create(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}

	prdDir := project.PRDPath(s.baseDir, name)
	if _, err := os.Stat(prdDir); err == nil {
		return fmt.Errorf("PRD %q already exists", name)
	}

	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		return fmt.Errorf("failed to create PRD directory: %w", err)
	}

	markdown := defaultMarkdown(name)
	if err := os.WriteFile(project.PRDMarkdownPath(s.baseDir, name), []byte(markdown), 0o644); err != nil {
		return fmt.Errorf("failed to write prd.md: %w", err)
	}

	document := defaultJSON(name)
	if err := s.Save(name, document); err != nil {
		return err
	}

	progressFile := project.PRDProgressPath(s.baseDir, name)
	if err := os.WriteFile(progressFile, []byte("## Codebase Patterns\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write progress.md: %w", err)
	}

	return nil
}

func (s Store) Save(name string, doc Document) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PRD JSON: %w", err)
	}
	data = append(data, '\n')

	filePath := project.PRDJSONPath(s.baseDir, name)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write prd.json: %w", err)
	}
	return nil
}

func (s Store) Load(name string) (Document, error) {
	filePath := project.PRDJSONPath(s.baseDir, name)
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return Document{}, fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Document{}, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}
	return doc, nil
}

func (s Store) List() ([]Summary, error) {
	root := project.PRDsPath(s.baseDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read PRD root: %w", err)
	}

	summaries := make([]Summary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		jsonPath := project.PRDJSONPath(s.baseDir, name)
		if _, err := os.Stat(jsonPath); err != nil {
			continue
		}

		doc, err := s.Load(name)
		if err != nil {
			return nil, err
		}

		summaries = append(summaries, Summary{
			Name:       name,
			Total:      len(doc.UserStories),
			Complete:   doc.CountComplete(),
			InProgress: doc.CountInProgress(),
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	return summaries, nil
}

func (s Store) AutoDetectName() (string, error) {
	summaries, err := s.List()
	if err != nil {
		return "", err
	}
	if len(summaries) == 0 {
		return "", fmt.Errorf("no PRDs found; run 'daedalus new <name>' first")
	}
	if len(summaries) > 1 {
		return "", fmt.Errorf("multiple PRDs found; specify one explicitly")
	}
	return summaries[0].Name, nil
}

func (s Store) ResolveName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name != "" {
		return name, nil
	}
	return s.AutoDetectName()
}

func defaultJSON(name string) Document {
	return Document{
		Project:     strings.TrimSpace(name),
		Description: "Describe your project and then update user stories in prd.json.",
		UserStories: []UserStory{
			{
				ID:          "US-001",
				Title:       "Define first implementation story",
				Description: "As an operator, I want to define my first actionable story so that Daedalus can run a concrete iteration.",
				AcceptanceCriteria: []string{
					"Story has clear objective.",
					"Story has measurable acceptance criteria.",
				},
				Priority: 1,
				Passes:   false,
			},
		},
	}
}

func defaultMarkdown(name string) string {
	projectName := filepath.Base(name)
	return "# " + projectName + "\n\n" +
		"## Overview\n" +
		"Describe the project goals.\n\n" +
		"## User Stories\n\n" +
		"### US-001: Define first implementation story\n" +
		"**Priority:** 1\n" +
		"**Description:** As an operator, I want to define my first actionable story so that Daedalus can run a concrete iteration.\n\n" +
		"**Acceptance Criteria:**\n" +
		"- [ ] Story has clear objective.\n" +
		"- [ ] Story has measurable acceptance criteria.\n"
}
