package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/EstebanForge/daedalus/internal/config"
	"github.com/EstebanForge/daedalus/internal/prd"
	"github.com/EstebanForge/daedalus/internal/project"
)

type tuiTheme struct {
	header        lipgloss.Style
	brand         lipgloss.Style
	meta          lipgloss.Style
	rule          lipgloss.Style
	tabs          lipgloss.Style
	tab           lipgloss.Style
	tabActive     lipgloss.Style
	tabNew        lipgloss.Style
	panel         lipgloss.Style
	panelTitle    lipgloss.Style
	panelBody     lipgloss.Style
	stateReady    lipgloss.Style
	stateRunning  lipgloss.Style
	statePaused   lipgloss.Style
	stateStopped  lipgloss.Style
	stateError    lipgloss.Style
	stateDone     lipgloss.Style
	activityMuted lipgloss.Style
	activityWarn  lipgloss.Style
	activityError lipgloss.Style
	shortcuts     lipgloss.Style
}

type tuiPalette struct {
	brand               string
	meta                string
	rule                string
	tabBorder           string
	tabActiveBorder     string
	tabActiveBackground string
	tabActiveForeground string
	tabNewBorder        string
	panelBorder         string
	panelBackground     string
	panelTitle          string
	panelBody           string
	stateReady          string
	stateRunning        string
	statePaused         string
	stateStopped        string
	stateError          string
	stateDone           string
	activityMuted       string
	activityWarn        string
	activityError       string
	shortcuts           string
}

type tuiRenderOptions struct {
	Width  int
	Height int
	ANSI   bool
	Clear  bool
	Writer io.Writer
}

func tuiPaletteForMode(mode string) tuiPalette {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "light":
		return tuiPalette{
			brand:               "#006A8E",
			meta:                "#5D677C",
			rule:                "#B7C1D5",
			tabBorder:           "#AAB5CA",
			tabActiveBorder:     "#0D7FA7",
			tabActiveBackground: "#E5EEFF",
			tabActiveForeground: "#16324F",
			tabNewBorder:        "#B6C0D2",
			panelBorder:         "#AEB8CC",
			panelBackground:     "#F7FAFF",
			panelTitle:          "#1F2F47",
			panelBody:           "#27364D",
			stateReady:          "#5D677C",
			stateRunning:        "#0D7FA7",
			statePaused:         "#996A00",
			stateStopped:        "#5D677C",
			stateError:          "#B42318",
			stateDone:           "#1A7F49",
			activityMuted:       "#5D677C",
			activityWarn:        "#996A00",
			activityError:       "#B42318",
			shortcuts:           "#4E5D76",
		}
	default:
		return tuiPalette{
			brand:               "#4FD1FF",
			meta:                "#94A3C2",
			rule:                "#3D496A",
			tabBorder:           "#4A5A7D",
			tabActiveBorder:     "#4FD1FF",
			tabActiveBackground: "#18213E",
			tabActiveForeground: "#D6E4FF",
			tabNewBorder:        "#64739A",
			panelBorder:         "#3D496A",
			panelBackground:     "#10162D",
			panelTitle:          "#DDE6FF",
			panelBody:           "#C9D5EE",
			stateReady:          "#94A3C2",
			stateRunning:        "#4FD1FF",
			statePaused:         "#F6C667",
			stateStopped:        "#94A3C2",
			stateError:          "#FF8A8A",
			stateDone:           "#63DCA6",
			activityMuted:       "#94A3C2",
			activityWarn:        "#F6C667",
			activityError:       "#FF8A8A",
			shortcuts:           "#8CA0C2",
		}
	}
}

