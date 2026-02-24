package app

import (
	"strings"
	"testing"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
	"github.com/EstebanForge/daedalus/internal/prd"
)

func TestNextGeneratedPRDNameSkipsExisting(t *testing.T) {
	t.Parallel()

	summaries := []prd.Summary{
		{Name: "main"},
		{Name: "prd-1"},
		{Name: "PRD-2"},
	}

	got := nextGeneratedPRDName(summaries)
	if got != "prd-3" {
		t.Fatalf("expected prd-3, got %q", got)
	}
}

func TestTuiSelectedStoryClampsIndex(t *testing.T) {
	t.Parallel()

	doc := prd.Document{
		UserStories: []prd.UserStory{
			{ID: "US-001", Title: "One"},
			{ID: "US-002", Title: "Two"},
		},
	}

	story, index, storyID := tuiSelectedStory(doc, 99)
	if story == nil {
		t.Fatal("expected selected story, got nil")
	}
	if index != 1 {
		t.Fatalf("expected index 1, got %d", index)
	}
	if storyID != "US-002" {
		t.Fatalf("expected story id US-002, got %q", storyID)
	}

	story, index, storyID = tuiSelectedStory(doc, -10)
	if story == nil {
		t.Fatal("expected selected story, got nil")
	}
	if index != 0 {
		t.Fatalf("expected index 0, got %d", index)
	}
	if storyID != "US-001" {
		t.Fatalf("expected story id US-001, got %q", storyID)
	}
}

// ── tuiProgressBar ────────────────────────────────────────────────────────────

func TestTuiProgressBarZeroTotal(t *testing.T) {
	t.Parallel()
	got := tuiProgressBar(0, 0, 10)
	if !strings.HasPrefix(got, "[") || !strings.Contains(got, "-") {
		t.Fatalf("unexpected output for zero total: %q", got)
	}
}

func TestTuiProgressBarFullCompletion(t *testing.T) {
	t.Parallel()
	got := tuiProgressBar(5, 5, 10)
	if !strings.Contains(got, "100%") {
		t.Fatalf("expected 100%%, got %q", got)
	}
	if strings.Contains(got, "-") {
		t.Fatalf("expected no empty slots, got %q", got)
	}
}

func TestTuiProgressBarHalf(t *testing.T) {
	t.Parallel()
	got := tuiProgressBar(5, 10, 20)
	if !strings.Contains(got, "50%") {
		t.Fatalf("expected 50%%, got %q", got)
	}
}

func TestTuiProgressBarClampsDoneAboveTotal(t *testing.T) {
	t.Parallel()
	got := tuiProgressBar(20, 10, 10)
	if !strings.Contains(got, "100%") {
		t.Fatalf("expected 100%%, got %q", got)
	}
}

func TestTuiProgressBarClampsNegativeDone(t *testing.T) {
	t.Parallel()
	got := tuiProgressBar(-3, 10, 10)
	if !strings.Contains(got, "0%") {
		t.Fatalf("expected 0%%, got %q", got)
	}
}

func TestTuiProgressBarMinWidth(t *testing.T) {
	t.Parallel()
	got := tuiProgressBar(0, 10, 0)
	if len(got) == 0 {
		t.Fatal("expected non-empty output for zero width")
	}
}

// ── tuiWrapLine ───────────────────────────────────────────────────────────────

func TestTuiWrapLineEmpty(t *testing.T) {
	t.Parallel()
	got := tuiWrapLine("", 40)
	if len(got) != 1 || got[0] != "" {
		t.Fatalf("expected single empty string, got %v", got)
	}
}

func TestTuiWrapLineZeroWidth(t *testing.T) {
	t.Parallel()
	got := tuiWrapLine("hello world", 0)
	if len(got) != 1 || got[0] != "" {
		t.Fatalf("expected single empty string for zero width, got %v", got)
	}
}

func TestTuiWrapLineShortFitsOnOne(t *testing.T) {
	t.Parallel()
	got := tuiWrapLine("hello", 40)
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("unexpected result: %v", got)
	}
}

