package project

import (
	"path/filepath"
	"testing"
)

func TestACPSessionsPath(t *testing.T) {
	t.Parallel()

	base := filepath.Join("tmp", "repo")
	got := ACPSessionsPath(base)
	want := filepath.Join(base, ".daedalus", "acp-sessions.json")
	if got != want {
		t.Fatalf("unexpected ACP sessions path: got %q want %q", got, want)
	}
}

func TestPRDPlansDir(t *testing.T) {
	t.Parallel()
	got := PRDPlansDir("/proj", "main")
	want := "/proj/.daedalus/prds/main/plans"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPRDPlanPath(t *testing.T) {
	t.Parallel()
	got := PRDPlanPath("/proj", "main", "US-42")
	want := "/proj/.daedalus/prds/main/plans/US-42.md"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPRDLearningsPath(t *testing.T) {
	t.Parallel()
	got := PRDLearningsPath("/proj", "main")
	want := "/proj/.daedalus/prds/main/learnings.md"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
