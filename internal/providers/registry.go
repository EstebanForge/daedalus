package providers

import (
	"strings"

	"github.com/EstebanForge/daedalus/internal/config"
)

type Registry struct {
	builders map[string]func(config.Config) Provider
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
		key = cfg.Provider.Default
	}

	builder, exists := r.builders[key]
	if !exists {
		return nil, NewUnknownProviderError(key)
	}

	return builder(cfg), nil
}
