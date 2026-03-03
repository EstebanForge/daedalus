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

func TestPushBranchPushesToBareRemote(t *testing.T) {
	t.Parallel()

	bareDir := t.TempDir()
	run(t, bareDir, "git", "init", "--bare")

	workDir := initRepo(t)
	writeFile(t, filepath.Join(workDir, "init.txt"), "init\n")
	run(t, workDir, "git", "add", "-A")
	run(t, workDir, "git", "commit", "-m", "initial commit")
	run(t, workDir, "git", "remote", "add", "origin", bareDir)

	committer := NewCommitter()
	if err := committer.PushBranch(context.Background(), workDir); err != nil {
		t.Fatalf("push branch: %v", err)
	}
}

func TestPushBranchFailsWithNoRemote(t *testing.T) {
	t.Parallel()

	workDir := initRepo(t)
	writeFile(t, filepath.Join(workDir, "file.txt"), "content\n")
	run(t, workDir, "git", "add", "-A")
	run(t, workDir, "git", "commit", "-m", "initial commit")

	committer := NewCommitter()
	if err := committer.PushBranch(context.Background(), workDir); err == nil {
		t.Fatal("expected error when no remote configured")
	}
}

func TestCreatePRFailsWhenGhNotFound(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("PATH", "")

	committer := NewCommitter()
	err := committer.CreatePR(context.Background(), workDir)
	if err == nil {
		t.Fatal("expected error when gh not in PATH")
	}
	if err.Error() == "" {
		t.Fatal("expected non-empty error message")
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
