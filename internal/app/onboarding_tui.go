package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/EstebanForge/daedalus/internal/config"
	"github.com/EstebanForge/daedalus/internal/onboarding"
	"github.com/EstebanForge/daedalus/internal/prd"
	"github.com/EstebanForge/daedalus/internal/project"
	"github.com/EstebanForge/daedalus/internal/providers"
	"github.com/EstebanForge/daedalus/internal/templates"
)

// onboardingScanResultMsg carries the result of the asynchronous repo scan.
type onboardingScanResultMsg struct {
	output string
	err    error
}

// onboardingTickMsg drives the spinner animation while scanning.
type onboardingTickMsg time.Time

// onboardingModel is the BubbleTea model for the 4-step onboarding flow.
type onboardingModel struct {
	ctx     context.Context
	app     App
	store   prd.Store
	cfg     config.Config
	baseDir string
	mgr     *onboarding.Manager
	state   onboarding.State
	step    string // current step name
	mode    onboarding.ProjectMode

	// step-specific input state
	cursor       int    // 0=yes, 1=no for git_ignore
	textInput    string // generic single/multi-line text input
	prdName      string // PRD name for create_prd step
	scanStatus   string // "idle" | "running" | "done" | "error"
	scanOutput   string // raw scan markdown
	scanErr      string
	spinnerFrame int

	width  int
	height int
	done   bool  // set when onboarding complete
	err    error // fatal error that aborts onboarding
}

var onboardingSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// stepNumber returns the 1-based display number for a step name.
func stepNumber(step string) int {
	switch step {
	case "git_ignore":
		return 1
	case "project_discovery":
		return 2
	case "jtbd":
		return 3
	case "create_prd":
		return 4
	}
	return 0
}

// runOnboardingTUI launches the interactive onboarding BubbleTea program.
func (a App) runOnboardingTUI(ctx context.Context, store prd.Store, cfg config.Config, baseDir string, ob *onboarding.Manager) error {
	if !shouldUseInteractiveTUI(a.in, a.out) {
		return fmt.Errorf("onboarding requires an interactive terminal; run 'daedalus' in a terminal to complete setup")
	}

	state, err := ob.LoadState()
	if err != nil {
		return fmt.Errorf("loading onboarding state: %w", err)
	}

	mode, err := ob.DetectProjectMode()
	if err != nil {
		return fmt.Errorf("detecting project mode: %w", err)
	}
	state.ProjectMode = mode

	step := ob.FirstIncompleteStep(state)
	if step == "" {
		return nil
	}

	model := onboardingModel{
		ctx:        ctx,
		app:        a,
		store:      store,
		cfg:        cfg,
		baseDir:    baseDir,
		mgr:        ob,
		state:      state,
		step:       step,
		mode:       mode,
		prdName:    "main",
		scanStatus: "idle",
	}

	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithInput(a.in), tea.WithOutput(a.out))
	finalModel, runErr := program.Run()
	if runErr != nil {
		return runErr
	}
	if m, ok := finalModel.(onboardingModel); ok && m.err != nil {
		return m.err
	}
	return nil
}

// Init satisfies tea.Model.
func (m onboardingModel) Init() tea.Cmd {
	if m.scanStatus == "running" {
		return onboardingSpinnerCmd()
	}
	return nil
}

// Update satisfies tea.Model.
func (m onboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, tea.Quit
	}

	switch current := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = current.Width
		m.height = current.Height
		return m, nil

	case onboardingTickMsg:
		if m.scanStatus == "running" {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(onboardingSpinnerFrames)
			return m, onboardingSpinnerCmd()
		}
		return m, nil

	case onboardingScanResultMsg:
		if current.err != nil {
			m.scanStatus = "error"
			m.scanErr = current.err.Error()
			return m, nil
		}
		m.scanStatus = "done"
		m.scanOutput = current.output
		if err := m.saveScanOutput(); err != nil {
			m.scanStatus = "error"
			m.scanErr = err.Error()
			return m, nil
		}
		return m.completeStep()

	case tea.KeyMsg:
		return m.handleKey(current)
	}

	return m, nil
}

// handleKey dispatches key events for the current step.
func (m onboardingModel) handleKey(msg tea.KeyMsg) (onboardingModel, tea.Cmd) {
	key := msg.String()

	// global quit
	if key == "ctrl+c" || key == "q" {
		return m, tea.Quit
	}

	switch m.step {
	case "git_ignore":
		return m.handleGitIgnoreKey(key)
	case "project_discovery":
		return m.handleProjectDiscoveryKey(msg)
	case "jtbd":
		return m.handleJTBDKey(msg)
	case "create_prd":
		return m.handleCreatePRDKey(msg)
	}
	return m, nil
}

