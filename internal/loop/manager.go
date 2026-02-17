package loop

import (
	"context"
	"fmt"
	"time"

	"github.com/EstebanForge/daedalus/internal/prd"
	"github.com/EstebanForge/daedalus/internal/providers"
)

type RetryPolicy struct {
	MaxRetries int
	Delays     []time.Duration
}

type Manager struct {
	store    prd.Store
	provider providers.Provider
	retry    RetryPolicy
}

func NewManager(store prd.Store, provider providers.Provider, retry RetryPolicy) Manager {
	return Manager{
		store:    store,
		provider: provider,
		retry:    retry,
	}
}

func (m Manager) RunOnce(ctx context.Context, name string, workDir string) error {
	doc, err := m.store.Load(name)
	if err != nil {
		return err
	}

	story := doc.NextStory()
	if story == nil {
		return nil
	}

	prompt := "Implement story " + story.ID + ": " + story.Title
	request := providers.IterationRequest{
		WorkDir:        workDir,
		Prompt:         prompt,
		ApprovalPolicy: "on-failure",
		SandboxPolicy:  "workspace-write",
		Model:          "default",
	}

	if err := m.runIterationWithRetry(ctx, request); err != nil {
		return fmt.Errorf("iteration failed: %w", err)
	}

	return nil
}

func (m Manager) runIterationWithRetry(ctx context.Context, request providers.IterationRequest) error {
	var lastErr error
	totalAttempts := m.retry.MaxRetries + 1
	if totalAttempts < 1 {
		totalAttempts = 1
	}

	for attempt := 0; attempt < totalAttempts; attempt++ {
		_, _, err := m.provider.RunIteration(ctx, request)
		if err == nil {
			return nil
		}

		lastErr = err
		if !providers.IsRetryable(err) {
			return err
		}

		if attempt == totalAttempts-1 {
			break
		}

		delay := m.retryDelay(attempt)
		if delay <= 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

func (m Manager) retryDelay(attempt int) time.Duration {
	if len(m.retry.Delays) == 0 {
		return 0
	}
	if attempt < len(m.retry.Delays) {
		return m.retry.Delays[attempt]
	}
	return m.retry.Delays[len(m.retry.Delays)-1]
}
