package app

import (
	"testing"

	"github.com/EstebanForge/daedalus/internal/prd"
)

func TestNextGeneratedPRDNameSkipsExisting(t *testing.T) {
	t.Parallel()

	summaries := []prd.Summary{
		{Name: "main"},
		{Name: "prd-1"},
		{Name: "PRD-2"},
	}

	got := nextGeneratedPRDName(summaries)
	if got != "prd-3" {
		t.Fatalf("expected prd-3, got %q", got)
	}
}

func TestTuiSelectedStoryClampsIndex(t *testing.T) {
	t.Parallel()

	doc := prd.Document{
		UserStories: []prd.UserStory{
			{ID: "US-001", Title: "One"},
			{ID: "US-002", Title: "Two"},
		},
	}

	story, index, storyID := tuiSelectedStory(doc, 99)
	if story == nil {
		t.Fatal("expected selected story, got nil")
	}
	if index != 1 {
		t.Fatalf("expected index 1, got %d", index)
	}
	if storyID != "US-002" {
		t.Fatalf("expected story id US-002, got %q", storyID)
	}

	story, index, storyID = tuiSelectedStory(doc, -10)
	if story == nil {
		t.Fatal("expected selected story, got nil")
	}
	if index != 0 {
		t.Fatalf("expected index 0, got %d", index)
	}
	if storyID != "US-001" {
		t.Fatalf("expected story id US-001, got %q", storyID)
	}
}