func (m onboardingModel) handleGitIgnoreKey(key string) (onboardingModel, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 1 {
			m.cursor++
		}
	case "enter":
		wantGitIgnore := m.cursor == 0 // 0=Yes, 1=No
		if wantGitIgnore {
			if err := m.writeGitIgnoreEntry(); err != nil {
				m.err = err
				return m, tea.Quit
			}
		}
		return m.completeStep()
	}
	return m, nil
}

func (m onboardingModel) handleProjectDiscoveryKey(msg tea.KeyMsg) (onboardingModel, tea.Cmd) {
	key := msg.String()

	switch m.scanStatus {
	case "running":
		// no keys accepted while scanning (q/ctrl+c handled globally)
		return m, nil

	case "error":
		switch key {
		case "r", "enter":
			// retry scan
			m.scanStatus = "running"
			m.scanErr = ""
			return m, tea.Batch(onboardingSpinnerCmd(), m.buildScanCmd())
		case "s":
			// skip scan with empty summary
			m.scanOutput = ""
			return m.completeStep()
		}
		return m, nil

	default: // "idle"
		switch key {
		case "enter":
			if strings.TrimSpace(m.textInput) == "" {
				return m, nil
			}
			m.scanStatus = "running"
			return m, tea.Batch(onboardingSpinnerCmd(), m.buildScanCmd())
		case "backspace":
			if len(m.textInput) > 0 {
				runes := []rune(m.textInput)
				m.textInput = string(runes[:len(runes)-1])
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.textInput += string(msg.Runes)
			}
		}
	}
	return m, nil
}

func (m onboardingModel) handleJTBDKey(msg tea.KeyMsg) (onboardingModel, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+d":
		return m.completeStep()
	case "enter":
		m.textInput += "\n"
	case "backspace":
		if len(m.textInput) > 0 {
			runes := []rune(m.textInput)
			m.textInput = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.textInput += string(msg.Runes)
		}
	}
	return m, nil
}

func (m onboardingModel) handleCreatePRDKey(msg tea.KeyMsg) (onboardingModel, tea.Cmd) {
	key := msg.String()
	switch key {
	case "enter":
		name := strings.TrimSpace(m.textInput)
		if name == "" {
			name = m.prdName // default "main"
		}
		m.prdName = name
		return m.completeStep()
	case "backspace":
		if len(m.textInput) > 0 {
			runes := []rune(m.textInput)
			m.textInput = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.textInput += string(msg.Runes)
		}
	}
	return m, nil
}

// completeStep marks the current step done, persists state, and advances.
func (m onboardingModel) completeStep() (onboardingModel, tea.Cmd) {
	switch m.step {
	case "git_ignore":
		m.state.Steps.GitIgnore = true

	case "project_discovery":
		m.state.Steps.ProjectDiscovery = true
		// Pre-fill JTBD textarea with the scan output draft
		m.textInput = buildJTBDDraft(m.textInput, m.scanOutput)

	case "jtbd":
		m.state.Steps.JTBD = true
		if err := m.saveJTBD(); err != nil {
			m.err = err
			return m, tea.Quit
		}

	case "create_prd":
		m.state.Steps.CreatePRD = true
		name := strings.TrimSpace(m.prdName)
		if name == "" {
			name = "main"
		}
		if err := m.store.Create(name); err != nil {
			// PRD already exists is OK
			if !strings.Contains(err.Error(), "already exists") {
				m.err = err
				return m, tea.Quit
			}
		}
		// Seed PRD directory with onboarding artifacts.
		if err := m.seedPRDFromOnboarding(name); err != nil {
			m.err = err
			return m, tea.Quit
		}
	}

	if err := m.mgr.SaveState(m.state); err != nil {
		m.err = err
		return m, tea.Quit
	}

	next := m.mgr.FirstIncompleteStep(m.state)
	if next == "" {
		m.state.Completed = true
		if err := m.mgr.SaveState(m.state); err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.done = true
		return m, tea.Quit
	}

	m.step = next
	m.cursor = 0

	// For jtbd step in empty mode: keep textInput empty (no pre-fill)
	// For jtbd step in existing mode: textInput was set in project_discovery branch above
	if m.step == "jtbd" && m.mode == onboarding.ProjectModeEmpty {
		m.textInput = ""
	}

	// For create_prd step: reset text input to show placeholder
	if m.step == "create_prd" {
		m.textInput = ""
	}

	return m, nil
}

