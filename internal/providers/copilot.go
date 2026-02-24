package providers

import (
	"context"
	"strings"

	"github.com/EstebanForge/daedalus/internal/config"
)

// copilotProvider wraps the GitHub Copilot CLI adapter.
// Invocation: copilot --prompt <prompt> [--model <model>]
type copilotProvider struct {
	cfg config.GenericProviderConfig
	run func(ctx context.Context, workDir string, args []string, events chan Event) (string, error)
}

func newCopilotProvider(cfg config.Config) Provider {
	return copilotProvider{
		cfg: cfg.Providers.Copilot,
		run: runCopilotCLICommand,
	}
}

func (p copilotProvider) Name() string {
	return "copilot"
}

func (p copilotProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming:      true,
		ToolCalls:      true,
		SandboxControl: false,
		ApprovalModes:  []string{"on-failure", "never"},
	}
}

func (p copilotProvider) RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, IterationResult{}, NewConfigurationError("copilot prompt is required", nil)
	}

	events := make(chan Event, 8)
	pushProviderEvent(events, EventIterationStarted, "copilot iteration started")

	// Copilot CLI uses --prompt flag.
	args := []string{"--prompt"}
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
		run = runCopilotCLICommand
	}

	go func() {
		defer close(events)

		output, runErr := run(ctx, request.WorkDir, args, events)
		if runErr != nil {
			pushProviderEvent(events, EventError, EncodeEventError(mapGenericCLIError("copilot", runErr)))
			pushProviderEvent(events, EventIterationDone, "copilot iteration failed")
			return
		}
		pushProviderEvent(events, EventCommandOutput, output)

		summary := strings.TrimSpace(output)
		pushProviderEvent(events, EventAssistantText, summary)
		pushProviderEvent(events, EventIterationDone, "copilot iteration finished")
	}()

	return events, IterationResult{Success: true}, nil
}

func runCopilotCLICommand(ctx context.Context, workDir string, args []string, events chan Event) (string, error) {
	return runCLIStreaming(ctx, "copilot", args, workDir, events)
}