func newTUITheme(renderer *lipgloss.Renderer, color bool, mode string) tuiTheme {
	withForeground := func(style lipgloss.Style, value string) lipgloss.Style {
		if !color {
			return style
		}
		return style.Foreground(lipgloss.Color(value))
	}
	withBackground := func(style lipgloss.Style, value string) lipgloss.Style {
		if !color {
			return style
		}
		return style.Background(lipgloss.Color(value))
	}
	withBorder := func(style lipgloss.Style, value string) lipgloss.Style {
		if !color {
			return style
		}
		return style.BorderForeground(lipgloss.Color(value))
	}
	palette := tuiPaletteForMode(mode)

	theme := tuiTheme{}
	theme.header = renderer.NewStyle().Bold(true)
	theme.brand = withForeground(renderer.NewStyle().Bold(true), palette.brand)
	theme.meta = withForeground(renderer.NewStyle(), palette.meta)
	theme.rule = withForeground(renderer.NewStyle(), palette.rule)
	theme.tabs = renderer.NewStyle()
	theme.tab = withBorder(renderer.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1), palette.tabBorder)
	theme.tabActive = withForeground(
		withBorder(
			withBackground(renderer.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Bold(true), palette.tabActiveBackground),
			palette.tabActiveBorder,
		),
		palette.tabActiveForeground,
	)
	theme.tabNew = withBorder(renderer.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1), palette.tabNewBorder)
	theme.panel = withBackground(withBorder(renderer.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1), palette.panelBorder), palette.panelBackground)
	theme.panelTitle = withForeground(renderer.NewStyle().Bold(true), palette.panelTitle)
	theme.panelBody = withForeground(renderer.NewStyle(), palette.panelBody)
	theme.stateReady = withForeground(renderer.NewStyle().Bold(true), palette.stateReady)
	theme.stateRunning = withForeground(renderer.NewStyle().Bold(true), palette.stateRunning)
	theme.statePaused = withForeground(renderer.NewStyle().Bold(true), palette.statePaused)
	theme.stateStopped = withForeground(renderer.NewStyle().Bold(true), palette.stateStopped)
	theme.stateError = withForeground(renderer.NewStyle().Bold(true), palette.stateError)
	theme.stateDone = withForeground(renderer.NewStyle().Bold(true), palette.stateDone)
	theme.activityMuted = withForeground(renderer.NewStyle().Padding(0, 1), palette.activityMuted)
	theme.activityWarn = withForeground(renderer.NewStyle().Padding(0, 1), palette.activityWarn)
	theme.activityError = withForeground(renderer.NewStyle().Padding(0, 1).Bold(true), palette.activityError)
	theme.shortcuts = withForeground(renderer.NewStyle().Padding(0, 1), palette.shortcuts)
	return theme
}

func (a App) renderStructuredTUIView(store prd.Store, cfg config.Config, baseDir string, state tuiSnapshot) {
	options := tuiRenderOptions{
		Width:  tuiEnvInt("COLUMNS", 120),
		Height: tuiEnvInt("LINES", 34),
		ANSI:   tuiShouldUseANSI(a.out),
		Clear:  !tuiBoolEnv("DAEDALUS_TUI_NO_CLEAR"),
		Writer: a.out,
	}
	screen := a.buildStructuredTUIView(store, cfg, baseDir, state, options)
	if options.ANSI && options.Clear {
		a.writef("\x1b[H\x1b[2J")
	}
	a.writeLine(screen)
}

