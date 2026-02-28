package providers

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
)

func TestACPIntegrationProviders(t *testing.T) {
	if strings.TrimSpace(os.Getenv("DAEDALUS_RUN_ACP_INTEGRATION")) != "1" {
		t.Skip("set DAEDALUS_RUN_ACP_INTEGRATION=1 to run real-provider ACP integration tests")
	}
	defer CloseAllSessions()

	registry := NewRegistry()
	cfg := enabledACPConfig("")

	for _, key := range providerKeys {
		key := key
		t.Run(key, func(t *testing.T) {
			applyIntegrationCommandOverride(&cfg, key)
			assertACPCommandAvailable(t, cfg, key)

			provider, err := registry.Resolve(key, cfg)
			if err != nil {
				t.Fatalf("resolve provider: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			events, result, runErr := provider.RunIteration(ctx, IterationRequest{
				WorkDir: t.TempDir(),
				Prompt:  "Reply with exactly: integration-ok",
			})
			if runErr != nil {
				t.Fatalf("run iteration: %v", runErr)
			}
			if !result.Success {
				t.Fatalf("expected started run success, got %+v", result)
			}

			collected := collectEvents(events)
			types := eventTypes(collected)
			if len(types) == 0 {
				t.Fatal("expected provider events")
			}
			if types[0] != EventIterationStarted {
				t.Fatalf("expected first event %q, got %q", EventIterationStarted, types[0])
			}
			if types[len(types)-1] != EventIterationDone {
				t.Fatalf("expected last event %q, got %q", EventIterationDone, types[len(types)-1])
			}
			for _, event := range collected {
				if event.Type == EventError {
					t.Fatalf("provider emitted runtime error: %s", event.Message)
				}
			}
		})
	}
}

func applyIntegrationCommandOverride(cfg *config.Config, key string) {
	envKey := "DAEDALUS_" + strings.ToUpper(key) + "_ACP_COMMAND"
	override := strings.TrimSpace(os.Getenv(envKey))
	if override == "" {
		return
	}

	switch key {
	case "codex":
		cfg.Providers.Codex.ACPCommand = override
	case "claude":
		cfg.Providers.Claude.ACPCommand = override
	case "gemini":
		cfg.Providers.Gemini.ACPCommand = override
	case "opencode":
		cfg.Providers.OpenCode.ACPCommand = override
	case "copilot":
		cfg.Providers.Copilot.ACPCommand = override
	case "qwen":
		cfg.Providers.Qwen.ACPCommand = override
	case "pi":
		cfg.Providers.Pi.ACPCommand = override
	}
}

func assertACPCommandAvailable(t *testing.T, cfg config.Config, key string) {
	t.Helper()

	providerCfg := getProviderConfig(cfg, key)
	command := resolveACPCommand(key, providerCfg.ACPCommand)
	if _, err := exec.LookPath(command.Binary); err != nil {
		t.Fatalf("acp command binary %q for provider %q not found in PATH: %v", command.Binary, key, err)
	}
}
