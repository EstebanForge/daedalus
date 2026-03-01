package providers

import (
	"fmt"
	"strings"

	"github.com/EstebanForge/daedalus/internal/config"
)

type Registry struct {
	builders map[string]func(config.Config) Provider
}

var knownProviderKeys = []string{
	"codex",
	"claude",
	"gemini",
	"opencode",
	"copilot",
	"qwen",
	"pi",
}

// KnownProviderKeys returns the canonical provider keys supported by Daedalus.
func KnownProviderKeys() []string {
	return append([]string(nil), knownProviderKeys...)
}

func NewRegistry() Registry {
	return Registry{
		builders: map[string]func(config.Config) Provider{
			"codex":    func(cfg config.Config) Provider { return newACPProvider(cfg, "codex") },
			"claude":   func(cfg config.Config) Provider { return newACPProvider(cfg, "claude") },
			"gemini":   func(cfg config.Config) Provider { return newACPProvider(cfg, "gemini") },
			"opencode": func(cfg config.Config) Provider { return newACPProvider(cfg, "opencode") },
			"copilot":  func(cfg config.Config) Provider { return newACPProvider(cfg, "copilot") },
			"qwen":     func(cfg config.Config) Provider { return newACPProvider(cfg, "qwen") },
			"pi":       func(cfg config.Config) Provider { return newACPProvider(cfg, "pi") },
		},
	}
}

func (r Registry) Resolve(name string, cfg config.Config) (Provider, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(cfg.Provider.Default))
	}

	builder, exists := r.builders[key]
	if !exists {
		return nil, NewUnknownProviderError(key)
	}
	if !providerEnabled(cfg, key) {
		return nil, NewConfigurationError(fmt.Sprintf("provider %q is disabled in config", key), nil)
	}

	return builder(cfg), nil
}

func providerEnabled(cfg config.Config, key string) bool {
	switch key {
	case "codex":
		return cfg.Providers.Codex.Enabled
	case "claude":
		return cfg.Providers.Claude.Enabled
	case "gemini":
		return cfg.Providers.Gemini.Enabled
	case "opencode":
		return cfg.Providers.OpenCode.Enabled
	case "copilot":
		return cfg.Providers.Copilot.Enabled
	case "qwen":
		return cfg.Providers.Qwen.Enabled
	case "pi":
		return cfg.Providers.Pi.Enabled
	default:
		return false
	}
}
