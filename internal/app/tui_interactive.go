package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/EstebanForge/daedalus/internal/config"
	"github.com/EstebanForge/daedalus/internal/prd"
	"github.com/EstebanForge/daedalus/internal/project"
)

type tuiRefreshMsg struct{}
type editorFinishedMsg struct {
	err error
}

type interactiveTUIModel struct {
	ctx        context.Context
	app        App
	store      prd.Store
	cfg        config.Config
	global     globalOptions
	baseDir    string
	state      *tuiState
	controller *loopController
	width      int
	height     int
}

func (a App) runInteractiveTUI(
	ctx context.Context,
	store prd.Store,
	cfg config.Config,
	global globalOptions,
	baseDir string,
	state *tuiState,
	controller *loopController,
) error {
	model := interactiveTUIModel{
		ctx:        ctx,
		app:        a,
		store:      store,
		cfg:        cfg,
		global:     global,
		baseDir:    baseDir,
		state:      state,
		controller: controller,
	}

	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithInput(a.in), tea.WithOutput(a.out))
	_, err := program.Run()
	return err
}

func shouldUseInteractiveTUI(in io.Reader, out io.Writer) bool {
	inputFile, inputOK := in.(*os.File)
	outputFile, outputOK := out.(*os.File)
	if !inputOK || !outputOK {
		return false
	}
	inInfo, inErr := inputFile.Stat()
	outInfo, outErr := outputFile.Stat()
	if inErr != nil || outErr != nil {
		return false
	}
	if inInfo.Mode()&os.ModeCharDevice == 0 || outInfo.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	if strings.TrimSpace(os.Getenv("DAEDALUS_TUI_FORCE_COMMAND")) != "" {
		return false
	}
	return true
}

func (m interactiveTUIModel) Init() tea.Cmd {
	return tuiRefreshCmd()
}

