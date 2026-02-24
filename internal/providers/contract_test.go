package providers

// contract_test.go — shared provider contract suite
//
// Each entry in allProviderEntries creates a concrete provider with an injected
// run function so the contract properties can be verified without a real CLI.
//
// Contract rules (from docs/reference/providers.md):
//   - Empty prompt → pre-start error, nil event channel, result.Success=false
//   - Valid request → no error on start; first event=iteration_started,
//     last event=iteration_finished; channel closes exactly once
//   - Provider run error → started,error,iteration_finished sequence
//   - Name() returns the registered key
//   - Capabilities() returns non-empty ApprovalModes

import (
	"context"
	"errors"
	"testing"

	"github.com/EstebanForge/daedalus/internal/config"
)

// runFn is the injectable run function type shared across all provider structs.
type runFn = func(ctx context.Context, workDir string, args []string, events chan Event) (string, error)

type providerEntry struct {
	name string
	make func(run runFn) Provider
}

func allProviderEntries() []providerEntry {
	return []providerEntry{
		{
			name: "codex",
			make: func(run runFn) Provider {
				return codexProvider{cfg: config.GenericProviderConfig{}, run: run}
			},
		},
		{
			name: "claude",
			make: func(run runFn) Provider {
				return claudeProvider{cfg: config.GenericProviderConfig{}, run: run}
			},
		},
		{
			name: "gemini",
			make: func(run runFn) Provider {
				return geminiProvider{cfg: config.GenericProviderConfig{}, run: run}
			},
		},
		{
			name: "opencode",
			make: func(run runFn) Provider {
				return opencodeProvider{cfg: config.GenericProviderConfig{}, run: run}
			},
		},
		{
			name: "copilot",
			make: func(run runFn) Provider {
				return copilotProvider{cfg: config.GenericProviderConfig{}, run: run}
			},
		},
		{
			name: "qwen",
			make: func(run runFn) Provider {
				return qwenProvider{cfg: config.GenericProviderConfig{}, run: run}
			},
		},
		{
			name: "pi",
			make: func(run runFn) Provider {
				return piProvider{cfg: config.GenericProviderConfig{}, run: run}
			},
		},
	}
}

// ── Contract 1: Name() ────────────────────────────────────────────────────────

func TestContractName(t *testing.T) {
	t.Parallel()

	noopRun := func(_ context.Context, _ string, _ []string, _ chan Event) (string, error) {
		return "ok", nil
	}
	for _, entry := range allProviderEntries() {
		entry := entry
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			p := entry.make(noopRun)
			if p.Name() != entry.name {
				t.Fatalf("expected Name()=%q, got %q", entry.name, p.Name())
			}
		})
	}
}

// ── Contract 2: Capabilities() ────────────────────────────────────────────────

func TestContractCapabilitiesNonEmpty(t *testing.T) {
	t.Parallel()

	noopRun := func(_ context.Context, _ string, _ []string, _ chan Event) (string, error) {
		return "ok", nil
	}
	for _, entry := range allProviderEntries() {
		entry := entry
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			p := entry.make(noopRun)
			caps := p.Capabilities()
			if len(caps.ApprovalModes) == 0 {
				t.Fatalf("%s: expected non-empty ApprovalModes, got empty", entry.name)
			}
		})
	}
}

// ── Contract 3: Empty prompt → pre-start error ────────────────────────────────

func TestContractEmptyPromptRejected(t *testing.T) {
	t.Parallel()

	noopRun := func(_ context.Context, _ string, _ []string, _ chan Event) (string, error) {
		return "ok", nil
	}
	for _, entry := range allProviderEntries() {
		entry := entry
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			p := entry.make(noopRun)
			events, result, err := p.RunIteration(context.Background(), IterationRequest{Prompt: "  "})
			if err == nil {
				t.Fatalf("%s: expected pre-start error for empty prompt", entry.name)
			}
			if events != nil {
				t.Fatalf("%s: expected nil event channel on pre-start error", entry.name)
			}
			if result.Success {
				t.Fatalf("%s: expected result.Success=false on pre-start error", entry.name)
			}
		})
	}
}

