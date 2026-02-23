package quality

import (
	"context"
	"testing"
)

func TestRunnerRunPassesWhenAllCommandsSucceed(t *testing.T) {
	t.Parallel()

	runner := NewRunner()
	report, err := runner.Run(context.Background(), "", []string{"echo ok", "true"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !report.Passed {
		t.Fatal("expected report to pass")
	}
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(report.Results))
	}
}

func TestRunnerRunFailsWhenAnyCommandFails(t *testing.T) {
	t.Parallel()

	runner := NewRunner()
	report, err := runner.Run(context.Background(), "", []string{"true", "exit 9"})
	if err != nil {
		t.Fatalf("expected no runner error for non-zero exit, got %v", err)
	}
	if report.Passed {
		t.Fatal("expected report to fail")
	}
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(report.Results))
	}
	if report.Results[1].ExitCode != 9 {
		t.Fatalf("expected second command exit code 9, got %d", report.Results[1].ExitCode)
	}
}

func TestRunnerRunReturnsErrorWhenNoCommandsConfigured(t *testing.T) {
	t.Parallel()

	runner := NewRunner()
	if _, err := runner.Run(context.Background(), "", nil); err == nil {
		t.Fatal("expected error when no commands are configured")
	}
}
