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
			"codex":  newCodexProvider,
			"claude": newClaudeProvider,
			"gemini": newGeminiProvider,
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
