package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/EstebanForge/daedalus/internal/project"
)

type SetupResult struct {
	Path    string
	Branch  string
	Created bool
}

type Manager struct{}

func NewManager() Manager {
	return Manager{}
}

func (Manager) Ensure(ctx context.Context, baseDir, prdName string) (SetupResult, error) {
	name := strings.TrimSpace(prdName)
	if name == "" {
		return SetupResult{}, fmt.Errorf("PRD name is required for worktree mode")
	}

	if err := ensureGitRepo(ctx, baseDir); err != nil {
		return SetupResult{}, err
	}

	worktreePath := project.WorktreePath(baseDir, name)
	branch := "daedalus/" + name

	if info, err := os.Stat(worktreePath); err == nil {
		if !info.IsDir() {
			return SetupResult{}, fmt.Errorf("worktree path exists and is not a directory: %s", worktreePath)
		}

		managed, managedErr := isManagedWorktree(ctx, baseDir, worktreePath)
		if managedErr != nil {
			return SetupResult{}, managedErr
		}
		if !managed {
			return SetupResult{}, fmt.Errorf("worktree path exists and is not managed by daedalus: %s", worktreePath)
		}

		return SetupResult{Path: worktreePath, Branch: branch, Created: false}, nil
	} else if !os.IsNotExist(err) {
		return SetupResult{}, fmt.Errorf("failed to inspect worktree path %s: %w", worktreePath, err)
	}

	if err := os.MkdirAll(project.WorktreesPath(baseDir), 0o755); err != nil {
		return SetupResult{}, fmt.Errorf("failed to create worktrees root: %w", err)
	}

	exists, err := branchExists(ctx, baseDir, branch)
	if err != nil {
		return SetupResult{}, err
	}

	if exists {
		if err := runGit(ctx, baseDir, "worktree", "add", worktreePath, branch); err != nil {
			return SetupResult{}, err
		}
	} else {
		if err := runGit(ctx, baseDir, "worktree", "add", "-b", branch, worktreePath, "HEAD"); err != nil {
			return SetupResult{}, err
		}
	}

	return SetupResult{Path: worktreePath, Branch: branch, Created: true}, nil
}

func ensureGitRepo(ctx context.Context, baseDir string) error {
	out, err := gitOutput(ctx, baseDir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return fmt.Errorf("worktree mode requires a git repository: %w", err)
	}
	if strings.TrimSpace(out) != "true" {
		return fmt.Errorf("worktree mode requires a git repository")
	}
	return nil
}

func branchExists(ctx context.Context, baseDir, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if strings.TrimSpace(baseDir) != "" {
		cmd.Dir = baseDir
	}
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("failed to check branch %q: %w", branch, err)
}

func isManagedWorktree(ctx context.Context, baseDir, worktreePath string) (bool, error) {
	out, err := gitOutput(ctx, baseDir, "worktree", "list", "--porcelain")
	if err != nil {
		return false, err
	}

	target, err := canonicalPath(worktreePath)
	if err != nil {
		return false, fmt.Errorf("failed to resolve worktree path: %w", err)
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		candidate := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		candidateAbs, absErr := canonicalPath(candidate)
		if absErr != nil {
			continue
		}
		if candidateAbs == target {
			return true, nil
		}
	}
	return false, nil
}

func canonicalPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// Fallback to absolute path when symlink resolution is not available.
		return filepath.Clean(absPath), nil
	}
	return filepath.Clean(resolved), nil
}

func runGit(ctx context.Context, baseDir string, args ...string) error {
	_, err := gitOutput(ctx, baseDir, args...)
	return err
}

func gitOutput(ctx context.Context, baseDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if strings.TrimSpace(baseDir) != "" {
		cmd.Dir = baseDir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		text := strings.TrimSpace(stderr.String())
		if text == "" {
			text = strings.TrimSpace(stdout.String())
		}
		if text == "" {
			return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), text)
	}

	return stdout.String(), nil
}
