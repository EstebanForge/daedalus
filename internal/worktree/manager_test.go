package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCreatesWorktreeAndBranch(t *testing.T) {
	t.Parallel()

	baseDir := initGitRepo(t)
	manager := NewManager()

	result, err := manager.Ensure(context.Background(), baseDir, "main")
	if err != nil {
		t.Fatalf("ensure worktree: %v", err)
	}
	if !result.Created {
		t.Fatal("expected created=true on first setup")
	}
	if !strings.HasSuffix(filepath.Clean(result.Path), filepath.Join(".daedalus", "worktrees", "main")) {
		t.Fatalf("unexpected worktree path: %s", result.Path)
	}
	if result.Branch != "daedalus/main" {
		t.Fatalf("unexpected branch: %s", result.Branch)
	}
}

func TestEnsureReusesManagedWorktree(t *testing.T) {
	t.Parallel()

	baseDir := initGitRepo(t)
	manager := NewManager()

	first, err := manager.Ensure(context.Background(), baseDir, "main")
	if err != nil {
		t.Fatalf("ensure first: %v", err)
	}
	second, err := manager.Ensure(context.Background(), baseDir, "main")
	if err != nil {
		t.Fatalf("ensure second: %v", err)
	}
	if !first.Created {
		t.Fatal("expected first ensure to create worktree")
	}
	if second.Created {
		t.Fatal("expected second ensure to reuse existing worktree")
	}
	if first.Path != second.Path {
		t.Fatalf("expected same path, first=%s second=%s", first.Path, second.Path)
	}
}

func TestEnsureFailsWhenPathExistsButNotManaged(t *testing.T) {
	t.Parallel()

	baseDir := initGitRepo(t)
	path := filepath.Join(baseDir, ".daedalus", "worktrees", "main")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "README.txt"), []byte("not a git worktree"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	manager := NewManager()
	_, err := manager.Ensure(context.Background(), baseDir, "main")
	if err == nil {
		t.Fatal("expected error for unmanaged existing path")
	}
	if !strings.Contains(err.Error(), "not managed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	baseDir := t.TempDir()
	runGitCmd(t, baseDir, "init")
	runGitCmd(t, baseDir, "config", "user.email", "test@example.com")
	runGitCmd(t, baseDir, "config", "user.name", "Daedalus Test")
	if err := os.WriteFile(filepath.Join(baseDir, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	runGitCmd(t, baseDir, "add", "README.md")
	runGitCmd(t, baseDir, "commit", "-m", "init")
	return baseDir
}

func runGitCmd(t *testing.T, baseDir string, args ...string) {
	t.Helper()
	_, err := gitOutput(context.Background(), baseDir, args...)
	if err != nil {
		t.Fatalf("git %s failed: %v", strings.Join(args, " "), err)
	}
}
