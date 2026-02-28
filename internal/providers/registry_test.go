package providers

import (
	"strings"
	"testing"

	"github.com/EstebanForge/daedalus/internal/config"
)

func TestRegistryResolveUnknownProviderReturnsError(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	_, err := registry.Resolve("unknown", config.Defaults())
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryResolveUsesConfigDefaultWhenNameEmpty(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Provider.Default = "claude"
	cfg.Providers.Claude.Enabled = true

	registry := NewRegistry()
	provider, err := registry.Resolve("", cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if provider.Name() != "claude" {
		t.Fatalf("expected claude provider, got %q", provider.Name())
	}
}

func TestRegistryResolveNormalizesCase(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Providers.Claude.Enabled = true

	registry := NewRegistry()
	provider, err := registry.Resolve("  CLAUDE  ", cfg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if provider.Name() != "claude" {
		t.Fatalf("expected claude provider, got %q", provider.Name())
	}
}

func TestRegistryResolveDisabledProviderReturnsError(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Provider.Default = "claude"
	cfg.Providers.Claude.Enabled = false

	registry := NewRegistry()
	_, err := registry.Resolve("", cfg)
	if err == nil {
		t.Fatal("expected error for disabled provider")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}
