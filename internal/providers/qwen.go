package providers

import (
	"context"
	"strings"

	"github.com/EstebanForge/daedalus/internal/config"
)

type qwenProvider struct {
	cfg config.GenericProviderConfig
	run func(ctx context.Context, workDir string, args []string, events chan Event) (string, error)
}

func newQwenProvider(cfg config.Config) Provider {
	return qwenProvider{
		cfg: cfg.Providers.Qwen,
		run: runQwenCLICommand,
	}
}

func (p qwenProvider) Name() string {
	return "qwen"
}

func (p qwenProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming:      true,
		ToolCalls:      true,
		SandboxControl: false,
		ApprovalModes:  []string{"on-failure", "never"},
	}
}

func (p qwenProvider) RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, IterationResult{}, NewConfigurationError("qwen prompt is required", nil)
	}

	events := make(chan Event, 8)
	pushProviderEvent(events, EventIterationStarted, "qwen iteration started")

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
		run = runQwenCLICommand
	}

	go func() {
		defer close(events)

		output, runErr := run(ctx, request.WorkDir, args, events)
		if runErr != nil {
			pushProviderEvent(events, EventError, EncodeEventError(mapGenericCLIError("qwen", runErr)))
			pushProviderEvent(events, EventIterationDone, "qwen iteration failed")
			return
		}
		pushProviderEvent(events, EventCommandOutput, output)

		summary := strings.TrimSpace(output)
		pushProviderEvent(events, EventAssistantText, summary)
		pushProviderEvent(events, EventIterationDone, "qwen iteration finished")
	}()

	return events, IterationResult{Success: true}, nil
}

func runQwenCLICommand(ctx context.Context, workDir string, args []string, events chan Event) (string, error) {
	return runCLIStreaming(ctx, "qwen", args, workDir, events)
}
