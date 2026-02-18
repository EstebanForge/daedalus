package providers

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	"github.com/EstebanForge/daedalus/internal/config"
)

func TestClaudeProviderRunIterationBuildsCLIArgs(t *testing.T) {
	t.Parallel()

	var gotWorkDir string
	var gotArgs []string

	provider := claudeProvider{
		cfg: config.GenericProviderConfig{
			Model:          "claude-sonnet-4-6",
			ApprovalPolicy: "on-request",
			SandboxPolicy:  "workspace-write",
		},
		run: func(_ context.Context, workDir string, args []string) (string, error) {
			gotWorkDir = workDir
			gotArgs = append([]string(nil), args...)
			return "done", nil
		},
	}

	events, result, err := provider.RunIteration(context.Background(), IterationRequest{
		WorkDir: "/tmp/work",
		Prompt:  "Implement story S-1",
	})
	if err != nil {
		t.Fatalf("RunIteration returned error: %v", err)
	}

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected closed events channel")
		}
	default:
		t.Fatal("expected events channel to be closed")
	}

	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}
	if result.Summary != "done" {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}

	expectedArgs := []string{"-p", "--permission-mode", "acceptEdits", "--model", "claude-sonnet-4-6", "Implement story S-1"}
	if !reflect.DeepEqual(expectedArgs, gotArgs) {
		t.Fatalf("unexpected args: got %v want %v", gotArgs, expectedArgs)
	}
	if gotWorkDir != "/tmp/work" {
		t.Fatalf("unexpected work dir: %q", gotWorkDir)
	}
}

func TestClaudeProviderRunIterationRejectsUnsupportedApprovalPolicy(t *testing.T) {
	t.Parallel()

	provider := claudeProvider{
		cfg: config.GenericProviderConfig{
			ApprovalPolicy: "bad-value",
		},
	}

	_, _, err := provider.RunIteration(context.Background(), IterationRequest{
		Prompt: "hello",
	})
	if err == nil {
		t.Fatal("expected error for unsupported approval policy")
	}

	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Category != ErrorConfiguration {
		t.Fatalf("unexpected error category: %s", providerErr.Category)
	}
}

func TestMapClaudeErrorNotFound(t *testing.T) {
	t.Parallel()

	err := mapClaudeError(exec.ErrNotFound)
	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Category != ErrorConfiguration {
		t.Fatalf("unexpected error category: %s", providerErr.Category)
	}
}

func TestMapClaudeErrorRateLimit(t *testing.T) {
	t.Parallel()

	err := mapClaudeError(errors.New("API returned 429 rate limit exceeded"))
	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Category != ErrorRateLimit {
		t.Fatalf("unexpected error category: %s", providerErr.Category)
	}
}