func (a App) buildStructuredTUIView(
	store prd.Store,
	cfg config.Config,
	baseDir string,
	state tuiSnapshot,
	options tuiRenderOptions,
) string {
	width := options.Width
	if width < 96 {
		width = 96
	}
	height := options.Height
	if height < 22 {
		height = 22
	}

	renderWriter := options.Writer
	if renderWriter == nil {
		renderWriter = io.Discard
	}
	renderer := lipgloss.NewRenderer(renderWriter)
	theme := newTUITheme(renderer, options.ANSI, state.themeMode)

	summaries, summariesErr := store.List()
	selected := strings.TrimSpace(state.selectedPRD)
	if selected == "" && len(summaries) > 0 {
		selected = summaries[0].Name
	}

	var (
		resolvedName string
		doc          prd.Document
		docLoaded    bool
		docErr       error
	)
	if selected != "" {
		resolvedName, docErr = store.ResolveName(selected)
		if docErr == nil {
			doc, docErr = store.Load(resolvedName)
			if docErr == nil {
				docLoaded = true
			}
		}
	}
	if selected == "" && len(summaries) == 0 {
		docErr = fmt.Errorf("no PRDs found; run 'daedalus new <name>' first")
	}
	if summariesErr != nil {
		docErr = summariesErr
	}

	header := tuiRenderHeaderLine(theme, width, cfg, state)
	tabs := tuiRenderTabsLine(theme, width, summaries, selected)
	rule := theme.rule.Render(strings.Repeat("─", width))

	chromeHeight := 6
	contentHeight := height - chromeHeight
	if contentHeight < 10 {
		contentHeight = 10
	}

	leftWidth := width * 34 / 100
	if leftWidth < 30 {
		leftWidth = 30
	}
	if leftWidth > width-34 {
		leftWidth = width - 34
	}
	rightWidth := width - leftWidth - 1

	leftPanel := tuiRenderPanel(theme, "Stories", leftWidth, contentHeight, tuiStoriesPanelLines(state, doc, docLoaded, docErr))
	rightTitle, rightLines := tuiMainPanel(state, cfg, baseDir, summaries, resolvedName, doc, docLoaded, docErr)
	rightPanel := tuiRenderPanel(theme, rightTitle, rightWidth, contentHeight, rightLines)
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	activity := tuiRenderActivityLine(theme, width, state)
	shortcuts := theme.shortcuts.Width(width).Render(tuiShortcutLine(state.view))

	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, rule, content, rule, activity, shortcuts)
}

func tuiRenderHeaderLine(theme tuiTheme, width int, cfg config.Config, state tuiSnapshot) string {
	provider := strings.TrimSpace(state.provider)
	if provider == "" {
		provider = strings.TrimSpace(cfg.Provider.Default)
	}
	if provider == "" {
		provider = "unknown"
	}

	stateLabel := tuiStateBadge(theme, strings.TrimSpace(state.loopState))
	left := lipgloss.JoinHorizontal(lipgloss.Center, theme.brand.Render("daedalus"), " ", stateLabel)

	rightText := fmt.Sprintf("Agent: %s  Iteration: %d  Time: %s", provider, state.iterations, tuiFormatElapsed(state.startedAt))
	availableRight := width - lipgloss.Width(left) - 1
	if availableRight < 10 {
		availableRight = 10
	}
	rightText = tuiTrimToWidth(rightText, availableRight)
	right := theme.meta.Render(rightText)

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return theme.header.Width(width).Render(line)
}

func tuiRenderTabsLine(theme tuiTheme, width int, summaries []prd.Summary, selected string) string {
	chips := make([]string, 0, len(summaries)+2)
	for index, summary := range summaries {
		if index >= 9 {
			break
		}
		label := fmt.Sprintf("%d %s %d/%d", index+1, summary.Name, summary.Complete, summary.Total)
		if summary.Name == selected {
			chips = append(chips, theme.tabActive.Render(label))
			continue
		}
		chips = append(chips, theme.tab.Render(label))
	}
	if len(chips) == 0 {
		chips = append(chips, theme.tab.Render("none"))
	}
	chips = append(chips, theme.tabNew.Render("+ New"))

	prefix := "PRDs: "
	used := lipgloss.Width(prefix)
	row := prefix
	for i, chip := range chips {
		separator := ""
		separatorWidth := 0
		if i > 0 {
			separator = " "
			separatorWidth = 1
		}
		chipWidth := lipgloss.Width(chip)
		if used+separatorWidth+chipWidth > width {
			ellipsis := " ..."
			if used+lipgloss.Width(ellipsis) <= width {
				row += ellipsis
			}
			break
		}
		row += separator + chip
		used += separatorWidth + chipWidth
	}
	return theme.tabs.Width(width).Render(row)
}

