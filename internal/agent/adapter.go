package agent

import (
	"context"
	"fmt"
)

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
}

type IterationResult struct {
	Success bool
	Summary string
}

type Adapter interface {
	RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error)
}

type ErrAdapterNotConfigured struct{}

func (e ErrAdapterNotConfigured) Error() string {
	return "agent adapter not configured"
}

type StubAdapter struct{}

func (a StubAdapter) RunIteration(_ context.Context, _ IterationRequest) (<-chan Event, IterationResult, error) {
	events := make(chan Event)
	close(events)
	return events, IterationResult{}, ErrAdapterNotConfigured{}
}

func NewStubAdapter() Adapter {
	return StubAdapter{}
}

func IsAdapterError(err error) bool {
	var target ErrAdapterNotConfigured
	return err != nil && fmt.Sprintf("%T", err) == fmt.Sprintf("%T", target)
}
