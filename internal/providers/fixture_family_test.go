package providers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EstebanForge/daedalus/internal/config"
)

// nativeACPProviders are providers that use the "<binary> acp" invocation pattern.
var nativeACPProviders = []string{"opencode", "gemini", "qwen", "copilot"}

// adapterACPProviders are providers that use a dedicated adapter binary.
var adapterACPProviders = []string{"codex", "claude", "pi"}

// TestFixtureNativeACPCommandShape verifies all native ACP providers resolve
// to the "<binary> acp" command shape.
func TestFixtureNativeACPCommandShape(t *testing.T) {
	t.Parallel()

	for _, key := range nativeACPProviders {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			cmd := resolveACPCommand(key, "")
			if cmd.Binary != key {
				t.Fatalf("binary: got %q want %q", cmd.Binary, key)
			}
			if len(cmd.Args) == 0 || cmd.Args[0] != "acp" {
				t.Fatalf("args: expected [\"acp\"], got %v", cmd.Args)
			}
		})
	}
}

// TestFixtureAdapterACPCommandShape verifies adapter-backed providers resolve
// to their dedicated adapter binary (no extra args).
func TestFixtureAdapterACPCommandShape(t *testing.T) {
	t.Parallel()

	wantBinaries := map[string]string{
		"codex":  "codex-acp",
		"claude": "claude-agent-acp",
		"pi":     "pi-acp",
	}

	for key, wantBinary := range wantBinaries {
		key, wantBinary := key, wantBinary
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			cmd := resolveACPCommand(key, "")
			if cmd.Binary != wantBinary {
				t.Fatalf("binary: got %q want %q", cmd.Binary, wantBinary)
			}
			if len(cmd.Args) != 0 {
				t.Fatalf("expected no extra args for adapter provider %q, got %v", key, cmd.Args)
			}
		})
	}
}

// TestFixtureNativeACPProviderFamilyIteration runs a full ACP iteration lifecycle
// fixture for every native ACP provider (opencode, gemini, qwen, copilot) using
// a simulated ACP server.
func TestFixtureNativeACPProviderFamilyIteration(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Cleanup(CloseAllSessions)

	registry := NewRegistry()
	cfg := enabledACPConfig(helperACPCommand("acp-helper"))

	for _, key := range nativeACPProviders {
		key := key
		t.Run(key, func(t *testing.T) {
			provider, err := registry.Resolve(key, cfg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}

			events, result, runErr := provider.RunIteration(context.Background(), IterationRequest{
				WorkDir: t.TempDir(),
				Prompt:  "fixture native family iteration",
			})
			if runErr != nil {
				t.Fatalf("run iteration: %v", runErr)
			}
			if !result.Success {
				t.Fatalf("expected success result, got %+v", result)
			}

			collected := collectEvents(events)
			types := eventTypes(collected)
			if len(types) == 0 {
				t.Fatal("expected at least one event")
			}
			if types[0] != EventIterationStarted {
				t.Fatalf("expected first event %q, got %q", EventIterationStarted, types[0])
			}
			if types[len(types)-1] != EventIterationDone {
				t.Fatalf("expected last event %q, got %q", EventIterationDone, types[len(types)-1])
			}
			for _, ev := range collected {
				if ev.Type == EventError {
					t.Fatalf("unexpected error event for provider %q: %s", key, ev.Message)
				}
			}
			if !containsEventType(collected, EventToolStarted) {
				t.Fatalf("expected tool_started event for provider %q, got types: %v", key, types)
			}
			if !containsEventType(collected, EventToolFinished) {
				t.Fatalf("expected tool_finished event for provider %q, got types: %v", key, types)
			}
		})
	}
}

// TestFixtureAdapterACPProviderFamilyIteration runs a full ACP iteration lifecycle
// fixture for every adapter-backed provider (codex, claude, pi) using a simulated
// ACP server.
func TestFixtureAdapterACPProviderFamilyIteration(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Cleanup(CloseAllSessions)

	registry := NewRegistry()
	cfg := enabledACPConfig(helperACPCommand("acp-helper"))

	for _, key := range adapterACPProviders {
		key := key
		t.Run(key, func(t *testing.T) {
			provider, err := registry.Resolve(key, cfg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}

			events, result, runErr := provider.RunIteration(context.Background(), IterationRequest{
				WorkDir: t.TempDir(),
				Prompt:  "fixture adapter family iteration",
			})
			if runErr != nil {
				t.Fatalf("run iteration: %v", runErr)
			}
			if !result.Success {
				t.Fatalf("expected success result, got %+v", result)
			}

			collected := collectEvents(events)
			types := eventTypes(collected)
			if len(types) == 0 {
				t.Fatal("expected at least one event")
			}
			if types[0] != EventIterationStarted {
				t.Fatalf("expected first event %q, got %q", EventIterationStarted, types[0])
			}
			if types[len(types)-1] != EventIterationDone {
				t.Fatalf("expected last event %q, got %q", EventIterationDone, types[len(types)-1])
			}
			for _, ev := range collected {
				if ev.Type == EventError {
					t.Fatalf("unexpected error event for provider %q: %s", key, ev.Message)
				}
			}
			if !containsEventType(collected, EventToolStarted) {
				t.Fatalf("expected tool_started event for provider %q, got types: %v", key, types)
			}
			if !containsEventType(collected, EventToolFinished) {
				t.Fatalf("expected tool_finished event for provider %q, got types: %v", key, types)
			}
		})
	}
}

// TestFixtureACPContextFilesInjected verifies that context files listed in the
// iteration request are read from disk and their content is appended to the
// prompt sent to the ACP provider.
func TestFixtureACPContextFilesInjected(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Cleanup(CloseAllSessions)

	workDir := t.TempDir()
	const contextMarker = "fixture-context-marker-XYZ"
	contextFile := filepath.Join(workDir, "context.md")
	if err := os.WriteFile(contextFile, []byte(contextMarker), 0o644); err != nil {
		t.Fatalf("write context file: %v", err)
	}

	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Codex.ACPCommand = helperACPCommand("acp-echo-helper")

	provider := newACPProvider(cfg, "codex")
	events, _, err := provider.RunIteration(context.Background(), IterationRequest{
		WorkDir:      workDir,
		Prompt:       "review the context",
		ContextFiles: []string{"context.md"},
	})
	if err != nil {
		t.Fatalf("run iteration: %v", err)
	}

	collected := collectEvents(events)
	if !assistantTextContains(collected, contextMarker) {
		t.Fatalf("expected context marker %q in assistant text, got events: %+v", contextMarker, collected)
	}
}

// assistantTextContains reports whether any EventAssistantText event's message
// contains the given substring.
func assistantTextContains(events []Event, substring string) bool {
	for _, ev := range events {
		if ev.Type == EventAssistantText && strings.Contains(ev.Message, substring) {
			return true
		}
	}
	return false
}
