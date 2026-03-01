package providers

import (
	"context"
	"strings"
	"testing"

	"github.com/EstebanForge/daedalus/internal/config"
)

func TestRunACPDoctorHealthyProvider(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Cleanup(CloseAllSessions)

	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Codex.ACPCommand = helperACPCommand("acp-helper")

	report, err := RunACPDoctor(context.Background(), cfg, []string{"codex"})
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	if len(report.Checks) != 1 {
		t.Fatalf("expected one doctor check, got %d", len(report.Checks))
	}

	check := report.Checks[0]
	if !check.Healthy {
		t.Fatalf("expected healthy check, got %+v", check)
	}
	if !strings.Contains(check.Command, "-test.run=TestACPHelperProcess") {
		t.Fatalf("unexpected command: %q", check.Command)
	}
	if !report.Healthy() {
		t.Fatal("expected healthy report")
	}
}

func TestRunACPDoctorMissingBinary(t *testing.T) {
	t.Parallel()
	t.Cleanup(CloseAllSessions)

	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Codex.ACPCommand = "definitely-missing-acp-binary"

	report, err := RunACPDoctor(context.Background(), cfg, []string{"codex"})
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}
	if len(report.Checks) != 1 {
		t.Fatalf("expected one doctor check, got %d", len(report.Checks))
	}
	check := report.Checks[0]
	if check.Healthy {
		t.Fatalf("expected unhealthy check, got %+v", check)
	}
	if !strings.Contains(check.Message, "not found in PATH") {
		t.Fatalf("expected PATH error message, got %q", check.Message)
	}
	if report.Healthy() {
		t.Fatal("expected unhealthy report")
	}
}

func TestRunACPDoctorRejectsUnknownTarget(t *testing.T) {
	t.Parallel()

	_, err := RunACPDoctor(context.Background(), config.Defaults(), []string{"unknown"})
	if err == nil {
		t.Fatal("expected unknown provider error")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}
