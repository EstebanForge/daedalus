package providers

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/EstebanForge/daedalus/internal/config"
)

func TestCodexProviderRunIterationBuildsCLIArgs(t *testing.T) {
	t.Parallel()

	var gotWorkDir string
	var gotArgs []string

	provider := codexProvider{
		cfg: config.GenericProviderConfig{
			Model:          "gpt-5-codex",
			ApprovalPolicy: "on-request",
			SandboxPolicy:  "workspace-write",
		},
		run: func(_ context.Context, workDir string, args []string, _ chan Event) (string, error) {
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

	gotEvents := collectEvents(events)
	wantEventTypes := []EventType{
		EventIterationStarted,
		EventCommandOutput,
		EventAssistantText,
		EventIterationDone,
	}
	if !reflect.DeepEqual(wantEventTypes, eventTypes(gotEvents)) {
		t.Fatalf("unexpected event sequence: got %v want %v", eventTypes(gotEvents), wantEventTypes)
	}

	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}
	if gotWorkDir != "/tmp/work" {
		t.Fatalf("unexpected work dir: %q", gotWorkDir)
	}
	if len(gotArgs) < 9 {
		t.Fatalf("unexpected args length: %v", gotArgs)
	}
	wantPrefix := []string{"exec", "--sandbox", "workspace-write", "--skip-git-repo-check", "--output-last-message"}
	if !reflect.DeepEqual(wantPrefix, gotArgs[:5]) {
		t.Fatalf("unexpected args prefix: got %v want %v", gotArgs[:5], wantPrefix)
	}
	if gotArgs[6] != "--model" || gotArgs[7] != "gpt-5-codex" {
		t.Fatalf("expected model flags, got %v", gotArgs)
	}
	if gotArgs[len(gotArgs)-1] != "Implement story S-1" {
		t.Fatalf("unexpected prompt arg: %v", gotArgs)
	}
}

func TestCodexProviderRunIterationRejectsUnsupportedApprovalPolicy(t *testing.T) {
	t.Parallel()

	provider := codexProvider{
		cfg: config.GenericProviderConfig{
			ApprovalPolicy: "invalid",
		},
	}

	_, _, err := provider.RunIteration(context.Background(), IterationRequest{
		Prompt: "hello",
	})
	if err == nil {
		t.Fatal("expected error for unsupported approval policy")
	}
}

func TestCodexProviderRunIterationEmitsErrorEvents(t *testing.T) {
	t.Parallel()

	provider := codexProvider{
		cfg: config.GenericProviderConfig{},
		run: func(_ context.Context, _ string, _ []string, _ chan Event) (string, error) {
			return "", errors.New("rate limit exceeded")
		},
	}

	events, _, err := provider.RunIteration(context.Background(), IterationRequest{
		Prompt: "hello",
	})
	if err != nil {
		t.Fatalf("expected start to succeed, got error: %v", err)
	}

	gotEvents := collectEvents(events)
	wantEventTypes := []EventType{
		EventIterationStarted,
		EventError,
		EventIterationDone,
	}
	if !reflect.DeepEqual(wantEventTypes, eventTypes(gotEvents)) {
		t.Fatalf("unexpected event sequence: got %v want %v", eventTypes(gotEvents), wantEventTypes)
	}
}

func TestCodexProviderRunIterationRejectsUnsupportedSandboxPolicy(t *testing.T) {
	t.Parallel()

	provider := codexProvider{
		cfg: config.GenericProviderConfig{
			SandboxPolicy: "read-only",
		},
	}

	_, _, err := provider.RunIteration(context.Background(), IterationRequest{
		Prompt: "hello",
	})
	if err == nil {
		t.Fatal("expected error for unsupported sandbox policy")
	}
}

func TestMapCodexErrorNotFound(t *testing.T) {
	t.Parallel()

	err := mapCodexError(exec.ErrNotFound)
	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Category != ErrorConfiguration {
		t.Fatalf("unexpected error category: %s", providerErr.Category)
	}
}

func TestMapCodexErrorRateLimit(t *testing.T) {
	t.Parallel()

	err := mapCodexError(errors.New("API returned 429 rate limit exceeded"))
	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Category != ErrorRateLimit {
		t.Fatalf("unexpected error category: %s", providerErr.Category)
	}
}

func TestReadCodexSummaryFallsBackToRawOutput(t *testing.T) {
	t.Parallel()

	summary, err := readCodexSummary("/definitely/missing/file", "  model summary  ")
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	if strings.TrimSpace(summary) != "model summary" {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func collectEvents(events <-chan Event) []Event {
	if events == nil {
		return nil
	}

	collected := make([]Event, 0)
	for event := range events {
		collected = append(collected, event)
	}
	return collected
}

func eventTypes(events []Event) []EventType {
	types := make([]EventType, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}