func (m interactiveTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch current := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(current)
	case tuiRefreshMsg:
		if m.ctx.Err() != nil {
			m.state.setActivity("Context cancelled.")
			m.state.setStopRequested(true)
			m.controller.requestStopNow()
			return m, tea.Quit
		}
		return m, tuiRefreshCmd()
	case tea.WindowSizeMsg:
		m.width = current.Width
		m.height = current.Height
		return m, nil
	case editorFinishedMsg:
		if current.err != nil {
			m.state.setActivity("Edit command failed: " + current.err.Error())
		} else {
			m.state.setActivity("PRD editor closed.")
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m interactiveTUIModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	snap := m.state.snapshot()
	view := strings.TrimSpace(strings.ToLower(snap.view))

	if view == "help" {
		switch key {
		case "?", "esc", "q", "ctrl+c":
		default:
			return m, nil
		}
	}

	if view == "settings" {
		switch key {
		case ",", "esc", "[", "]", "q", "ctrl+c", "?":
		default:
			return m, nil
		}
	}

	if view == "picker" {
		switch key {
		case "j", "k", "up", "down", "n", "enter", "esc", "l", "1", "2", "3", "4", "5", "6", "7", "8", "9", "q", "ctrl+c", "?":
		default:
			return m, nil
		}
	}

	switch key {
	case "ctrl+c", "q":
		m.state.setStopRequested(true)
		m.controller.requestStopNow()
		m.state.setActivity("Exiting TUI.")
		return m, tea.Quit
	case "esc":
		switch snap.view {
		case "help", "settings", "picker":
			m.state.closeOverlayView()
			m.state.setActivity("Returned to " + m.state.snapshot().view + " view.")
			return m, nil
		case "diff":
			m.state.setView("dashboard")
			m.state.setActivity("Dashboard view.")
			return m, nil
		}
	case "s":
		runCtx, cancelRun := context.WithCancel(m.ctx)
		if !m.controller.start(cancelRun) {
			cancelRun()
			m.state.setActivity("Loop is already running.")
			return m, nil
		}
		m.state.setError(nil)
		m.state.setPauseRequested(false)
		m.state.setStopRequested(false)
		m.state.setLoopState("running")
		m.state.setActivity("Loop started.")
		m.app.logTUIRuntimeAction(m.store, m.baseDir, m.state.snapshot(), "tui start requested")
		workerApp := m.app
		workerApp.out = io.Discard
		go workerApp.runTUILoopWorker(runCtx, m.store, m.cfg, m.global, m.baseDir, m.state, m.controller)
		return m, nil
	case "p":
		m.state.setPauseRequested(true)
		m.controller.requestPause()
		m.state.setActivity("Pause requested; waiting for current iteration.")
		m.app.logTUIRuntimeAction(m.store, m.baseDir, m.state.snapshot(), "tui pause requested")
		return m, nil
	case "x":
		m.state.setStopRequested(true)
		m.controller.requestStop()
		m.state.setActivity("Stop requested; waiting for current iteration.")
		m.app.logTUIRuntimeAction(m.store, m.baseDir, m.state.snapshot(), "tui stop requested")
		return m, nil
	case "X":
		m.state.setStopRequested(true)
		m.controller.requestStopNow()
		m.state.setActivity("Immediate stop requested.")
		m.app.logTUIRuntimeAction(m.store, m.baseDir, m.state.snapshot(), "tui stop-now requested")
		return m, nil
	case "d":
		if snap.view == "diff" {
			m.state.setView("dashboard")
			m.state.setActivity("Dashboard view.")
			return m, nil
		}
		m.state.setView("diff")
		m.state.setActivity("Diff view.")
		return m, nil
	case "u":
		m.state.setView("stories")
		m.state.setActivity("Stories view.")
		return m, nil
	case "l":
		if snap.view == "picker" {
			m.state.closeOverlayView()
			m.state.setActivity("Returned to " + m.state.snapshot().view + " view.")
		} else {
			m.state.setViewWithHistory("picker")
			m.state.setActivity("PRD picker view.")
		}
		return m, nil
	case ",":
		if snap.view == "settings" {
			m.state.closeOverlayView()
			m.state.setActivity("Returned to " + m.state.snapshot().view + " view.")
		} else {
			m.state.setViewWithHistory("settings")
			m.state.setActivity("Settings view.")
		}
		return m, nil
	case "t":
		switch snap.view {
		case "dashboard":
			m.state.setView("logs")
			m.state.setActivity("Logs view.")
		case "logs":
			m.state.setView("dashboard")
			m.state.setActivity("Dashboard view.")
		default:
			m.state.setView("logs")
			m.state.setActivity("Logs view.")
		}
		return m, nil
	case "n":
		if snap.view == "picker" {
			m.createPRDFromPicker()
		} else {
			m.createPRDWithAutoName()
		}
		return m, nil
	case "tab":
		m.cyclePRD(1)
		return m, nil
	case "shift+tab":
		m.cyclePRD(-1)
		return m, nil
	case "enter":
		if snap.view == "picker" {
			m.selectPickerPRD()
			return m, nil
		}
	case "]":
		m.cycleProvider(1)
		return m, nil
	case "[":
		m.cycleProvider(-1)
		return m, nil
	case "f":
		m.cycleLogFilter()
		return m, nil
	case "?":
		if snap.view == "help" {
			m.state.closeOverlayView()
			m.state.setActivity("Returned to " + m.state.snapshot().view + " view.")
		} else {
			m.state.setViewWithHistory("help")
			m.state.setActivity("Help view.")
		}
		return m, nil
	case "e":
		return m.launchEditor()
	case "+", "=":
		m.state.setLogTail(snap.logTail + 1)
		m.state.setActivity("Log tail set to " + strconv.Itoa(m.state.snapshot().logTail) + ".")
		return m, nil
	case "-", "_":
		m.state.setLogTail(snap.logTail - 1)
		m.state.setActivity("Log tail set to " + strconv.Itoa(m.state.snapshot().logTail) + ".")
		return m, nil
	case "up", "k":
		return m.handleUp()
	case "down", "j":
		return m.handleDown()
	}

	if tabIndex, ok := parseTUITabShortcut(key); ok {
		m.switchToTab(tabIndex)
		return m, nil
	}

	return m, nil
}

func (m interactiveTUIModel) handleUp() (tea.Model, tea.Cmd) {
	snap := m.state.snapshot()
	switch strings.TrimSpace(strings.ToLower(snap.view)) {
	case "logs":
		m.state.moveLogOffset(1)
		return m, nil
	case "diff":
		m.state.moveDiffOffset(-1)
		return m, nil
	case "picker":
		summaries, err := m.store.List()
		if err != nil {
			m.state.setActivity("Unable to load PRDs.")
			return m, nil
		}
		m.state.movePickerIndex(-1, len(summaries))
		return m, nil
	default:
		m.state.moveStoryIndex(-1, m.currentStoryCount())
		return m, nil
	}
}

func (m interactiveTUIModel) handleDown() (tea.Model, tea.Cmd) {
	snap := m.state.snapshot()
	switch strings.TrimSpace(strings.ToLower(snap.view)) {
	case "logs":
		m.state.moveLogOffset(-1)
		return m, nil
	case "diff":
		m.state.moveDiffOffset(1)
		return m, nil
	case "picker":
		summaries, err := m.store.List()
		if err != nil {
			m.state.setActivity("Unable to load PRDs.")
			return m, nil
		}
		m.state.movePickerIndex(1, len(summaries))
		return m, nil
	default:
		m.state.moveStoryIndex(1, m.currentStoryCount())
		return m, nil
	}
}

func (m interactiveTUIModel) launchEditor() (tea.Model, tea.Cmd) {
	selected := strings.TrimSpace(m.state.snapshot().selectedPRD)
	resolved, err := m.store.ResolveName(selected)
	if err != nil {
		m.state.setActivity("Unable to resolve PRD for edit.")
		return m, nil
	}

	path := project.PRDMarkdownPath(m.baseDir, resolved)
	if _, err := os.Stat(path); err != nil {
		m.state.setActivity("PRD markdown file not found.")
		return m, nil
	}

	command, editorArgs := resolveEditorCommand()
	editorArgs = append(editorArgs, path)
	cmd := exec.Command(command, editorArgs...)
	cmd.Dir = m.baseDir

	m.state.setActivity("Opening PRD editor for " + resolved + ".")
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m interactiveTUIModel) currentStoryCount() int {
	selected := strings.TrimSpace(m.state.snapshot().selectedPRD)
	if selected == "" {
		return 0
	}
	name, err := m.store.ResolveName(selected)
	if err != nil {
		return 0
	}
	doc, err := m.store.Load(name)
	if err != nil {
		return 0
	}
	return len(doc.UserStories)
}

func (m interactiveTUIModel) selectPickerPRD() {
	summaries, err := m.store.List()
	if err != nil {
		m.state.setActivity("Unable to load PRDs.")
		return
	}
	if len(summaries) == 0 {
		m.state.setActivity("No PRDs found.")
		return
	}

	index := m.state.snapshot().pickerIndex
	if index < 0 {
		index = 0
	}
	if index > len(summaries)-1 {
		index = len(summaries) - 1
	}

	name := summaries[index].Name
	m.state.setSelectedPRD(name)
	m.state.closeOverlayView()
	m.state.setActivity("Switched to PRD " + name + ".")
}

func (m interactiveTUIModel) createPRDFromPicker() {
	m.createPRDWithAutoName()
}

func (m interactiveTUIModel) createPRDWithAutoName() {
	summaries, err := m.store.List()
	if err != nil {
		m.state.setActivity("Unable to load PRDs.")
		return
	}

	name := nextGeneratedPRDName(summaries)
	if err := m.store.Create(name); err != nil {
		m.state.setActivity("Failed to create PRD: " + err.Error())
		return
	}

	m.state.setSelectedPRD(name)
	m.state.setView("dashboard")
	m.state.setActivity("Created PRD " + name + ".")
}

func nextGeneratedPRDName(summaries []prd.Summary) string {
	used := make(map[string]struct{}, len(summaries))
	for _, summary := range summaries {
		used[strings.ToLower(strings.TrimSpace(summary.Name))] = struct{}{}
	}

	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("prd-%d", i)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
	return fmt.Sprintf("prd-%d", time.Now().Unix())
}

func (m interactiveTUIModel) View() string {
	ansi := strings.TrimSpace(os.Getenv("NO_COLOR")) == "" && !strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb")
	return m.app.buildStructuredTUIView(
		m.store,
		m.cfg,
		m.baseDir,
		m.state.snapshot(),
		tuiRenderOptions{
			Width:  m.width,
			Height: m.height,
			ANSI:   ansi,
			Clear:  false,
			Writer: m.app.out,
		},
	)
}

func tuiRefreshCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		return tuiRefreshMsg{}
	})
}