func tuiRenderPanel(theme tuiTheme, title string, width, height int, lines []string) string {
	if width < 20 {
		width = 20
	}
	if height < 8 {
		height = 8
	}
	bodyWidth := max(10, width-4)
	bodyHeight := max(3, height-4)

	wrapped := tuiWrapLines(lines, bodyWidth)
	if len(wrapped) > bodyHeight {
		wrapped = wrapped[:bodyHeight]
		if bodyHeight > 0 {
			wrapped[bodyHeight-1] = tuiTrimToWidth(wrapped[bodyHeight-1], max(3, bodyWidth-3)) + "..."
		}
	}
	content := strings.Join(wrapped, "\n")

	inner := lipgloss.JoinVertical(
		lipgloss.Left,
		theme.panelTitle.Render("["+strings.TrimSpace(title)+"]"),
		theme.panelBody.Width(bodyWidth).Height(bodyHeight).Render(content),
	)
	return theme.panel.Width(width).Height(height).Render(inner)
}

func tuiRenderActivityLine(theme tuiTheme, width int, state tuiSnapshot) string {
	activity := tuiActivityLine(state)
	style := theme.activityMuted
	if strings.TrimSpace(state.lastError) != "" {
		style = theme.activityError
	} else if state.pauseRequested || state.stopRequested {
		style = theme.activityWarn
	}
	return style.Width(width).Render(activity)
}

func tuiStateBadge(theme tuiTheme, raw string) string {
	state := strings.ToLower(strings.TrimSpace(raw))
	if state == "" {
		state = "ready"
	}
	label := strings.ToUpper(state)

	style := theme.stateReady
	switch state {
	case "running":
		style = theme.stateRunning
	case "paused":
		style = theme.statePaused
	case "stopped":
		style = theme.stateStopped
	case "error":
		style = theme.stateError
	case "completed":
		style = theme.stateDone
	}
	return style.Render("[" + label + "]")
}

func tuiActivityLine(state tuiSnapshot) string {
	if strings.TrimSpace(state.lastError) != "" {
		return "Activity: Error - " + strings.TrimSpace(state.lastError)
	}
	if strings.TrimSpace(state.lastActivity) != "" {
		return "Activity: " + strings.TrimSpace(state.lastActivity)
	}
	if state.pauseRequested {
		return "Activity: Pause requested."
	}
	if state.stopRequested {
		return "Activity: Stop requested."
	}
	return "Activity: Ready."
}

func tuiShortcutLine(view string) string {
	switch strings.TrimSpace(strings.ToLower(view)) {
	case "logs":
		return "Keys: s start | p pause | x stop | t dashboard | d diff | l PRDs | j/k scroll | f filter | +/- tail | q quit"
	case "diff":
		return "Keys: d dashboard | t logs | l PRDs | j/k scroll | e edit | q quit"
	case "picker":
		return "Keys: j/k move | enter select | n new | 1-9 switch | esc close | q quit"
	case "settings":
		return "Keys: , close | [ ] provider | l PRDs | ? help | q quit"
	case "help":
		return "Keys: ? close | esc close | q quit"
	default:
		return "Keys: s start | p pause | x stop | t logs | d diff | e edit | n new | l PRDs | 1-9 switch | j/k stories | q quit"
	}
}

func tuiStoriesPanelLines(state tuiSnapshot, doc prd.Document, docLoaded bool, docErr error) []string {
	if docErr != nil {
		return []string{"Stories unavailable: " + docErr.Error()}
	}
	if !docLoaded {
		return []string{"No PRD selected."}
	}
	if len(doc.UserStories) == 0 {
		return []string{"No stories found in prd.json."}
	}

	selected, selectedIndex, _ := tuiSelectedStory(doc, state.storyIndex)

	lines := make([]string, 0, len(doc.UserStories)+4)
	total := len(doc.UserStories)
	complete := doc.CountComplete()
	lines = append(lines, fmt.Sprintf("Progress: %d/%d complete", complete, total))
	lines = append(lines, tuiProgressBar(complete, total, 24))
	lines = append(lines, "")

	for i, story := range doc.UserStories {
		prefix := " "
		if i == selectedIndex {
			prefix = ">"
		} else if selected != nil && story.ID == selected.ID {
			prefix = "•"
		}
		lines = append(lines, fmt.Sprintf("%s %s %s (P%d)", prefix, tuiStoryStatusBadge(story), story.ID, story.Priority))
		lines = append(lines, fmt.Sprintf("  %s", story.Title))
	}

	return lines
}

