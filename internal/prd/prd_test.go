package prd

import (
	"os"
	"testing"

	"github.com/EstebanForge/daedalus/internal/project"
)

func TestValidateDetectsDuplicatePriority(t *testing.T) {
	t.Parallel()

	doc := Document{
		Project: "demo",
		UserStories: []UserStory{
			{
				ID:                 "US-001",
				Title:              "One",
				Description:        "first",
				AcceptanceCriteria: []string{"a"},
				Priority:           1,
			},
			{
				ID:                 "US-002",
				Title:              "Two",
				Description:        "second",
				AcceptanceCriteria: []string{"b"},
				Priority:           1,
			},
		},
	}

	result := Validate(doc)
	if result.Valid() {
		t.Fatal("expected invalid document")
	}
}

func TestNextStoryPrefersInProgressStory(t *testing.T) {
	t.Parallel()

	doc := Document{
		Project: "demo",
		UserStories: []UserStory{
			{ID: "US-001", Title: "one", Priority: 1, Passes: false},
			{ID: "US-002", Title: "two", Priority: 2, InProgress: true, Passes: false},
		},
	}

	next := doc.NextStory()
	if next == nil || next.ID != "US-002" {
		t.Fatalf("expected US-002, got %+v", next)
	}
}

func TestStoreCreateCreatesRuntimeArtifacts(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := NewStore(baseDir)

	if err := store.Create("main"); err != nil {
		t.Fatalf("create: %v", err)
	}

	paths := []string{
		project.PRDMarkdownPath(baseDir, "main"),
		project.PRDJSONPath(baseDir, "main"),
		project.PRDProgressPath(baseDir, "main"),
		project.PRDAgentLogPath(baseDir, "main"),
		project.PRDEventsPath(baseDir, "main"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s to exist: %v", path, err)
		}
	}
}