func TestTuiWrapLineWrapsAtWordBoundary(t *testing.T) {
	t.Parallel()
	got := tuiWrapLine("hello world foo", 6)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 lines, got %v", got)
	}
	for _, line := range got {
		if len(line) > 6 {
			t.Fatalf("line %q exceeds width 6", line)
		}
	}
}

func TestTuiWrapLineLongWordSplits(t *testing.T) {
	t.Parallel()
	got := tuiWrapLine("abcdefghij", 4)
	// "abcdefghij" with width=4 should split into ["abcd", "efgh", "ij"]
	if len(got) < 2 {
		t.Fatalf("expected long word to be split, got %v", got)
	}
}

// ── tuiWrapLines ─────────────────────────────────────────────────────────────

func TestTuiWrapLinesEmpty(t *testing.T) {
	t.Parallel()
	got := tuiWrapLines(nil, 40)
	if len(got) == 0 {
		t.Fatal("expected at least one line for empty input")
	}
}

func TestTuiWrapLinesPreservesCount(t *testing.T) {
	t.Parallel()
	lines := []string{"short", "also short"}
	got := tuiWrapLines(lines, 80)
	if len(got) < len(lines) {
		t.Fatalf("expected at least %d lines, got %d", len(lines), len(got))
	}
}

func TestTuiWrapLinesExpandsLong(t *testing.T) {
	t.Parallel()
	lines := []string{"word1 word2 word3"}
	got := tuiWrapLines(lines, 5)
	if len(got) < 3 {
		t.Fatalf("expected at least 3 lines for tight width, got %v", got)
	}
}

// ── tuiTrimToWidth ────────────────────────────────────────────────────────────

func TestTuiTrimToWidthShortPassthrough(t *testing.T) {
	t.Parallel()
	got := tuiTrimToWidth("hello", 40)
	if got != "hello" {
		t.Fatalf("expected passthrough, got %q", got)
	}
}

func TestTuiTrimToWidthZeroWidth(t *testing.T) {
	t.Parallel()
	got := tuiTrimToWidth("hello", 0)
	if got != "" {
		t.Fatalf("expected empty for zero width, got %q", got)
	}
}

func TestTuiTrimToWidthTruncatesWithEllipsis(t *testing.T) {
	t.Parallel()
	got := tuiTrimToWidth("hello world", 8)
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
	if len([]rune(got)) > 8 {
		t.Fatalf("result %q exceeds width 8", got)
	}
}

func TestTuiTrimToWidthSmallWidth(t *testing.T) {
	t.Parallel()
	// width <= 3: truncate without ellipsis
	got := tuiTrimToWidth("hello", 2)
	if len([]rune(got)) > 2 {
		t.Fatalf("result %q exceeds width 2", got)
	}
}

// ── tuiFormatElapsed ──────────────────────────────────────────────────────────

func TestTuiFormatElapsedZeroTime(t *testing.T) {
	t.Parallel()
	got := tuiFormatElapsed(time.Time{})
	if got != "0s" {
		t.Fatalf("expected 0s for zero time, got %q", got)
	}
}

func TestTuiFormatElapsedSeconds(t *testing.T) {
	t.Parallel()
	got := tuiFormatElapsed(time.Now().Add(-30 * time.Second))
	if !strings.HasSuffix(got, "s") {
		t.Fatalf("expected seconds suffix, got %q", got)
	}
}

func TestTuiFormatElapsedMinutes(t *testing.T) {
	t.Parallel()
	got := tuiFormatElapsed(time.Now().Add(-90 * time.Second))
	if !strings.Contains(got, "m") {
		t.Fatalf("expected minutes in format, got %q", got)
	}
}

func TestTuiFormatElapsedHours(t *testing.T) {
	t.Parallel()
	got := tuiFormatElapsed(time.Now().Add(-2 * time.Hour))
	if !strings.Contains(got, "h") {
		t.Fatalf("expected hours in format, got %q", got)
	}
}

// ── tuiFallbackText ───────────────────────────────────────────────────────────