func tuiMainPanel(
	state tuiSnapshot,
	cfg config.Config,
	baseDir string,
	summaries []prd.Summary,
	resolvedName string,
	doc prd.Document,
	docLoaded bool,
	docErr error,
) (string, []string) {
	switch strings.TrimSpace(strings.ToLower(state.view)) {
	case "stories":
		return "Story Details", tuiStoryDetailsLines(state, doc, docLoaded, docErr)
	case "logs":
		return "Logs", tuiLogLines(baseDir, resolvedName, state, docErr)
	case "diff":
		return "Diff", tuiDiffLines(baseDir, resolvedName, state, docErr)
	case "picker":
		return "PRDs", tuiPickerLines(summaries, state)
	case "help":
		return "Help", tuiHelpLines(state)
	case "settings":
		return "Settings", tuiSettingsLines(cfg, state)
	default:
		return "Dashboard", tuiDashboardLines(baseDir, resolvedName, state, doc, docLoaded, docErr)
	}
}

func tuiDashboardLines(baseDir, prdName string, state tuiSnapshot, doc prd.Document, docLoaded bool, docErr error) []string {
	lines := []string{
		fmt.Sprintf("Loop: %s", strings.TrimSpace(state.loopState)),
		fmt.Sprintf("Iterations: %d", state.iterations),
		fmt.Sprintf("Provider: %s", strings.TrimSpace(state.provider)),
		fmt.Sprintf("Last run: %s", tuiFallbackText(strings.TrimSpace(state.lastRunAt), "not yet")),
		"",
	}

	if docErr != nil {
		return append(lines, "Status unavailable: "+docErr.Error())
	}
	if !docLoaded {
		return append(lines, "No PRD selected.")
	}

	lines = append(lines,
		fmt.Sprintf("Project: %s", doc.Project),
		fmt.Sprintf("Description: %s", strings.TrimSpace(doc.Description)),
	)
	if branch, dir := tuiWorktreeInfo(baseDir, prdName); branch != "" {
		lines = append(lines,
			fmt.Sprintf("Branch: %s", branch),
			fmt.Sprintf("Dir: %s", dir),
		)
	}

	selected, _, selectedID := tuiSelectedStory(doc, state.storyIndex)
	if selected == nil {
		lines = append(lines, "", "All stories are complete.")
		return lines
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Current Story: %s - %s", selectedID, selected.Title))
	lines = append(lines, fmt.Sprintf("Priority: %d", selected.Priority))
	lines = append(lines, "Description:")
	lines = append(lines, selected.Description)
	lines = append(lines, "")
	lines = append(lines, "Acceptance Criteria:")
	for _, criterion := range selected.AcceptanceCriteria {
		lines = append(lines, "- "+criterion)
	}

	return lines
}

func tuiStoryDetailsLines(state tuiSnapshot, doc prd.Document, docLoaded bool, docErr error) []string {
	if docErr != nil {
		return []string{"Stories unavailable: " + docErr.Error()}
	}
	if !docLoaded {
		return []string{"No PRD selected."}
	}
	if len(doc.UserStories) == 0 {
		return []string{"No stories found in prd.json."}
	}

	selected, selectedIndex, selectedID := tuiSelectedStory(doc, state.storyIndex)
	if selected == nil {
		return []string{"All stories are complete."}
	}

	lines := []string{
		fmt.Sprintf("Selected: %s (%d/%d)", selectedID, selectedIndex+1, len(doc.UserStories)),
		fmt.Sprintf("Status: %s", tuiStoryStatusBadge(*selected)),
		fmt.Sprintf("Priority: %d", selected.Priority),
		"",
		selected.Title,
		"",
		selected.Description,
		"",
		"Acceptance Criteria:",
	}
	for _, criterion := range selected.AcceptanceCriteria {
		lines = append(lines, "- "+criterion)
	}
	lines = append(lines, "", "Use j/k to change the selected story.")

	if state.pauseRequested {
		lines = append(lines, "Pause requested; current iteration will complete first.")
	}
	if state.stopRequested {
		lines = append(lines, "Stop requested; current iteration will complete first.")
	}
	return lines
}

func tuiLogLines(baseDir, name string, state tuiSnapshot, docErr error) []string {
	if docErr != nil {
		return []string{"Logs unavailable: " + docErr.Error()}
	}
	if strings.TrimSpace(name) == "" {
		return []string{"Logs unavailable: no PRD selected."}
	}

	eventsPath := project.PRDEventsPath(baseDir, name)
	entries, parseErr := readEventEntries(eventsPath)
	if parseErr == nil {
		filtered := filterEventEntries(entries, state.logFilter)
		lines := []string{fmt.Sprintf("Source: %s  filter=%s  tail=%d  offset=%d", eventsPath, state.logFilter, state.logTail, max(0, state.logOffset))}
		if len(filtered) == 0 {
			return append(lines, "No matching event entries.")
		}
		offset := max(0, state.logOffset)
		end := len(filtered) - offset
		if end < 0 {
			end = 0
		}
		start := end - state.logTail
		if start < 0 {
			start = 0
		}
		if end < start {
			end = start
		}
		for _, entry := range filtered[start:end] {
			lines = append(lines, fmt.Sprintf("%s  #%d  [%s] %s", entry.Timestamp, entry.Iteration, entry.Type, entry.Message))
		}
		lines = append(lines, "", "Use j/k to scroll log history.")
		return lines
	}

	agentLogPath := project.PRDAgentLogPath(baseDir, name)
	contents, err := os.ReadFile(agentLogPath)
	if err != nil {
		return []string{fmt.Sprintf("Logs unavailable: %v (event parse error: %v)", err, parseErr)}
	}
	text := strings.TrimSpace(string(contents))
	if text == "" {
		return []string{"No log entries yet."}
	}

	lines := strings.Split(text, "\n")
	start := 0
	if len(lines) > state.logTail {
		start = len(lines) - state.logTail
	}
	output := []string{fmt.Sprintf("Source: %s  fallback=agent.log  tail=%d", agentLogPath, state.logTail)}
	output = append(output, lines[start:]...)
	return output
}

func tuiPickerLines(summaries []prd.Summary, state tuiSnapshot) []string {
	lines := []string{
		"Select a PRD to focus on.",
		"",
	}
	if len(summaries) == 0 {
		return append(lines, "No PRDs found.", "", "Press n to create a PRD.")
	}

	index := state.pickerIndex
	if index < 0 {
		index = 0
	}
	if index > len(summaries)-1 {
		index = len(summaries) - 1
	}

	for i, summary := range summaries {
		cursor := " "
		if i == index {
			cursor = ">"
		}
		lines = append(lines, fmt.Sprintf("%s %d. %s  %d/%d complete", cursor, i+1, summary.Name, summary.Complete, summary.Total))
	}

	lines = append(lines, "", "Keys: j/k move, enter select, n new, esc close.")
	return lines
}

func tuiHelpLines(state tuiSnapshot) []string {
	view := strings.TrimSpace(strings.ToLower(state.previousView))
	if view == "" {
		view = "dashboard"
	}
	return []string{
		"Loop Control",
		"- s: start/resume loop",
		"- p: pause after current iteration",
		"- x: stop after current iteration",
		"",
		"Views",
		"- t: toggle dashboard/logs",
		"- d: toggle diff view",
		"- l: open PRD picker",
		"- ,: toggle settings",
		"- ?: toggle help",
		"",
		"PRD Navigation",
		"- 1-9: switch tab directly",
		"- tab/shift+tab: cycle PRDs",
		"- n: create a new PRD (auto name)",
		"- e: edit current prd.md",
		"",
		"Scrolling",
		"- j/k: move stories or scroll logs/diff",
		"- +/-: adjust log tail size",
		"",
		fmt.Sprintf("Current view before help: %s", view),
	}
}

func tuiDiffLines(baseDir, name string, state tuiSnapshot, docErr error) []string {
	if docErr != nil {
		return []string{"Diff unavailable: " + docErr.Error()}
	}
	if strings.TrimSpace(name) == "" {
		return []string{"Diff unavailable: no PRD selected."}
	}

	workDir, source := tuiResolveDiffWorkDir(baseDir, name)
	lines, err := tuiLoadLatestDiff(workDir)
	if err != nil {
		return []string{
			fmt.Sprintf("Diff source: %s", source),
			"Unable to load git diff: " + err.Error(),
		}
	}
	if len(lines) == 0 {
		return []string{
			fmt.Sprintf("Diff source: %s", source),
			"No commit diff available yet.",
		}
	}

	maxLines := len(lines)
	offset := max(0, state.diffOffset)
	if offset > maxLines-1 {
		offset = maxLines - 1
	}
	visible := lines[offset:]
	header := fmt.Sprintf("Diff source: %s  lines=%d  offset=%d", source, maxLines, offset)
	output := make([]string, 0, len(visible)+2)
	output = append(output, header, "")
	output = append(output, visible...)
	return output
}

func tuiResolveDiffWorkDir(baseDir, name string) (string, string) {
	worktreePath := project.WorktreePath(baseDir, name)
	if info, err := os.Stat(worktreePath); err == nil && info.IsDir() {
		return worktreePath, fmt.Sprintf("%s (worktree daedalus/%s)", worktreePath, name)
	}
	return baseDir, fmt.Sprintf("%s (current branch)", baseDir)
}

func tuiLoadLatestDiff(workDir string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "show", "--no-color", "--stat", "--patch", "--max-count=1")
	data, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s", message)
	}

	text := strings.TrimSpace(string(data))
	if text == "" {
		return []string{}, nil
	}
	lines := strings.Split(text, "\n")
	const limit = 400
	if len(lines) > limit {
		lines = append(lines[:limit], fmt.Sprintf("... truncated (%d more lines)", len(lines)-limit))
	}
	return lines, nil
}

