package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReturnsDefaultsWhenFileDoesNotExist(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.toml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Provider.Default != "codex" {
		t.Fatalf("expected default provider codex, got %q", cfg.Provider.Default)
	}
	if len(cfg.Quality.Commands) == 0 {
		t.Fatal("expected default quality commands")
	}
	if cfg.Worktree.Enabled {
		t.Fatal("expected worktree mode disabled by default")
	}
	if cfg.UI.Theme != "auto" {
		t.Fatalf("expected ui.theme auto by default, got %q", cfg.UI.Theme)
	}
}

func TestValidateFailsOnInvalidQualityCommands(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.Quality.Commands = []string{"go test ./...", "   "}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "quality.commands") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolvePathUsesDAEDALUSConfigEnv(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("DAEDALUS_CONFIG", envPath)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))

	path, err := ResolvePath("")
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	if path != envPath {
		t.Fatalf("expected %q, got %q", envPath, path)
	}
}

func TestLoadAppliesFallbacksForQualityCommands(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := "[provider]\ndefault = \"codex\"\n\n[retry]\nmax_retries = 1\ndelays = [\"0s\"]\n\n[quality]\ncommands = []\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Quality.Commands) == 0 {
		t.Fatal("expected fallback quality command")
	}
}

func TestValidateFailsOnInvalidUITheme(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.UI.Theme = "night"
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "ui.theme") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAppliesFallbackForUITheme(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := "[provider]\ndefault = \"codex\"\n\n[retry]\nmax_retries = 1\ndelays = [\"0s\"]\n\n[quality]\ncommands = [\"go test ./...\"]\n\n[ui]\ntheme = \"\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.UI.Theme != "auto" {
		t.Fatalf("expected fallback ui.theme auto, got %q", cfg.UI.Theme)
	}
}

func TestCompletionDefaultsAreFalse(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	if cfg.Completion.PushOnComplete {
		t.Fatal("expected push_on_complete to default to false")
	}
	if cfg.Completion.AutoPROnComplete {
		t.Fatal("expected auto_pr_on_complete to default to false")
	}
}

func TestLoadAppliesCompletionFromTOML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := "[provider]\ndefault = \"codex\"\n\n[retry]\nmax_retries = 1\ndelays = [\"0s\"]\n\n[quality]\ncommands = [\"go test ./...\"]\n\n[completion]\npush_on_complete = true\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.Completion.PushOnComplete {
		t.Fatal("expected push_on_complete to be true")
	}
	if cfg.Completion.AutoPROnComplete {
		t.Fatal("expected auto_pr_on_complete to remain false")
	}
}

func TestValidateRejectsAutoPRWithoutPush(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.Completion.AutoPROnComplete = true
	cfg.Completion.PushOnComplete = false
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "auto_pr_on_complete") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadReadsProviderACPCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := "[provider]\ndefault = \"codex\"\n\n[retry]\nmax_retries = 1\ndelays = [\"0s\"]\n\n[quality]\ncommands = [\"go test ./...\"]\n\n[providers.codex]\nenabled = true\nacp_command = \"codex-acp\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Providers.Codex.ACPCommand != "codex-acp" {
		t.Fatalf("expected acp command to be loaded, got %q", cfg.Providers.Codex.ACPCommand)
	}
}

func TestPlanConfigDefaultsToEnabled(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	if !cfg.Plan.Enabled {
		t.Fatal("expected plan.enabled to default to true")
	}
}

func TestReviewConfigDefaultsToEnabled(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	if !cfg.Review.Enabled {
		t.Fatal("expected review.enabled to default to true")
	}
	if len(cfg.Review.Perspectives) != 3 {
		t.Fatalf("expected 3 review perspectives by default, got %d", len(cfg.Review.Perspectives))
	}
}

func TestCompoundConfigDefaultsToEnabled(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	if !cfg.Compound.Enabled {
		t.Fatal("expected compound.enabled to default to true")
	}
}

func TestValidateAcceptsCompoundEngineeringConfig(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Plan.Enabled = true
	cfg.Review.Enabled = true
	cfg.Review.Perspectives = []string{"security", "performance", "complexity"}
	cfg.Compound.Enabled = true
	err := Validate(cfg)
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}
