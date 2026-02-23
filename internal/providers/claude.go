package providers

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/EstebanForge/daedalus/internal/config"
)

type claudeProvider struct {
	cfg config.GenericProviderConfig
	run func(ctx context.Context, workDir string, args []string, events chan Event) (string, error)
}

func newClaudeProvider(cfg config.Config) Provider {
	return claudeProvider{
		cfg: cfg.Providers.Claude,
		run: runClaudeCLICommand,
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

func (p claudeProvider) RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, IterationResult{}, NewConfigurationError("claude prompt is required", nil)
	}

	permissionMode, err := mapClaudePermissionMode(request.ApprovalPolicy, p.cfg.ApprovalPolicy)
	if err != nil {
		return nil, IterationResult{}, err
	}

	if err := validateClaudeSandboxPolicy(request.SandboxPolicy, p.cfg.SandboxPolicy); err != nil {
		return nil, IterationResult{}, err
	}

	events := make(chan Event, 8)
	pushProviderEvent(events, EventIterationStarted, "claude iteration started")

	args := []string{"-p", "--permission-mode", permissionMode}
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
		run = runClaudeCLICommand
	}

	go func() {
		defer close(events)

		output, runErr := run(ctx, request.WorkDir, args, events)
		if runErr != nil {
			mappedErr := mapClaudeError(runErr)
			pushProviderEvent(events, EventError, EncodeEventError(mappedErr))
			pushProviderEvent(events, EventIterationDone, "claude iteration failed")
			return
		}
		pushProviderEvent(events, EventCommandOutput, output)

		summary := strings.TrimSpace(output)
		pushProviderEvent(events, EventAssistantText, summary)
		pushProviderEvent(events, EventIterationDone, "claude iteration finished")
	}()

	return events, IterationResult{Success: true}, nil
}

func mapClaudePermissionMode(runtimeValue, configValue string) (string, error) {
	policy := strings.TrimSpace(runtimeValue)
	if policy == "" {
		policy = strings.TrimSpace(configValue)
	}
	if policy == "" {
		policy = "on-failure"
	}

	switch policy {
	case "on-failure":
		return "default", nil
	case "on-request":
		return "acceptEdits", nil
	case "never":
		return "dontAsk", nil
	default:
		return "", NewConfigurationError(fmt.Sprintf("unsupported claude approval policy %q", policy), nil)
	}
}

func validateClaudeSandboxPolicy(runtimeValue, configValue string) error {
	policy := strings.TrimSpace(runtimeValue)
	if policy == "" {
		policy = strings.TrimSpace(configValue)
	}
	if policy == "" {
		return nil
	}
	switch policy {
	case "workspace-write":
		return nil
	default:
		return NewConfigurationError(fmt.Sprintf("unsupported claude sandbox policy %q", policy), nil)
	}
}

func runClaudeCLICommand(ctx context.Context, workDir string, args []string, events chan Event) (string, error) {
	return runCLIStreaming(ctx, "claude", args, workDir, events)
}

func mapClaudeError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ProviderError{Category: ErrorTimeout, Message: err.Error(), Err: err}
	case errors.Is(err, exec.ErrNotFound):
		return NewConfigurationError("claude CLI binary not found in PATH", err)
	case strings.Contains(strings.ToLower(err.Error()), "executable file not found"), strings.Contains(strings.ToLower(err.Error()), "not found in $path"):
		return NewConfigurationError("claude CLI binary not found in PATH", err)
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "authentication"), strings.Contains(message, "auth"), strings.Contains(message, "token"), strings.Contains(message, "unauthorized"):
		return ProviderError{Category: ErrorAuthentication, Message: err.Error(), Err: err}
	case strings.Contains(message, "rate limit"), strings.Contains(message, "429"):
		return ProviderError{Category: ErrorRateLimit, Message: err.Error(), Err: err}
	case strings.Contains(message, "timeout"), strings.Contains(message, "timed out"), strings.Contains(message, "deadline exceeded"):
		return ProviderError{Category: ErrorTimeout, Message: err.Error(), Err: err}
	case strings.Contains(message, "temporar"), strings.Contains(message, "unavailable"), strings.Contains(message, "try again"):
		return ProviderError{Category: ErrorTransient, Message: err.Error(), Err: err}
	default:
		return ProviderError{Category: ErrorFatal, Message: err.Error(), Err: err}
	}
}
