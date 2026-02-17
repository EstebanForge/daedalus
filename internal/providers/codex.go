package providers

import (
	"context"

	"github.com/EstebanForge/daedalus/internal/config"
)

type codexProvider struct {
	cfg config.GenericProviderConfig
}

func newCodexProvider(cfg config.Config) Provider {
	return codexProvider{
		cfg: cfg.Providers.Codex,
	}
}

func (p codexProvider) Name() string {
	return "codex"
}

func (p codexProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming:      true,
		ToolCalls:      true,
		SandboxControl: true,
		ApprovalModes:  []string{"on-failure", "on-request", "never"},
	}
}

func (p codexProvider) RunIteration(_ context.Context, _ IterationRequest) (<-chan Event, IterationResult, error) {
	events := make(chan Event)
	close(events)

	return events, IterationResult{}, NewConfigurationError("codex provider module is not implemented yet", nil)
}