func tuiSelectedStory(doc prd.Document, preferred int) (*prd.UserStory, int, string) {
	if len(doc.UserStories) == 0 {
		return nil, 0, ""
	}
	if preferred < 0 {
		preferred = 0
	}
	if preferred > len(doc.UserStories)-1 {
		preferred = len(doc.UserStories) - 1
	}
	story := &doc.UserStories[preferred]
	return story, preferred, story.ID
}

func tuiWorktreeInfo(baseDir, name string) (string, string) {
	if strings.TrimSpace(name) == "" {
		return "", ""
	}
	worktreePath := project.WorktreePath(baseDir, name)
	if info, err := os.Stat(worktreePath); err == nil && info.IsDir() {
		return "daedalus/" + name, ".daedalus/worktrees/" + name
	}
	return "", ""
}

func tuiSettingsLines(cfg config.Config, state tuiSnapshot) []string {
	provider := strings.TrimSpace(state.provider)
	if provider == "" {
		provider = strings.TrimSpace(cfg.Provider.Default)
	}
	configuredTheme := normalizeThemeMode(cfg.UI.Theme)
	if configuredTheme == "" {
		configuredTheme = "auto"
	}
	activeTheme := strings.TrimSpace(strings.ToLower(state.themeMode))
	if activeTheme == "" {
		activeTheme = "dark"
	}
	themeSource := "ui.theme"
	if envTheme := normalizeThemeMode(os.Getenv("DAEDALUS_THEME")); envTheme == "dark" || envTheme == "light" {
		themeSource = "DAEDALUS_THEME"
	}

	lines := []string{
		fmt.Sprintf("Default provider: %s", cfg.Provider.Default),
		fmt.Sprintf("Selected provider: %s", provider),
		fmt.Sprintf("Theme mode: %s", activeTheme),
		fmt.Sprintf("Theme source: %s (config=%s)", themeSource, configuredTheme),
		fmt.Sprintf("Worktree mode: %t", cfg.Worktree.Enabled),
		fmt.Sprintf("Retry max: %d", cfg.Retry.MaxRetries),
		fmt.Sprintf("Retry delays: %s", strings.Join(cfg.Retry.Delays, ", ")),
		fmt.Sprintf("Quality commands: %s", strings.Join(cfg.Quality.Commands, " ; ")),
		"",
		"Agent adapters:",
	}
	lines = append(lines, tuiProviderStatusLines(cfg, provider)...)
	return lines
}