func TestTuiFallbackTextNonEmpty(t *testing.T) {
	t.Parallel()
	got := tuiFallbackText("actual", "fallback")
	if got != "actual" {
		t.Fatalf("expected actual value, got %q", got)
	}
}

func TestTuiFallbackTextEmpty(t *testing.T) {
	t.Parallel()
	got := tuiFallbackText("   ", "fallback")
	if got != "fallback" {
		t.Fatalf("expected fallback for blank input, got %q", got)
	}
}

func TestTuiFallbackTextEmptyString(t *testing.T) {
	t.Parallel()
	got := tuiFallbackText("", "default")
	if got != "default" {
		t.Fatalf("expected default, got %q", got)
	}
}

// ── tuiActivityLine ───────────────────────────────────────────────────────────

func TestTuiActivityLineError(t *testing.T) {
	t.Parallel()
	state := tuiSnapshot{lastError: "something broke"}
	got := tuiActivityLine(state)
	if !strings.Contains(got, "Error") {
		t.Fatalf("expected Error in activity line, got %q", got)
	}
	if !strings.Contains(got, "something broke") {
		t.Fatalf("expected error message in activity line, got %q", got)
	}
}

func TestTuiActivityLineActivity(t *testing.T) {
	t.Parallel()
	state := tuiSnapshot{lastActivity: "running story"}
	got := tuiActivityLine(state)
	if !strings.Contains(got, "running story") {
		t.Fatalf("expected activity text, got %q", got)
	}
}

func TestTuiActivityLinePauseRequested(t *testing.T) {
	t.Parallel()
	state := tuiSnapshot{pauseRequested: true}
	got := tuiActivityLine(state)
	if !strings.Contains(got, "Pause") {
		t.Fatalf("expected Pause in activity line, got %q", got)
	}
}

func TestTuiActivityLineStopRequested(t *testing.T) {
	t.Parallel()
	state := tuiSnapshot{stopRequested: true}
	got := tuiActivityLine(state)
	if !strings.Contains(got, "Stop") {
		t.Fatalf("expected Stop in activity line, got %q", got)
	}
}

func TestTuiActivityLineReady(t *testing.T) {
	t.Parallel()
	state := tuiSnapshot{}
	got := tuiActivityLine(state)
	if !strings.Contains(got, "Ready") {
		t.Fatalf("expected Ready in activity line, got %q", got)
	}
}

// ── tuiShortcutLine ───────────────────────────────────────────────────────────

func TestTuiShortcutLineKnownViews(t *testing.T) {
	t.Parallel()
	views := []string{"logs", "diff", "picker", "settings", "help", "dashboard", ""}
	for _, view := range views {
		got := tuiShortcutLine(view)
		if strings.TrimSpace(got) == "" {
			t.Fatalf("expected non-empty shortcut line for view %q", view)
		}
	}
}

func TestTuiShortcutLineLogsContainsFilter(t *testing.T) {
	t.Parallel()
	got := tuiShortcutLine("logs")
	if !strings.Contains(got, "filter") {
		t.Fatalf("expected filter key in logs shortcuts, got %q", got)
	}
}

func TestTuiShortcutLineSettingsContainsProvider(t *testing.T) {
	t.Parallel()
	got := tuiShortcutLine("settings")
	if !strings.Contains(got, "provider") {
		t.Fatalf("expected provider key in settings shortcuts, got %q", got)
	}
}

// ── tuiStoryStatusBadge ───────────────────────────────────────────────────────

func TestTuiStoryStatusBadgePasses(t *testing.T) {
	t.Parallel()
	story := prd.UserStory{Passes: true}
	got := tuiStoryStatusBadge(story)
	if got != "[passed]" {
		t.Fatalf("expected [passed], got %q", got)
	}
}

func TestTuiStoryStatusBadgeInProgress(t *testing.T) {
	t.Parallel()
	story := prd.UserStory{InProgress: true}
	got := tuiStoryStatusBadge(story)
	if got != "[in-progress]" {
		t.Fatalf("expected [in-progress], got %q", got)
	}
}

