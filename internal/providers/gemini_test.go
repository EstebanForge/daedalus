package providers

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	"github.com/EstebanForge/daedalus/internal/config"
)

// ── gemini ───────────────────────────────────────────────────────────────────

func TestGeminiProviderRunIterationBuildsCLIArgs(t *testing.T) {
	t.Parallel()

	var gotWorkDir string
	var gotArgs []string

	provider := geminiProvider{
		cfg: config.GenericProviderConfig{Model: "gemini-2.0-flash"},
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
		t.Fatalf("expected success result")
	}
	if gotWorkDir != "/tmp/work" {
		t.Fatalf("unexpected work dir: %q", gotWorkDir)
	}
	wantArgs := []string{"-p", "--model", "gemini-2.0-flash", "Implement story S-1"}
	if !reflect.DeepEqual(wantArgs, gotArgs) {
		t.Fatalf("unexpected args: got %v want %v", gotArgs, wantArgs)
	}
}

func TestGeminiProviderRunIterationNoModelFlag(t *testing.T) {
	t.Parallel()

	var gotArgs []string
	provider := geminiProvider{
		cfg: config.GenericProviderConfig{Model: "default"},
		run: func(_ context.Context, _ string, args []string, _ chan Event) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "ok", nil
		},
	}

	events, _, err := provider.RunIteration(context.Background(), IterationRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collectEvents(events) // wait for goroutine to finish
	wantArgs := []string{"-p", "hello"}
	if !reflect.DeepEqual(wantArgs, gotArgs) {
		t.Fatalf("unexpected args: got %v want %v", gotArgs, wantArgs)
	}
}

