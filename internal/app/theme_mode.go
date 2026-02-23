package app

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
)

var detectOSThemeModeFn = detectOSThemeMode
var runThemeProbeFn = runThemeProbe

func resolveTUIColorMode(cfg config.Config) string {
	if envTheme := normalizeThemeMode(os.Getenv("DAEDALUS_THEME")); envTheme == "dark" || envTheme == "light" {
		return envTheme
	}

	configuredTheme := normalizeThemeMode(cfg.UI.Theme)
	switch configuredTheme {
	case "dark", "light":
		return configuredTheme
	case "auto":
		if detected, ok := detectOSThemeModeFn(); ok {
			return detected
		}
		return "dark"
	default:
		if detected, ok := detectOSThemeModeFn(); ok {
			return detected
		}
		return "dark"
	}
}

func normalizeThemeMode(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "dark", "light", "auto":
		return normalized
	default:
		return ""
	}
}

func detectOSThemeMode() (string, bool) {
	switch runtime.GOOS {
	case "darwin":
		return detectDarwinThemeMode()
	case "windows":
		return detectWindowsThemeMode()
	default:
		return detectUnixThemeMode()
	}
}

func detectDarwinThemeMode() (string, bool) {
	output, err := runThemeProbeFn("defaults", "read", "-g", "AppleInterfaceStyle")
	if err != nil {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "does not exist") || strings.Contains(lower, "could not find") {
			return "light", true
		}
		return "", false
	}
	if strings.Contains(strings.ToLower(output), "dark") {
		return "dark", true
	}
	return "light", true
}

func detectWindowsThemeMode() (string, bool) {
	output, err := runThemeProbeFn("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "/v", "AppsUseLightTheme")
	if err != nil {
		return "", false
	}
	lower := strings.ToLower(output)
	if strings.Contains(lower, "0x0") {
		return "dark", true
	}
	if strings.Contains(lower, "0x1") {
		return "light", true
	}
	return "", false
}

func detectUnixThemeMode() (string, bool) {
	if gtkTheme := strings.ToLower(strings.TrimSpace(os.Getenv("GTK_THEME"))); gtkTheme != "" {
		if strings.Contains(gtkTheme, "dark") {
			return "dark", true
		}
		return "light", true
	}

	if kdeTheme := strings.ToLower(strings.TrimSpace(os.Getenv("KDE_COLOR_SCHEME"))); kdeTheme != "" {
		if strings.Contains(kdeTheme, "dark") {
			return "dark", true
		}
		return "light", true
	}

	if output, err := runThemeProbeFn("gsettings", "get", "org.gnome.desktop.interface", "color-scheme"); err == nil {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "prefer-dark") {
			return "dark", true
		}
		if strings.Contains(lower, "default") || strings.Contains(lower, "prefer-light") {
			return "light", true
		}
	}

	if output, err := runThemeProbeFn("gsettings", "get", "org.gnome.desktop.interface", "gtk-theme"); err == nil {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "dark") {
			return "dark", true
		}
		if strings.TrimSpace(lower) != "" {
			return "light", true
		}
	}

	return "", false
}

func runThemeProbe(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}