// ── Contract 4: Success event sequence ────────────────────────────────────────

func TestContractSuccessEventSequence(t *testing.T) {
	t.Parallel()

	successRun := func(_ context.Context, _ string, _ []string, _ chan Event) (string, error) {
		return "output text", nil
	}
	for _, entry := range allProviderEntries() {
		entry := entry
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			p := entry.make(successRun)
			events, result, err := p.RunIteration(context.Background(), IterationRequest{
				WorkDir: "/tmp",
				Prompt:  "do the work",
			})
			if err != nil {
				t.Fatalf("%s: expected no pre-start error, got: %v", entry.name, err)
			}
			if events == nil {
				t.Fatalf("%s: expected non-nil event channel on success start", entry.name)
			}
			if !result.Success {
				t.Fatalf("%s: expected result.Success=true on success start", entry.name)
			}

			collected := collectEvents(events)
			types := eventTypes(collected)

			if len(types) == 0 {
				t.Fatalf("%s: expected at least one event", entry.name)
			}
			if types[0] != EventIterationStarted {
				t.Fatalf("%s: expected first event %q, got %q", entry.name, EventIterationStarted, types[0])
			}
			if types[len(types)-1] != EventIterationDone {
				t.Fatalf("%s: expected last event %q, got %q", entry.name, EventIterationDone, types[len(types)-1])
			}
		})
	}
}

// ── Contract 5: Channel closes exactly once ───────────────────────────────────

func TestContractChannelCloses(t *testing.T) {
	t.Parallel()

	successRun := func(_ context.Context, _ string, _ []string, _ chan Event) (string, error) {
		return "output", nil
	}
	for _, entry := range allProviderEntries() {
		entry := entry
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			p := entry.make(successRun)
			events, _, err := p.RunIteration(context.Background(), IterationRequest{Prompt: "hello"})
			if err != nil {
				t.Fatalf("%s: unexpected pre-start error: %v", entry.name, err)
			}
			// Draining must complete (channel must close); test times out otherwise.
			collectEvents(events)
		})
	}
}

// ── Contract 6: Provider run error → started,error,finished sequence ──────────

func TestContractRunErrorEventSequence(t *testing.T) {
	t.Parallel()

	errorRun := func(_ context.Context, _ string, _ []string, _ chan Event) (string, error) {
		return "", errors.New("simulated provider failure")
	}
	for _, entry := range allProviderEntries() {
		entry := entry
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			p := entry.make(errorRun)
			events, _, err := p.RunIteration(context.Background(), IterationRequest{Prompt: "hello"})
			if err != nil {
				t.Fatalf("%s: expected no pre-start error, got: %v", entry.name, err)
			}

			collected := collectEvents(events)
			types := eventTypes(collected)

			if len(types) < 3 {
				t.Fatalf("%s: expected at least 3 events on run error, got %d: %v", entry.name, len(types), types)
			}
			if types[0] != EventIterationStarted {
				t.Fatalf("%s: expected first event %q, got %q", entry.name, EventIterationStarted, types[0])
			}
			// Second-to-last must be error
			errIdx := len(types) - 2
			if types[errIdx] != EventError {
				t.Fatalf("%s: expected second-to-last event %q, got %q", entry.name, EventError, types[errIdx])
			}
			if types[len(types)-1] != EventIterationDone {
				t.Fatalf("%s: expected last event %q, got %q", entry.name, EventIterationDone, types[len(types)-1])
			}
		})
	}
}

// ── Contract 7: Context cancellation does not panic ───────────────────────────

func TestContractContextCancellationNoPanic(t *testing.T) {
	t.Parallel()

	blockedRun := func(ctx context.Context, _ string, _ []string, _ chan Event) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	for _, entry := range allProviderEntries() {
		entry := entry
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			p := entry.make(blockedRun)
			ctx, cancel := context.WithCancel(context.Background())

			events, _, err := p.RunIteration(ctx, IterationRequest{Prompt: "hello"})
			if err != nil {
				t.Fatalf("%s: unexpected pre-start error: %v", entry.name, err)
			}
			cancel()
			// Must drain cleanly with no panic or deadlock.
			collectEvents(events)
		})
	}
}
