package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type CommitResult struct {
	Committed bool
	CommitSHA string
	Message   string
}

type Committer struct{}

func NewCommitter() Committer {
	return Committer{}
}

func (Committer) CommitStory(ctx context.Context, workDir, storyID, storyTitle string) (CommitResult, error) {
	dirty, err := hasChanges(ctx, workDir)
	if err != nil {
		return CommitResult{}, err
	}
	if !dirty {
		return CommitResult{Committed: false, Message: "no changes to commit"}, nil
	}

	message := fmt.Sprintf("feat(%s): %s", strings.TrimSpace(storyID), strings.TrimSpace(storyTitle))
	if err := runGit(ctx, workDir, "add", "-A"); err != nil {
		return CommitResult{}, err
	}
	if err := runGit(ctx, workDir, "commit", "-m", message); err != nil {
		return CommitResult{}, err
	}

	sha, err := gitOutput(ctx, workDir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return CommitResult{}, err
	}

	return CommitResult{
		Committed: true,
		CommitSHA: strings.TrimSpace(sha),
		Message:   message,
	}, nil
}

func hasChanges(ctx context.Context, workDir string) (bool, error) {
	out, err := gitOutput(ctx, workDir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func runGit(ctx context.Context, workDir string, args ...string) error {
	_, err := gitOutput(ctx, workDir, args...)
	return err
}

func gitOutput(ctx context.Context, workDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if strings.TrimSpace(workDir) != "" {
		cmd.Dir = workDir
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
