package providers

import (
	"context"
	"strings"

	"github.com/EstebanForge/daedalus/internal/config"
)

type opencodeProvider struct {
	cfg config.GenericProviderConfig
	run func(ctx context.Context, workDir string, args []string, events chan Event) (string, error)
}

func newOpencodeProvider(cfg config.Config) Provider {
	return opencodeProvider{
		cfg: cfg.Providers.OpenCode,
		run: runOpencodeCLICommand,
	}
}

func (p opencodeProvider) Name() string {
	return "opencode"
}

func (p opencodeProvider) Capabilities() Capabilities {
	return Capabilities{
		Streaming:      true,
		ToolCalls:      true,
		SandboxControl: false,
		ApprovalModes:  []string{"on-failure", "never"},
	}
}

func (p opencodeProvider) RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, IterationResult{}, NewConfigurationError("opencode prompt is required", nil)
	}

	events := make(chan Event, 8)
	pushProviderEvent(events, EventIterationStarted, "opencode iteration started")

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
		run = runOpencodeCLICommand
	}

	go func() {
		defer close(events)

		output, runErr := run(ctx, request.WorkDir, args, events)
		if runErr != nil {
			pushProviderEvent(events, EventError, EncodeEventError(mapGenericCLIError("opencode", runErr)))
			pushProviderEvent(events, EventIterationDone, "opencode iteration failed")
			return
		}
		pushProviderEvent(events, EventCommandOutput, output)

		summary := strings.TrimSpace(output)
		pushProviderEvent(events, EventAssistantText, summary)
		pushProviderEvent(events, EventIterationDone, "opencode iteration finished")
	}()

	return events, IterationResult{Success: true}, nil
}

func runOpencodeCLICommand(ctx context.Context, workDir string, args []string, events chan Event) (string, error) {
	return runCLIStreaming(ctx, "opencode", args, workDir, events)
}
