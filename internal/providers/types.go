package providers

import "context"

type EventType string

const (
	EventIterationStarted EventType = "iteration_started"
	EventAssistantText    EventType = "assistant_text"
	EventToolStarted      EventType = "tool_started"
	EventToolFinished     EventType = "tool_finished"
	EventCommandOutput    EventType = "command_output"
	EventIterationDone    EventType = "iteration_finished"
	EventError            EventType = "error"
)

type Event struct {
	Type    EventType
	Message string
}

type IterationRequest struct {
	WorkDir        string
	Prompt         string
	ContextFiles   []string
	ApprovalPolicy string
	SandboxPolicy  string
	Model          string
	Metadata       map[string]string
}

type IterationResult struct {
	Success       bool
	Summary       string
	ProviderRunID string
}

type Capabilities struct {
	Streaming      bool
	ToolCalls      bool
	SandboxControl bool
	ApprovalModes  []string
	MaxContextHint int
}

type Provider interface {
	Name() string
	Capabilities() Capabilities
	RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error)
}