func TestGeminiProviderRunIterationRejectsEmptyPrompt(t *testing.T) {
	t.Parallel()

	provider := geminiProvider{cfg: config.GenericProviderConfig{}}
	_, _, err := provider.RunIteration(context.Background(), IterationRequest{Prompt: "  "})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestGeminiProviderRunIterationEmitsErrorEvents(t *testing.T) {
	t.Parallel()

	provider := geminiProvider{
		cfg: config.GenericProviderConfig{},
		run: func(_ context.Context, _ string, _ []string, _ chan Event) (string, error) {
			return "", errors.New("rate limit exceeded")
		},
	}

	events, _, err := provider.RunIteration(context.Background(), IterationRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("expected start to succeed, got error: %v", err)
	}

	gotEvents := collectEvents(events)
	wantEventTypes := []EventType{EventIterationStarted, EventError, EventIterationDone}
	if !reflect.DeepEqual(wantEventTypes, eventTypes(gotEvents)) {
		t.Fatalf("unexpected event sequence: got %v want %v", eventTypes(gotEvents), wantEventTypes)
	}
}

// ── opencode ─────────────────────────────────────────────────────────────────

func TestOpencodeProviderRunIterationBuildsCLIArgs(t *testing.T) {
	t.Parallel()

	var gotArgs []string
	provider := opencodeProvider{
		cfg: config.GenericProviderConfig{Model: "gpt-4o"},
		run: func(_ context.Context, _ string, args []string, _ chan Event) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "done", nil
		},
	}

	events, result, err := provider.RunIteration(context.Background(), IterationRequest{
		WorkDir: "/tmp",
		Prompt:  "do the thing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collectEvents(events) // wait for goroutine to finish
	if !result.Success {
		t.Fatal("expected success")
	}
	wantArgs := []string{"-p", "--model", "gpt-4o", "do the thing"}
	if !reflect.DeepEqual(wantArgs, gotArgs) {
		t.Fatalf("unexpected args: got %v want %v", gotArgs, wantArgs)
	}
}

func TestOpencodeProviderRunIterationRejectsEmptyPrompt(t *testing.T) {
	t.Parallel()

	provider := opencodeProvider{cfg: config.GenericProviderConfig{}}
	_, _, err := provider.RunIteration(context.Background(), IterationRequest{})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

// ── copilot ──────────────────────────────────────────────────────────────────

func TestCopilotProviderRunIterationUsesPromptFlag(t *testing.T) {
	t.Parallel()

	var gotArgs []string
	provider := copilotProvider{
		cfg: config.GenericProviderConfig{},
		run: func(_ context.Context, _ string, args []string, _ chan Event) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "done", nil
		},
	}

	events, _, err := provider.RunIteration(context.Background(), IterationRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collectEvents(events) // wait for goroutine to finish
	if len(gotArgs) < 2 || gotArgs[0] != "--prompt" {
		t.Fatalf("expected --prompt flag, got %v", gotArgs)
	}
}

func TestCopilotProviderRunIterationRejectsEmptyPrompt(t *testing.T) {
	t.Parallel()

	provider := copilotProvider{cfg: config.GenericProviderConfig{}}
	_, _, err := provider.RunIteration(context.Background(), IterationRequest{})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

// ── qwen ─────────────────────────────────────────────────────────────────────

func TestQwenProviderRunIterationBuildsCLIArgs(t *testing.T) {
	t.Parallel()

	var gotArgs []string
	provider := qwenProvider{
		cfg: config.GenericProviderConfig{},
		run: func(_ context.Context, _ string, args []string, _ chan Event) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "done", nil
		},
	}

	events, _, err := provider.RunIteration(context.Background(), IterationRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collectEvents(events) // wait for goroutine to finish
	wantArgs := []string{"-p", "hello"}
	if !reflect.DeepEqual(wantArgs, gotArgs) {
		t.Fatalf("unexpected args: got %v want %v", gotArgs, wantArgs)
	}
}

func TestQwenProviderRunIterationRejectsEmptyPrompt(t *testing.T) {
	t.Parallel()

	provider := qwenProvider{cfg: config.GenericProviderConfig{}}
	_, _, err := provider.RunIteration(context.Background(), IterationRequest{})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

// ── pi ───────────────────────────────────────────────────────────────────────

func TestPiProviderRunIterationBuildsCLIArgs(t *testing.T) {
	t.Parallel()

	var gotArgs []string
	provider := piProvider{
		cfg: config.GenericProviderConfig{},
		run: func(_ context.Context, _ string, args []string, _ chan Event) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "done", nil
		},
	}

	events, _, err := provider.RunIteration(context.Background(), IterationRequest{Prompt: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	collectEvents(events) // wait for goroutine to finish
	wantArgs := []string{"-p", "hello"}
	if !reflect.DeepEqual(wantArgs, gotArgs) {
		t.Fatalf("unexpected args: got %v want %v", gotArgs, wantArgs)
	}
}

func TestPiProviderRunIterationRejectsEmptyPrompt(t *testing.T) {
	t.Parallel()

	provider := piProvider{cfg: config.GenericProviderConfig{}}
	_, _, err := provider.RunIteration(context.Background(), IterationRequest{})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

// ── mapGenericCLIError ────────────────────────────────────────────────────────

func TestMapGenericCLIErrorNotFound(t *testing.T) {
	t.Parallel()

	err := mapGenericCLIError("gemini", exec.ErrNotFound)
	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Category != ErrorConfiguration {
		t.Fatalf("unexpected category: %s", providerErr.Category)
	}
}

func TestMapGenericCLIErrorRateLimit(t *testing.T) {
	t.Parallel()

	err := mapGenericCLIError("gemini", errors.New("API 429 rate limit exceeded"))
	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Category != ErrorRateLimit {
		t.Fatalf("unexpected category: %s", providerErr.Category)
	}
}

func TestMapGenericCLIErrorTransient(t *testing.T) {
	t.Parallel()

	err := mapGenericCLIError("gemini", errors.New("service temporarily unavailable"))
	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if providerErr.Category != ErrorTransient {
		t.Fatalf("unexpected category: %s", providerErr.Category)
	}
}

// ── registry ─────────────────────────────────────────────────────────────────

func TestRegistryResolvesAllProviders(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	names := []string{"codex", "claude", "gemini", "opencode", "copilot", "qwen", "pi"}
	for _, name := range names {
		p, err := registry.Resolve(name, config.Defaults())
		if err != nil {
			t.Errorf("resolve %q: %v", name, err)
			continue
		}
		if p.Name() != name {
			t.Errorf("expected name %q, got %q", name, p.Name())
		}
	}
}