func (m interactiveTUIModel) switchToTab(tabIndex int) {
	summaries, err := m.store.List()
	if err != nil {
		m.state.setActivity("Unable to load PRD tabs.")
		return
	}
	if tabIndex < 1 || tabIndex > len(summaries) {
		m.state.setActivity("Tab " + strconv.Itoa(tabIndex) + " is not available.")
		return
	}
	target := summaries[tabIndex-1].Name
	m.state.setSelectedPRD(target)
	m.state.setActivity("Switched to PRD " + target + ".")
}

func (m interactiveTUIModel) cyclePRD(step int) {
	summaries, err := m.store.List()
	if err != nil {
		m.state.setActivity("Unable to load PRDs.")
		return
	}
	if len(summaries) == 0 {
		m.state.setActivity("No PRDs found.")
		return
	}
	current := strings.TrimSpace(m.state.snapshot().selectedPRD)
	index := 0
	for i := range summaries {
		if summaries[i].Name == current {
			index = i
			break
		}
	}
	next := (index + step + len(summaries)) % len(summaries)
	target := summaries[next].Name
	m.state.setSelectedPRD(target)
	m.state.setActivity("Switched to PRD " + target + ".")
}

func (m interactiveTUIModel) cycleProvider(step int) {
	keys := []string{"codex", "claude", "gemini", "opencode", "copilot", "qwen", "pi"}
	current := strings.ToLower(strings.TrimSpace(m.state.snapshot().provider))
	if current != "" && !slices.Contains(keys, current) {
		keys = append(keys, current)
	}
	if len(keys) == 0 {
		m.state.setActivity("No provider adapters available.")
		return
	}
	index := 0
	for i := range keys {
		if keys[i] == current {
			index = i
			break
		}
	}
	next := (index + step + len(keys)) % len(keys)
	provider := keys[next]
	m.state.setProvider(provider)
	m.state.setActivity("Active provider set to " + provider + ".")
}

func (m interactiveTUIModel) cycleLogFilter() {
	filters := []string{"all", "error", "command_output", "iteration_started", "iteration_completed"}
	current := strings.ToLower(strings.TrimSpace(m.state.snapshot().logFilter))
	index := 0
	for i := range filters {
		if filters[i] == current {
			index = i
			break
		}
	}
	next := (index + 1) % len(filters)
	m.state.setLogFilter(filters[next])
	m.state.setActivity("Log filter set to " + filters[next] + ".")
}
