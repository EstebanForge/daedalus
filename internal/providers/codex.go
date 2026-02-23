package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/EstebanForge/daedalus/internal/config"
)

type codexProvider struct {
	cfg config.GenericProviderConfig
	run func(ctx context.Context, workDir string, args []string, events chan Event) (string, error)
}

func newCodexProvider(cfg config.Config) Provider {
	return codexProvider{
		cfg: cfg.Providers.Codex,
		run: runCodexCLICommand,
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

func (p codexProvider) RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, IterationResult{}, NewConfigurationError("codex prompt is required", nil)
	}

	if _, err := resolveCodexApprovalPolicy(request.ApprovalPolicy, p.cfg.ApprovalPolicy); err != nil {
		return nil, IterationResult{}, err
	}
	sandbox, err := resolveCodexSandboxPolicy(request.SandboxPolicy, p.cfg.SandboxPolicy)
	if err != nil {
		return nil, IterationResult{}, err
	}

	events := make(chan Event, 8)
	pushProviderEvent(events, EventIterationStarted, "codex iteration started")

	outputFile, err := os.CreateTemp("", "daedalus-codex-last-message-*.txt")
	if err != nil {
		close(events)
		return nil, IterationResult{}, NewConfigurationError("failed to create codex output temp file", err)
	}
	outputPath := outputFile.Name()
	if closeErr := outputFile.Close(); closeErr != nil {
		close(events)
		return nil, IterationResult{}, NewConfigurationError("failed to initialize codex output temp file", closeErr)
	}
	defer func() {
		_ = os.Remove(outputPath)
	}()

	args := []string{"exec", "--sandbox", sandbox, "--skip-git-repo-check", "--output-last-message", outputPath}
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
		run = runCodexCLICommand
	}

	go func() {
		defer close(events)

		rawOutput, runErr := run(ctx, request.WorkDir, args, events)
		if runErr != nil {
			mappedErr := mapCodexError(runErr)
			pushProviderEvent(events, EventError, EncodeEventError(mappedErr))
			pushProviderEvent(events, EventIterationDone, "codex iteration failed")
			return
		}
		pushProviderEvent(events, EventCommandOutput, rawOutput)

		summary, summaryErr := readCodexSummary(outputPath, rawOutput)
		if summaryErr != nil {
			pushProviderEvent(events, EventError, EncodeEventError(summaryErr))
			pushProviderEvent(events, EventIterationDone, "codex iteration failed")
			return
		}
		pushProviderEvent(events, EventAssistantText, summary)
		pushProviderEvent(events, EventIterationDone, "codex iteration finished")
	}()

	return events, IterationResult{Success: true}, nil
}

func pushProviderEvent(events chan Event, eventType EventType, message string) {
	if events == nil {
		return
	}
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	events <- Event{Type: eventType, Message: trimmed}
}

func resolveCodexApprovalPolicy(runtimeValue, configValue string) (string, error) {
	policy := strings.TrimSpace(runtimeValue)
	if policy == "" {
		policy = strings.TrimSpace(configValue)
	}
	if policy == "" {
		policy = "on-failure"
	}

	switch policy {
	case "on-failure", "on-request", "never":
		return policy, nil
	default:
		return "", NewConfigurationError(fmt.Sprintf("unsupported codex approval policy %q", policy), nil)
	}
}

func resolveCodexSandboxPolicy(runtimeValue, configValue string) (string, error) {
	policy := strings.TrimSpace(runtimeValue)
	if policy == "" {
		policy = strings.TrimSpace(configValue)
	}
	if policy == "" {
		policy = "workspace-write"
	}

	switch policy {
	case "workspace-write":
		return policy, nil
	default:
		return "", NewConfigurationError(fmt.Sprintf("unsupported codex sandbox policy %q", policy), nil)
	}
}

func runCodexCLICommand(ctx context.Context, workDir string, args []string, events chan Event) (string, error) {
	return runCLIStreaming(ctx, "codex", args, workDir, events)
}

func readCodexSummary(outputPath, rawOutput string) (string, error) {
	data, err := os.ReadFile(outputPath)
	if err == nil {
		text := strings.TrimSpace(string(data))
		if text != "" {
			return text, nil
		}
	}

	fallback := strings.TrimSpace(rawOutput)
	if fallback == "" {
		return "", NewConfigurationError("codex returned empty output", nil)
	}
	return fallback, nil
}

func mapCodexError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ProviderError{Category: ErrorTimeout, Message: err.Error(), Err: err}
	case errors.Is(err, exec.ErrNotFound):
		return NewConfigurationError("codex CLI binary not found in PATH", err)
	case strings.Contains(strings.ToLower(err.Error()), "executable file not found"), strings.Contains(strings.ToLower(err.Error()), "not found in $path"):
		return NewConfigurationError("codex CLI binary not found in PATH", err)
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
