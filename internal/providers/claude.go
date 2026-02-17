package providers

import (
	"context"

	"github.com/EstebanForge/daedalus/internal/config"
)

type claudeProvider struct {
	cfg config.GenericProviderConfig
}

func newClaudeProvider(cfg config.Config) Provider {
	return claudeProvider{
		cfg: cfg.Providers.Claude,
	}
}

func (p claudeProvider) Name() string {
	return "claude"
}

func (p claudeProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming:      true,
		ToolCalls:      true,
		SandboxControl: true,
		ApprovalModes:  []string{"on-failure", "on-request", "never"},
	}
}

func (p claudeProvider) RunIteration(_ context.Context, _ IterationRequest) (<-chan Event, IterationResult, error) {
	events := make(chan Event)
	close(events)

	return events, IterationResult{}, NewConfigurationError("claude provider module is not implemented yet", nil)
}
