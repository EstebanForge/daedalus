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