// View satisfies tea.Model.
func (m onboardingModel) View() string {
	if m.done {
		return "Onboarding complete. Starting Daedalus...\n"
	}

	var sb strings.Builder

	// Header
	n := stepNumber(m.step)
	fmt.Fprintf(&sb, "Daedalus Setup (step %d/4)\n", n)
	sb.WriteString(strings.Repeat("─", 40) + "\n\n")

	// Body
	switch m.step {
	case "git_ignore":
		sb.WriteString("Add Daedalus generated files to .gitignore?\n\n")
		choices := []string{"Yes", "No"}
		for i, choice := range choices {
			if i == m.cursor {
				fmt.Fprintf(&sb, "  > %s\n", choice)
			} else {
				fmt.Fprintf(&sb, "    %s\n", choice)
			}
		}
		sb.WriteString("\n[↑/↓] select  [Enter] confirm  [q] quit\n")

	case "project_discovery":
		switch m.scanStatus {
		case "idle":
			sb.WriteString("Describe your project (press Enter to scan):\n\n")
			sb.WriteString("> " + m.textInput + "_\n\n")
			sb.WriteString("[Enter] submit  [q] quit\n")
		case "running":
			frame := onboardingSpinnerFrames[m.spinnerFrame%len(onboardingSpinnerFrames)]
			fmt.Fprintf(&sb, "Scanning repository... %s\n\n", frame)
			sb.WriteString("[q] quit\n")
		case "error":
			fmt.Fprintf(&sb, "Scan failed: %s\n\n", m.scanErr)
			sb.WriteString("[Enter/r] retry  [s] skip  [q] quit\n")
		}

	case "jtbd":
		sb.WriteString("Edit your Jobs-to-be-Done (Ctrl+D to confirm):\n\n")
		content := m.textInput
		if content == "" {
			content = "(start typing...)"
		}
		sb.WriteString(content)
		sb.WriteString("_\n\n")
		sb.WriteString("[Enter] new line  [Ctrl+D] confirm  [q] quit\n")

	case "create_prd":
		sb.WriteString("Enter PRD name (default: main):\n\n")
		display := m.textInput
		if display == "" {
			display = m.prdName
		}
		sb.WriteString("> " + display + "_\n\n")
		sb.WriteString("[Enter] confirm  [q] quit\n")
	}

	return sb.String()
}

// buildScanCmd builds a tea.Cmd that runs the repository scan asynchronously.
func (m onboardingModel) buildScanCmd() tea.Cmd {
	ctx := m.ctx
	cfg := m.cfg
	baseDir := m.baseDir
	description := m.textInput

	return func() tea.Msg {
		registry := providers.NewRegistry()
		provider, err := registry.Resolve(cfg.Provider.Default, cfg)
		if err != nil {
			return onboardingScanResultMsg{err: fmt.Errorf("provider unavailable (%s): %w; press s to skip", cfg.Provider.Default, err)}
		}

		prompt := buildOnboardingScanPrompt(description)
		contextFiles := findOnboardingContextFiles(baseDir)

		req := providers.IterationRequest{
			WorkDir:        baseDir,
			Prompt:         prompt,
			ContextFiles:   contextFiles,
			ApprovalPolicy: "never",
		}

		eventCh, _, runErr := provider.RunIteration(ctx, req)
		if runErr != nil {
			return onboardingScanResultMsg{err: runErr}
		}

		var output strings.Builder
		var lastErr error
		for event := range eventCh {
			switch event.Type {
			case providers.EventAssistantText:
				output.WriteString(event.Message)
			case providers.EventError:
				lastErr = fmt.Errorf("%s", event.Message)
			}
		}

		if lastErr != nil {
			return onboardingScanResultMsg{err: lastErr}
		}

		result := strings.TrimSpace(output.String())
		if result == "" {
			result = "*(scan produced no output)*"
		}
		return onboardingScanResultMsg{output: result}
	}
}

func buildOnboardingScanPrompt(description string) string {
	return fmt.Sprintf(`You are performing a read-only repository analysis.

Project description: %s

Produce a filled-in version of the following document template. Rules:
- Replace every [...] placeholder with actual content derived from the repository.
- Keep all section headings exactly as shown (same text, same level).
- Do not add, remove, or rename any section.
- Do not include any commentary outside the document.
- Be concise and factual. Do not make any changes to the repository.

%s`, description, templates.ProjectSummary)
}

func findOnboardingContextFiles(baseDir string) []string {
	candidates := []string{"AGENTS.md", "README.md", "go.mod"}
	var files []string
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(baseDir, candidate)); err == nil {
			files = append(files, candidate)
		}
	}
	return files
}

