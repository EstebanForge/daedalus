package providers

import (
	"context"
	"testing"

	"github.com/EstebanForge/daedalus/internal/config"
)

var providerKeys = []string{"codex", "claude", "gemini", "opencode", "copilot", "qwen", "pi"}

func TestContractNameACPProviders(t *testing.T) {
	cfg := enabledACPConfig("")
	registry := NewRegistry()

	for _, key := range providerKeys {
		key := key
		t.Run(key, func(t *testing.T) {
			provider, err := registry.Resolve(key, cfg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if provider.Name() != key {
				t.Fatalf("expected Name()=%q, got %q", key, provider.Name())
			}
			if len(provider.Capabilities().ApprovalModes) == 0 {
				t.Fatalf("%s: expected non-empty ApprovalModes", key)
			}
		})
	}
}

func TestContractEmptyPromptRejectedACPProviders(t *testing.T) {
	cfg := enabledACPConfig("")
	registry := NewRegistry()

	for _, key := range providerKeys {
		key := key
		t.Run(key, func(t *testing.T) {
			provider, err := registry.Resolve(key, cfg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			events, result, runErr := provider.RunIteration(context.Background(), IterationRequest{Prompt: " "})
			if runErr == nil {
				t.Fatal("expected pre-start error for empty prompt")
			}
			if events != nil {
				t.Fatal("expected nil event channel for pre-start error")
			}
			if result.Success {
				t.Fatalf("expected result.Success=false, got %+v", result)
			}
		})
	}
}

func TestContractSuccessEventSequenceACPProviders(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Cleanup(CloseAllSessions)

	cfg := enabledACPConfig(helperACPCommand("acp-helper"))
	registry := NewRegistry()

	for _, key := range providerKeys {
		key := key
		t.Run(key, func(t *testing.T) {
			provider, err := registry.Resolve(key, cfg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}

			events, result, runErr := provider.RunIteration(context.Background(), IterationRequest{
				WorkDir: t.TempDir(),
				Prompt:  "do the work",
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
		})
	}
}

func TestContractRunErrorEventSequenceACPProviders(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Cleanup(CloseAllSessions)

	cfg := enabledACPConfig(helperACPCommand("acp-error-helper"))
	registry := NewRegistry()

	for _, key := range providerKeys {
		key := key
		t.Run(key, func(t *testing.T) {
			provider, err := registry.Resolve(key, cfg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}

			events, _, runErr := provider.RunIteration(context.Background(), IterationRequest{
				WorkDir: t.TempDir(),
				Prompt:  "do the work",
			})
			if runErr != nil {
				t.Fatalf("expected no pre-start error, got %v", runErr)
			}

			collected := collectEvents(events)
			types := eventTypes(collected)
			if len(types) < 3 {
				t.Fatalf("expected at least 3 events, got %v", types)
			}
			if types[0] != EventIterationStarted {
				t.Fatalf("expected first event %q, got %q", EventIterationStarted, types[0])
			}
			if types[len(types)-2] != EventError {
				t.Fatalf("expected second-to-last event %q, got %q", EventError, types[len(types)-2])
			}
			if types[len(types)-1] != EventIterationDone {
				t.Fatalf("expected last event %q, got %q", EventIterationDone, types[len(types)-1])
			}
		})
	}
}

func enabledACPConfig(acpCommand string) config.Config {
	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Claude.Enabled = true
	cfg.Providers.Gemini.Enabled = true
	cfg.Providers.OpenCode.Enabled = true
	cfg.Providers.Copilot.Enabled = true
	cfg.Providers.Qwen.Enabled = true
	cfg.Providers.Pi.Enabled = true

	if acpCommand != "" {
		cfg.Providers.Codex.ACPCommand = acpCommand
		cfg.Providers.Claude.ACPCommand = acpCommand
		cfg.Providers.Gemini.ACPCommand = acpCommand
		cfg.Providers.OpenCode.ACPCommand = acpCommand
		cfg.Providers.Copilot.ACPCommand = acpCommand
		cfg.Providers.Qwen.ACPCommand = acpCommand
		cfg.Providers.Pi.ACPCommand = acpCommand
	}

	return cfg
}
