package providers

import (
	"context"
	"strings"

	"github.com/EstebanForge/daedalus/internal/config"
)

type piProvider struct {
	cfg config.GenericProviderConfig
	run func(ctx context.Context, workDir string, args []string, events chan Event) (string, error)
}

func newPiProvider(cfg config.Config) Provider {
	return piProvider{
		cfg: cfg.Providers.Pi,
		run: runPiCLICommand,
	}
}

func (p piProvider) Name() string {
	return "pi"
}

func (p piProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming:      true,
		ToolCalls:      true,
		SandboxControl: false,
		ApprovalModes:  []string{"on-failure", "never"},
	}
}

func (p piProvider) RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, IterationResult{}, NewConfigurationError("pi prompt is required", nil)
	}

	events := make(chan Event, 8)
	pushProviderEvent(events, EventIterationStarted, "pi iteration started")

	args := []string{"-p"}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = strings.TrimSpace(p.cfg.Model)
	}
	if model != "" && model != "default" {
		args = append(args, "--model", model)
	}
	args = append(args, request.Prompt)

	run := p.run
	if run == nil {
		run = runPiCLICommand
	}

	go func() {
		defer close(events)

		output, runErr := run(ctx, request.WorkDir, args, events)
		if runErr != nil {
			pushProviderEvent(events, EventError, EncodeEventError(mapGenericCLIError("pi", runErr)))
			pushProviderEvent(events, EventIterationDone, "pi iteration failed")
			return
		}
		pushProviderEvent(events, EventCommandOutput, output)

		summary := strings.TrimSpace(output)
		pushProviderEvent(events, EventAssistantText, summary)
		pushProviderEvent(events, EventIterationDone, "pi iteration finished")
	}()

	return events, IterationResult{Success: true}, nil
}

func runPiCLICommand(ctx context.Context, workDir string, args []string, events chan Event) (string, error) {
	return runCLIStreaming(ctx, "pi", args, workDir, events)
}