// saveScanOutput writes project-summary.md to the onboarding directory.
func (m *onboardingModel) saveScanOutput() error {
	dir := project.OnboardingPath(m.baseDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating onboarding directory: %w", err)
	}
	path := filepath.Join(dir, "project-summary.md")
	if err := os.WriteFile(path, []byte(m.scanOutput+"\n"), 0o644); err != nil {
		return fmt.Errorf("saving project summary: %w", err)
	}
	return nil
}

// saveJTBD writes jtbd.md to the onboarding directory.
func (m *onboardingModel) saveJTBD() error {
	dir := project.OnboardingPath(m.baseDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating onboarding directory: %w", err)
	}
	path := filepath.Join(dir, "jtbd.md")
	content := strings.TrimSpace(m.textInput)
	if content == "" {
		content = strings.TrimSpace(templates.JTBD)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		return fmt.Errorf("saving jtbd: %w", err)
	}
	return nil
}

// writeGitIgnoreEntry appends Daedalus entries to .gitignore.
func (m *onboardingModel) writeGitIgnoreEntry() error {
	gitignorePath := filepath.Join(m.baseDir, ".gitignore")

	entry := "\n# Daedalus generated files\n.daedalus/worktrees/\n"

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening .gitignore: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("writing .gitignore entry: %w", err)
	}
	return nil
}

// buildJTBDDraft constructs a JTBD markdown draft from the project description
// and the scan output. The canonical template is used as the base so the
// document structure is deterministic across all provider backends.
func buildJTBDDraft(description, summary string) string {
	draft := strings.TrimRight(templates.JTBD, "\n")

	desc := strings.TrimSpace(description)
	sum := strings.TrimSpace(summary)
	if desc == "" && sum == "" {
		return draft + "\n"
	}

	var sb strings.Builder
	sb.WriteString(draft)
	sb.WriteString("\n\n---\n\n## Source Material\n\n")
	if desc != "" {
		sb.WriteString("**Project description (from onboarding):**\n\n")
		sb.WriteString(desc)
		sb.WriteString("\n\n")
	}
	if sum != "" {
		sb.WriteString("**Repository scan summary:**\n\n")
		sb.WriteString(sum)
		sb.WriteString("\n")
	}
	return sb.String()
}

// seedPRDFromOnboarding copies onboarding artifacts into the newly created PRD
// directory so the loop can use them as context files. Missing source files are
// skipped silently (they are optional).
func (m *onboardingModel) seedPRDFromOnboarding(prdName string) error {
	onboardingDir := project.OnboardingPath(m.baseDir)

	// project-summary.md → .daedalus/prds/<name>/project-summary.md
	summaryDst := project.PRDProjectSummaryPath(m.baseDir, prdName)
	if raw, err := os.ReadFile(filepath.Join(onboardingDir, "project-summary.md")); err == nil {
		if writeErr := os.WriteFile(summaryDst, raw, 0o644); writeErr != nil {
			return fmt.Errorf("seeding project-summary.md: %w", writeErr)
		}
	}

	// jtbd.md → .daedalus/prds/<name>/jtbd.md
	jtbdDst := project.PRDJTBDPath(m.baseDir, prdName)
	if raw, err := os.ReadFile(filepath.Join(onboardingDir, "jtbd.md")); err == nil {
		if writeErr := os.WriteFile(jtbdDst, raw, 0o644); writeErr != nil {
			return fmt.Errorf("seeding jtbd.md: %w", writeErr)
		}
	}

	// architecture-design.md — created from the canonical template, optionally
	// seeded with the project summary when available.
	archDst := project.PRDArchitecturePath(m.baseDir, prdName)
	archContent := buildArchitectureDesignSeed(m.scanOutput)
	if writeErr := os.WriteFile(archDst, []byte(archContent), 0o644); writeErr != nil {
		return fmt.Errorf("seeding architecture-design.md: %w", writeErr)
	}

	return nil
}

// buildArchitectureDesignSeed returns the initial content for architecture-design.md.
// When a scan summary is available it is appended as source material so the
// engineer can use it while authoring the document.
func buildArchitectureDesignSeed(scanSummary string) string {
	base := strings.TrimRight(templates.ArchitectureDesign, "\n")
	sum := strings.TrimSpace(scanSummary)
	if sum == "" {
		return base + "\n"
	}
	return base + "\n\n---\n\n## Source Material (from repository scan)\n\n" + sum + "\n"
}

func onboardingSpinnerCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return onboardingTickMsg(t)
	})
}
