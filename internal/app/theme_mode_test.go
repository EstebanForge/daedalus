package app

import (
	"testing"

	"github.com/EstebanForge/daedalus/internal/config"
)

func TestResolveTUIColorModePrefersEnvOverride(t *testing.T) {
	original := detectOSThemeModeFn
	detectOSThemeModeFn = func() (string, bool) {
		return "dark", true
	}
	t.Cleanup(func() { detectOSThemeModeFn = original })

	t.Setenv("DAEDALUS_THEME", "light")
	cfg := config.Defaults()
	cfg.UI.Theme = "dark"

	if got := resolveTUIColorMode(cfg); got != "light" {
		t.Fatalf("expected env override light, got %q", got)
	}
}

func TestResolveTUIColorModeUsesConfigTheme(t *testing.T) {
	original := detectOSThemeModeFn
	detectOSThemeModeFn = func() (string, bool) {
		return "dark", true
	}
	t.Cleanup(func() { detectOSThemeModeFn = original })

	t.Setenv("DAEDALUS_THEME", "")
	cfg := config.Defaults()
	cfg.UI.Theme = "light"

	if got := resolveTUIColorMode(cfg); got != "light" {
		t.Fatalf("expected config light, got %q", got)
	}
}

func TestResolveTUIColorModeUsesDetectedThemeWhenAuto(t *testing.T) {
	original := detectOSThemeModeFn
	detectOSThemeModeFn = func() (string, bool) {
		return "light", true
	}
	t.Cleanup(func() { detectOSThemeModeFn = original })

	t.Setenv("DAEDALUS_THEME", "")
	cfg := config.Defaults()
	cfg.UI.Theme = "auto"

	if got := resolveTUIColorMode(cfg); got != "light" {
		t.Fatalf("expected detected light, got %q", got)
	}
}

func TestResolveTUIColorModeFallsBackToDarkWhenDetectionFails(t *testing.T) {
	original := detectOSThemeModeFn
	detectOSThemeModeFn = func() (string, bool) {
		return "", false
	}
	t.Cleanup(func() { detectOSThemeModeFn = original })

	t.Setenv("DAEDALUS_THEME", "")
	cfg := config.Defaults()
	cfg.UI.Theme = "auto"

	if got := resolveTUIColorMode(cfg); got != "dark" {
		t.Fatalf("expected dark fallback, got %q", got)
	}
}