func tuiProviderStatusLines(cfg config.Config, active string) []string {
	type adapter struct {
		name    string
		enabled bool
		model   string
	}
	adapters := []adapter{
		{name: "codex", enabled: cfg.Providers.Codex.Enabled, model: cfg.Providers.Codex.Model},
		{name: "claude", enabled: cfg.Providers.Claude.Enabled, model: cfg.Providers.Claude.Model},
		{name: "gemini", enabled: cfg.Providers.Gemini.Enabled, model: cfg.Providers.Gemini.Model},
		{name: "opencode", enabled: cfg.Providers.OpenCode.Enabled, model: cfg.Providers.OpenCode.Model},
		{name: "copilot", enabled: cfg.Providers.Copilot.Enabled, model: cfg.Providers.Copilot.Model},
		{name: "qwen", enabled: cfg.Providers.Qwen.Enabled, model: cfg.Providers.Qwen.Model},
		{name: "pi", enabled: cfg.Providers.Pi.Enabled, model: cfg.Providers.Pi.Model},
	}

	normalizedActive := strings.ToLower(strings.TrimSpace(active))
	knownActive := false
	lines := make([]string, 0, len(adapters)+1)
	for _, adapter := range adapters {
		status := "disabled"
		if adapter.enabled {
			status = "enabled"
		}
		model := strings.TrimSpace(adapter.model)
		if model == "" {
			model = "-"
		}
		marker := " "
		if adapter.name == normalizedActive {
			marker = "*"
			knownActive = true
		}
		lines = append(lines, fmt.Sprintf("%s %s (%s, model=%s)", marker, adapter.name, status, model))
	}
	if normalizedActive != "" && !knownActive {
		lines = append(lines, fmt.Sprintf("* %s (custom key; adapter not registered)", normalizedActive))
	}
	return lines
}

