package providers

import (
	"context"

	"github.com/EstebanForge/daedalus/internal/config"
)

type geminiProvider struct {
	cfg config.GenericProviderConfig
}

func newGeminiProvider(cfg config.Config) Provider {
	return geminiProvider{
		cfg: cfg.Providers.Gemini,
	}
}

func (p geminiProvider) Name() string {
	return "gemini"
}

func (p geminiProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming:      true,
		ToolCalls:      true,
		SandboxControl: true,
		ApprovalModes:  []string{"on-failure", "on-request", "never"},
	}
}

func (p geminiProvider) RunIteration(_ context.Context, _ IterationRequest) (<-chan Event, IterationResult, error) {
	events := make(chan Event)
	close(events)

	return events, IterationResult{}, NewConfigurationError("gemini provider module is not implemented yet", nil)
}