func TestTuiStoryStatusBadgePending(t *testing.T) {
	t.Parallel()
	story := prd.UserStory{}
	got := tuiStoryStatusBadge(story)
	if got != "[pending]" {
		t.Fatalf("expected [pending], got %q", got)
	}
}

func TestTuiStoryStatusBadgePassesTakesPrecedence(t *testing.T) {
	t.Parallel()
	// passes=true and inProgress=true: passes wins
	story := prd.UserStory{Passes: true, InProgress: true}
	got := tuiStoryStatusBadge(story)
	if got != "[passed]" {
		t.Fatalf("expected [passed] to take precedence, got %q", got)
	}
}

// ── tuiProviderStatusLines ────────────────────────────────────────────────────

func TestTuiProviderStatusLinesAllSevenProviders(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Claude.Enabled = true
	cfg.Providers.Gemini.Enabled = true
	cfg.Providers.OpenCode.Enabled = true
	cfg.Providers.Copilot.Enabled = true
	cfg.Providers.Qwen.Enabled = true
	cfg.Providers.Pi.Enabled = true

	lines := tuiProviderStatusLines(cfg, "codex")

	wantNames := []string{"codex", "claude", "gemini", "opencode", "copilot", "qwen", "pi"}
	for _, name := range wantNames {
		found := false
		for _, line := range lines {
			if strings.Contains(line, name) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("provider %q not found in status lines: %v", name, lines)
		}
	}
}

func TestTuiProviderStatusLinesActiveMarked(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	lines := tuiProviderStatusLines(cfg, "gemini")

	var geminiLine string
	for _, line := range lines {
		if strings.Contains(line, "gemini") {
			geminiLine = line
			break
		}
	}
	if geminiLine == "" {
		t.Fatal("gemini not found in status lines")
	}
	if !strings.HasPrefix(geminiLine, "*") {
		t.Fatalf("expected gemini line to be marked with *, got %q", geminiLine)
	}
}

func TestTuiProviderStatusLinesInactiveNotMarked(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	lines := tuiProviderStatusLines(cfg, "codex")

	for _, line := range lines {
		if strings.Contains(line, "gemini") {
			if strings.HasPrefix(line, "*") {
				t.Fatalf("expected gemini not to be marked when codex is active, got %q", line)
			}
		}
	}
}

func TestTuiProviderStatusLinesCustomProviderShownAsUnknown(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	lines := tuiProviderStatusLines(cfg, "myprovider")

	found := false
	for _, line := range lines {
		if strings.Contains(line, "myprovider") {
			found = true
			if !strings.Contains(line, "custom") {
				t.Fatalf("expected 'custom' label for unknown provider, got %q", line)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected custom provider line to be present")
	}
}

func TestTuiProviderStatusLinesDisabledShown(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.Providers.Pi.Enabled = false
	lines := tuiProviderStatusLines(cfg, "codex")

	for _, line := range lines {
		if strings.Contains(line, "pi") {
			if !strings.Contains(line, "disabled") {
				t.Fatalf("expected pi to show as disabled, got %q", line)
			}
			return
		}
	}
	t.Fatal("pi not found in status lines")
}

func TestTuiProviderStatusLinesEnabledShown(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.Providers.Qwen.Enabled = true
	lines := tuiProviderStatusLines(cfg, "codex")

	for _, line := range lines {
		if strings.Contains(line, "qwen") {
			if !strings.Contains(line, "enabled") {
				t.Fatalf("expected qwen to show as enabled, got %q", line)
			}
			return
		}
	}
	t.Fatal("qwen not found in status lines")
}

func TestTuiProviderStatusLinesModelShown(t *testing.T) {
	t.Parallel()
	cfg := config.Defaults()
	cfg.Providers.Gemini.Model = "gemini-2.0-flash"
	lines := tuiProviderStatusLines(cfg, "codex")

	for _, line := range lines {
		if strings.Contains(line, "gemini") {
			if !strings.Contains(line, "gemini-2.0-flash") {
				t.Fatalf("expected model in gemini line, got %q", line)
			}
			return
		}
	}
	t.Fatal("gemini not found in status lines")
}