func tuiStoryStatusBadge(story prd.UserStory) string {
	if story.Passes {
		return "[passed]"
	}
	if story.InProgress {
		return "[in-progress]"
	}
	return "[pending]"
}

func tuiProgressBar(done, total, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("-", width) + "]"
	}
	if width < 10 {
		width = 10
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	filled := done * width / total
	if filled > width {
		filled = width
	}
	empty := width - filled
	return fmt.Sprintf("[%s%s] %d%%", strings.Repeat("#", filled), strings.Repeat("-", empty), (done*100)/total)
}

func tuiWrapLines(lines []string, width int) []string {
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, tuiWrapLine(strings.TrimSpace(line), width)...)
	}
	if len(wrapped) == 0 {
		return []string{""}
	}
	return wrapped
}

func tuiWrapLine(text string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	if text == "" {
		return []string{""}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	lines := make([]string, 0, len(words))
	current := ""
	for _, word := range words {
		if lipgloss.Width(word) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			for lipgloss.Width(word) > width {
				runes := []rune(word)
				cut := width
				if cut > len(runes) {
					cut = len(runes)
				}
				lines = append(lines, string(runes[:cut]))
				word = string(runes[cut:])
			}
			if word != "" {
				current = word
			}
			continue
		}
		if current == "" {
			current = word
			continue
		}
		candidate := current + " " + word
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func tuiTrimToWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 3 {
		runes := []rune(value)
		if width > len(runes) {
			width = len(runes)
		}
		return string(runes[:width])
	}
	runes := []rune(value)
	cut := width - 3
	if cut > len(runes) {
		cut = len(runes)
	}
	return string(runes[:cut]) + "..."
}

func tuiFallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func tuiFormatElapsed(startedAt time.Time) string {
	if startedAt.IsZero() {
		return "0s"
	}
	duration := time.Since(startedAt)
	if duration < 0 {
		duration = 0
	}
	seconds := int(duration.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	secondsRemainder := seconds % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm%02ds", minutes, secondsRemainder)
	}
	hours := minutes / 60
	minutesRemainder := minutes % 60
	return fmt.Sprintf("%dh%02dm", hours, minutesRemainder)
}

func tuiEnvInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func tuiBoolEnv(name string) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func tuiShouldUseANSI(writer io.Writer) bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
