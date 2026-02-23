package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCommitStoryCommitsWhenRepoHasChanges(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "hello.txt"), "hello\n")

	committer := NewCommitter()
	result, err := committer.CommitStory(context.Background(), repo, "US-101", "Add hello file")
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	if !result.Committed {
		t.Fatal("expected commit to be created")
	}
	if result.CommitSHA == "" {
		t.Fatal("expected commit sha")
	}
}

func TestCommitStorySkipsWhenNoChanges(t *testing.T) {
	t.Parallel()

	repo := initRepo(t)
	committer := NewCommitter()
	result, err := committer.CommitStory(context.Background(), repo, "US-102", "No-op")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Committed {
		t.Fatal("expected no commit when repository is clean")
	}
}

func initRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "dev@example.com")
	run(t, dir, "git", "config", "user.name", "Dev User")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v (%s)", name, args, err, string(output))
	}
}
