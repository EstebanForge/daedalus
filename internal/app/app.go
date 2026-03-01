package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
	daedalusgit "github.com/EstebanForge/daedalus/internal/git"
	"github.com/EstebanForge/daedalus/internal/loop"
	"github.com/EstebanForge/daedalus/internal/onboarding"
	"github.com/EstebanForge/daedalus/internal/prd"
	"github.com/EstebanForge/daedalus/internal/project"
	"github.com/EstebanForge/daedalus/internal/providers"
	"github.com/EstebanForge/daedalus/internal/quality"
	daedalusworktree "github.com/EstebanForge/daedalus/internal/worktree"
)

type App struct {
	version string
	in      io.Reader
	out     io.Writer
	runEdit func(ctx context.Context, command string, args []string) error
}

type tuiState struct {
	mu             sync.Mutex
	selectedPRD    string
	view           string
	previousView   string
	loopState      string
	themeMode      string
	lastError      string
	lastActivity   string
	iterations     int
	startedAt      time.Time
	lastRunAt      string
	provider       string
	pauseRequested bool
	stopRequested  bool
	storyIndex     int
	pickerIndex    int
	logFilter      string
	logTail        int
	logOffset      int
	diffOffset     int
}

type tuiSnapshot struct {
	selectedPRD    string
	view           string
	previousView   string
	loopState      string
	themeMode      string
	lastError      string
	lastActivity   string
	iterations     int
	startedAt      time.Time
	lastRunAt      string
	provider       string
	pauseRequested bool
	stopRequested  bool
	storyIndex     int
	pickerIndex    int
	logFilter      string
	logTail        int
	logOffset      int
	diffOffset     int
}

func (s *tuiState) snapshot() tuiSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return tuiSnapshot{
		selectedPRD:    s.selectedPRD,
		view:           s.view,
		previousView:   s.previousView,
		loopState:      s.loopState,
		themeMode:      s.themeMode,
		lastError:      s.lastError,
		lastActivity:   s.lastActivity,
		iterations:     s.iterations,
		startedAt:      s.startedAt,
		lastRunAt:      s.lastRunAt,
		provider:       s.provider,
		pauseRequested: s.pauseRequested,
		stopRequested:  s.stopRequested,
		storyIndex:     s.storyIndex,
		pickerIndex:    s.pickerIndex,
		logFilter:      s.logFilter,
		logTail:        s.logTail,
		logOffset:      s.logOffset,
		diffOffset:     s.diffOffset,
	}
}

func (s *tuiState) setView(view string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view = view
}

func (s *tuiState) setViewWithHistory(next string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := strings.TrimSpace(strings.ToLower(s.view))
	target := strings.TrimSpace(strings.ToLower(next))
	if target == "" {
		target = "dashboard"
	}
	if current == "" {
		current = "dashboard"
	}
	if current != target {
		s.previousView = current
	}
	s.view = target
}

func (s *tuiState) closeOverlayView() {
	s.mu.Lock()
	defer s.mu.Unlock()
	restore := strings.TrimSpace(strings.ToLower(s.previousView))
	if restore == "" {
		restore = "dashboard"
	}
	s.view = restore
}

func (s *tuiState) setSelectedPRD(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selectedPRD = name
	s.storyIndex = 0
	s.pickerIndex = 0
	s.logOffset = 0
	s.diffOffset = 0
}

func (s *tuiState) setLoopState(loopState string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loopState = loopState
}

func (s *tuiState) setThemeMode(themeMode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalized := strings.TrimSpace(strings.ToLower(themeMode))
	switch normalized {
	case "dark", "light":
		s.themeMode = normalized
	default:
		s.themeMode = "dark"
	}
}

func (s *tuiState) setActivity(activity string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActivity = strings.TrimSpace(activity)
}

func (s *tuiState) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.lastError = ""
		return
	}
	s.lastError = err.Error()
}

func (s *tuiState) markIterationSuccess(provider string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.iterations++
	s.lastRunAt = time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(provider) != "" {
		s.provider = provider
	}
}

func (s *tuiState) setProvider(provider string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(provider) != "" {
		s.provider = provider
	}
}

func (s *tuiState) setPauseRequested(value bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pauseRequested = value
}

func (s *tuiState) setStopRequested(value bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopRequested = value
}

func (s *tuiState) setLogFilter(filter string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalized := strings.TrimSpace(strings.ToLower(filter))
	if normalized == "" {
		normalized = "all"
	}
	s.logFilter = normalized
	s.logOffset = 0
}

func (s *tuiState) setLogTail(tail int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tail < 1 {
		tail = 1
	}
	s.logTail = tail
}

func (s *tuiState) moveStoryIndex(delta int, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if total < 1 {
		s.storyIndex = 0
		return
	}
	next := s.storyIndex + delta
	if next < 0 {
		next = 0
	}
	if next > total-1 {
		next = total - 1
	}
	s.storyIndex = next
}

func (s *tuiState) movePickerIndex(delta int, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if total < 1 {
		s.pickerIndex = 0
		return
	}
	next := s.pickerIndex + delta
	if next < 0 {
		next = 0
	}
	if next > total-1 {
		next = total - 1
	}
	s.pickerIndex = next
}

func (s *tuiState) moveLogOffset(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.logOffset + delta
	if next < 0 {
		next = 0
	}
	s.logOffset = next
}

func (s *tuiState) moveDiffOffset(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.diffOffset + delta
	if next < 0 {
		next = 0
	}
	s.diffOffset = next
}

type loopController struct {
	mu           sync.Mutex
	running      bool
	pauseRequest bool
	stopRequest  bool
	cancel       context.CancelFunc
}

func (c *loopController) start(cancel context.CancelFunc) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return false
	}
	c.running = true
	c.pauseRequest = false
	c.stopRequest = false
	c.cancel = cancel
	return true
}

func (c *loopController) stopRunning() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	c.cancel = nil
}

func (c *loopController) requestPause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pauseRequest = true
}

func (c *loopController) requestStop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopRequest = true
}

func (c *loopController) requestStopNow() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopRequest = true
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *loopController) checkRequests() (pause bool, stop bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pauseRequest, c.stopRequest
}

type globalOptions struct {
	ConfigPath     string
	Provider       string
	ProviderSet    bool
	Worktree       bool
	WorktreeSet    bool
	MaxRetries     int
	MaxRetriesSet  bool
	RetryDelays    []string
	RetryDelaysSet bool
}

type runOptions struct {
	Name string

	Provider       string
	ProviderSet    bool
	Worktree       bool
	WorktreeSet    bool
	MaxRetries     int
	MaxRetriesSet  bool
	RetryDelays    []string
	RetryDelaysSet bool
}

func New(version string) App {
	return App{
		version: version,
		in:      os.Stdin,
		out:     os.Stdout,
		runEdit: runEditorCommand,
	}
}

func (a App) Run(ctx context.Context, args []string) error {
	defer providers.CloseAllSessions()

	global, remainingArgs, err := parseGlobalOptions(args)
	if err != nil {
		return err
	}

	configPath, err := config.ResolvePath(global.ConfigPath)
	if err != nil {
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	baseDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to read working directory: %w", err)
	}

	store := prd.NewStore(baseDir)
	command := ""
	if len(remainingArgs) > 0 {
		command = remainingArgs[0]
	}

	switch command {
	case "", "tui":
		return a.runTUI(ctx, store, cfg, global, baseDir)
	case "new":
		return a.runNew(store, remainingArgs[1:])
	case "list":
		return a.runList(store)
	case "status":
		return a.runStatus(store, remainingArgs[1:])
	case "validate":
		return a.runValidate(store, remainingArgs[1:])
	case "doctor":
		return a.runDoctor(ctx, cfg, global, remainingArgs[1:])
	case "sessions", "session":
		return a.runSessions(baseDir, remainingArgs[1:])
	case "run":
		return a.runLoop(ctx, store, cfg, global, baseDir, remainingArgs[1:])
	case "plugin":
		return a.runPlugin(ctx, store, cfg, global, baseDir, remainingArgs[1:])
	case "edit":
		return a.runEditCommand(ctx, store, baseDir, remainingArgs[1:])
	case "help", "-h", "--help":
		a.printHelp()
		return nil
	case "version", "-v", "--version":
		a.writef("daedalus version %s\n", a.version)
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (a App) runNew(store prd.Store, args []string) error {
	name := "main"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		name = args[0]
	}

	if err := store.Create(name); err != nil {
		return err
	}
	a.writef("Created PRD %q under .daedalus/prds/%s/\n", name, name)
	return nil
}

func (a App) runList(store prd.Store) error {
	summaries, err := store.List()
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		a.writeLine("No PRDs found.")
		return nil
	}

	for _, summary := range summaries {
		a.writef("%s  %d/%d complete  in-progress:%d\n", summary.Name, summary.Complete, summary.Total, summary.InProgress)
	}
	return nil
}

func (a App) runStatus(store prd.Store, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	name, err := store.ResolveName(name)
	if err != nil {
		return err
	}
	doc, err := store.Load(name)
	if err != nil {
		return err
	}

	a.writef("PRD: %s\n", name)
	a.writef("Project: %s\n", doc.Project)
	a.writef("Stories: %d total\n", len(doc.UserStories))
	a.writef("  complete: %d\n", doc.CountComplete())
	a.writef("  in-progress: %d\n", doc.CountInProgress())
	a.writef("  pending: %d\n", len(doc.UserStories)-doc.CountComplete()-doc.CountInProgress())

	next := doc.NextStory()
	if next == nil {
		a.writeLine("Next: none (all complete)")
		return nil
	}
	a.writef("Next: %s - %s\n", next.ID, next.Title)
	return nil
}

func (a App) runValidate(store prd.Store, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	name, err := store.ResolveName(name)
	if err != nil {
		return err
	}
	doc, err := store.Load(name)
	if err != nil {
		return err
	}

	result := prd.Validate(doc)
	if result.Valid() {
		a.writef("PRD %q is valid.\n", name)
		return nil
	}

	a.writef("PRD %q is invalid:\n", name)
	for _, validationErr := range result.Errors {
		a.writef("- %s\n", validationErr)
	}
	return fmt.Errorf("validation failed")
}

func (a App) runDoctor(ctx context.Context, cfg config.Config, global globalOptions, args []string) error {
	targets := make([]string, 0, len(args)+1)
	for _, arg := range args {
		target := strings.TrimSpace(arg)
		if target == "" {
			continue
		}
		targets = append(targets, target)
	}
	if len(targets) == 0 && global.ProviderSet {
		targets = append(targets, global.Provider)
	}

	report, err := providers.RunACPDoctor(ctx, cfg, targets)
	if err != nil {
		return err
	}

	issues := 0
	for _, check := range report.Checks {
		if !check.Enabled {
			a.writef("- %s: disabled (%s)\n", check.ProviderKey, check.Message)
			continue
		}
		if !check.Healthy {
			issues++
			a.writef("- %s: FAIL\n", check.ProviderKey)
			a.writef("  command: %s\n", check.Command)
			if strings.TrimSpace(check.BinaryPath) != "" {
				a.writef("  binary: %s\n", check.BinaryPath)
			}
			a.writef("  error: %s\n", check.Message)
			continue
		}

		a.writef("- %s: OK\n", check.ProviderKey)
		a.writef("  command: %s\n", check.Command)
		a.writef("  binary: %s\n", check.BinaryPath)
		a.writef("  approval_modes: %s\n", strings.Join(check.Capabilities.ApprovalModes, ", "))
		if len(check.Capabilities.SupportedModels) > 0 {
			a.writef("  models: %s\n", strings.Join(check.Capabilities.SupportedModels, ", "))
		}
	}

	if issues > 0 {
		return fmt.Errorf("doctor found %d unhealthy provider(s)", issues)
	}
	return nil
}

func (a App) runSessions(baseDir string, args []string) error {
	subcommand := "list"
	remaining := args
	if len(remaining) > 0 {
		candidate := strings.ToLower(strings.TrimSpace(remaining[0]))
		if candidate != "" {
			subcommand = candidate
			remaining = remaining[1:]
		}
	}

	filter := ""
	if len(remaining) > 0 {
		filter = strings.ToLower(strings.TrimSpace(remaining[0]))
		if err := validateProviderFilter(filter); err != nil {
			return err
		}
	}

	switch subcommand {
	case "list":
		return a.runSessionsList(baseDir, filter)
	case "status":
		return a.runSessionsStatus(baseDir, filter)
	default:
		return fmt.Errorf("unknown sessions subcommand: %s", subcommand)
	}
}

func (a App) runSessionsList(baseDir, providerFilter string) error {
	persisted, err := providers.ListPersistedACPSessions(baseDir)
	if err != nil {
		return err
	}
	active := providers.ListActiveACPSessions()

	persisted = filterPersistedSessionsByProvider(persisted, providerFilter)
	active = filterActiveSessionsByProvider(active, providerFilter)

	a.writeLine("Persisted ACP sessions:")
	if len(persisted) == 0 {
		a.writeLine("(none)")
	} else {
		for _, record := range persisted {
			staleState := "fresh"
			if record.Stale {
				staleState = "stale"
			}
			a.writef("- provider=%s session=%s updated=%s stale=%s workdir=%s\n", record.ProviderKey, record.SessionID, record.UpdatedAt, staleState, record.WorkDir)
		}
	}

	a.writeLine("Active ACP sessions:")
	if len(active) == 0 {
		a.writeLine("(none)")
		return nil
	}
	for _, record := range active {
		a.writef("- provider=%s session=%s last_used=%s expires=%s workdir=%s\n",
			record.ProviderKey,
			record.SessionID,
			record.LastUsedAt.UTC().Format(time.RFC3339),
			record.ExpiresAt.UTC().Format(time.RFC3339),
			record.WorkDir,
		)
	}
	return nil
}

func (a App) runSessionsStatus(baseDir, providerFilter string) error {
	persisted, err := providers.ListPersistedACPSessions(baseDir)
	if err != nil {
		return err
	}
	active := providers.ListActiveACPSessions()

	persisted = filterPersistedSessionsByProvider(persisted, providerFilter)
	active = filterActiveSessionsByProvider(active, providerFilter)

	staleCount := 0
	for _, record := range persisted {
		if record.Stale {
			staleCount++
		}
	}

	scope := "all providers"
	if providerFilter != "" {
		scope = providerFilter
	}
	a.writef("ACP session status (%s)\n", scope)
	a.writef("- active: %d\n", len(active))
	a.writef("- persisted: %d\n", len(persisted))
	a.writef("- stale persisted: %d\n", staleCount)
	return nil
}

func (a App) runLoop(ctx context.Context, store prd.Store, cfg config.Config, global globalOptions, baseDir string, args []string) error {
	ob := onboarding.NewManager(baseDir)
	required, obErr := ob.IsRequired()
	if obErr != nil {
		return fmt.Errorf("onboarding check failed: %w", obErr)
	}
	if required {
		return fmt.Errorf("run onboarding first: daedalus (without arguments)")
	}

	run, err := parseRunOptions(args)
	if err != nil {
		return err
	}

	providerName, maxRetries, retryDelays, useWorktree, err := resolveRuntimeSettings(cfg, global, run)
	if err != nil {
		return err
	}

	name := run.Name
	name, err = store.ResolveName(name)
	if err != nil {
		return err
	}

	registry := providers.NewRegistry()
	provider, err := registry.Resolve(providerName, cfg)
	if err != nil {
		return err
	}

	execDir := baseDir
	if useWorktree {
		setupResult, setupErr := daedalusworktree.NewManager().Ensure(ctx, baseDir, name)
		if setupErr != nil {
			return setupErr
		}
		execDir = setupResult.Path
		a.writef("Using worktree %q on branch %q.\n", setupResult.Path, setupResult.Branch)
	}

	manager := loop.NewManager(store, provider, loop.RetryPolicy{
		MaxRetries: maxRetries,
		Delays:     retryDelays,
	}, resolveIterationOptions(cfg, provider.Name()), quality.NewRunner(), cfg.Quality.Commands, daedalusgit.NewCommitter())
	if err := manager.RunOnce(ctx, name, baseDir, execDir); err != nil {
		return err
	}

	a.writef("Run completed with provider %q.\n", provider.Name())
	return nil
}

func (a App) printHelp() {
	a.writeLine("Daedalus - Codex-native autonomous delivery loop")
	a.writeLine("")
	a.writeLine("Usage:")
	a.writeLine("  daedalus [--config <path>] [--provider <name>] [--worktree[=<bool>]] [--max-retries <n>] [--retry-delays <csv>] [command]")
	a.writeLine("")
	a.writeLine("Commands:")
	a.writeLine("  new [name]          Create a PRD scaffold")
	a.writeLine("  list                List PRDs")
	a.writeLine("  status [name]       Show PRD status")
	a.writeLine("  validate [name]     Validate PRD JSON")
	a.writeLine("  doctor [provider]   Probe ACP provider health")
	a.writeLine("  sessions [cmd]      ACP session cache observability")
	a.writeLine("  run [name]          Run one iteration (supports --worktree)")
	a.writeLine("  plugin run [name]   Plugin adapter: run one iteration and emit JSON result")
	a.writeLine("  edit [name]         Open prd.md in editor")
	a.writeLine("  help                Show help")
	a.writeLine("  version             Show version")
}

func (a App) runPlugin(ctx context.Context, store prd.Store, cfg config.Config, global globalOptions, baseDir string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("plugin command requires a subcommand")
	}

	switch args[0] {
	case "run":
		originalOut := a.out
		a.out = io.Discard
		runErr := a.runLoop(ctx, store, cfg, global, baseDir, args[1:])
		a.out = originalOut
		if runErr != nil {
			a.writeJSON(map[string]interface{}{
				"ok":    false,
				"error": runErr.Error(),
			})
			return runErr
		}
		a.writeJSON(map[string]interface{}{
			"ok":      true,
			"action":  "run",
			"message": "iteration completed",
		})
		return nil
	default:
		return fmt.Errorf("unknown plugin subcommand: %s", args[0])
	}
}

func (a App) runEditCommand(ctx context.Context, store prd.Store, baseDir string, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	name, err := store.ResolveName(name)
	if err != nil {
		return err
	}

	path := project.PRDMarkdownPath(baseDir, name)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}

	command, editorArgs := resolveEditorCommand()
	editorArgs = append(editorArgs, path)

	runner := a.runEdit
	if runner == nil {
		runner = runEditorCommand
	}
	if err := runner(ctx, command, editorArgs); err != nil {
		return fmt.Errorf("editor command failed: %w", err)
	}
	return nil
}

func resolveEditorCommand() (string, []string) {
	raw := strings.TrimSpace(os.Getenv("DAEDALUS_EDITOR"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if raw == "" {
		return "vi", nil
	}

	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return "vi", nil
	}
	return parts[0], parts[1:]
}

func runEditorCommand(ctx context.Context, command string, args []string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (a App) runTUI(ctx context.Context, store prd.Store, cfg config.Config, global globalOptions, baseDir string) error {
	// Onboarding is only presented when we can show an interactive TUI.
	// Command-mode / piped usage skips the onboarding gate entirely.
	if shouldUseInteractiveTUI(a.in, a.out) {
		ob := onboarding.NewManager(baseDir)
		required, err := ob.IsRequired()
		if err != nil {
			return fmt.Errorf("onboarding check failed: %w", err)
		}
		if required {
			if err := a.runOnboardingTUI(ctx, store, cfg, baseDir, ob); err != nil {
				return fmt.Errorf("onboarding failed: %w", err)
			}
		}
	}

	state := &tuiState{
		selectedPRD:  "",
		view:         "dashboard",
		previousView: "dashboard",
		loopState:    "ready",
		lastActivity: "Ready. Press s to start the loop.",
		startedAt:    time.Now().UTC(),
		logFilter:    "all",
		logTail:      16,
	}
	controller := &loopController{}
	if name, err := store.AutoDetectName(); err == nil {
		state.selectedPRD = name
	} else if summaries, listErr := store.List(); listErr == nil && len(summaries) > 0 {
		state.selectedPRD = summaries[0].Name
	}
	if providerName, _, _, _, err := resolveRuntimeSettings(cfg, global, runOptions{}); err == nil {
		state.provider = providerName
	}
	state.setThemeMode(resolveTUIColorMode(cfg))

	if shouldUseInteractiveTUI(a.in, a.out) {
		if interactiveErr := a.runInteractiveTUI(ctx, store, cfg, global, baseDir, state, controller); interactiveErr == nil {
			return nil
		} else {
			a.writef("Interactive TUI error: %v\n", interactiveErr)
			a.writeLine("Falling back to command mode.")
		}
	}

	a.writeLine("Daedalus TUI")
	a.writeLine("Keys: s(start/run) p(pause) x(stop) t(log) d(diff) n(new) l(PRDs) e(edit) 1-9(switch) j/k(nav) [ ](provider) ,(settings) ?(help) q(quit)")
	return a.runTUICommandLoop(ctx, store, cfg, global, baseDir, state, controller)
}

func (a App) runTUICommandLoop(
	ctx context.Context,
	store prd.Store,
	cfg config.Config,
	global globalOptions,
	baseDir string,
	state *tuiState,
	controller *loopController,
) error {
	scanner := bufio.NewScanner(a.in)
	for {
		snap := state.snapshot()
		a.renderTUIView(store, cfg, baseDir, snap)
		a.writef("daedalus[%s]> ", snap.view)

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			a.writeLine("")
			return nil
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])
		args := parts[1:]

		if tabIndex, ok := parseTUITabShortcut(cmd); ok {
			summaries, err := store.List()
			if err != nil {
				state.setActivity("Unable to load PRD tabs.")
				a.writef("Error: %v\n", err)
				continue
			}
			if tabIndex > len(summaries) {
				state.setActivity(fmt.Sprintf("Tab %d is not available.", tabIndex))
				a.writef("Tab %d is not available.\n", tabIndex)
				continue
			}
			target := summaries[tabIndex-1].Name
			state.setSelectedPRD(target)
			state.setActivity(fmt.Sprintf("Switched to PRD %s.", target))
			a.writef("Selected PRD: %s\n", target)
			continue
		}

		switch cmd {
		case "?", "help":
			a.writeLine("Views: d/dashboard, u/stories, l/logs, diff, picker, h/help, ,/settings")
			a.writeLine("Actions: s/run, p/pause, x/stop, xx/stop-now, v/validate, n/use <name>, 1-9 switch PRD tab, provider <name>, providers, list, status, doctor [provider], sessions [list|status] [provider], f/filter <event|all>, tail <n>, q/quit")
		case "d", "dashboard":
			state.setView("dashboard")
			state.setActivity("Dashboard view.")
		case "u", "stories":
			state.setView("stories")
			state.setActivity("Stories view.")
		case "l", "logs":
			state.setView("logs")
			state.setActivity("Logs view.")
		case "diff":
			state.setView("diff")
			state.setActivity("Diff view.")
		case "picker":
			state.setView("picker")
			state.setActivity("PRD picker view.")
		case "h":
			state.setView("help")
			state.setActivity("Help view.")
		case ",", "settings":
			state.setView("settings")
			state.setActivity("Settings view.")
		case "t", "toggle":
			if snap.view == "dashboard" {
				state.setView("logs")
				state.setActivity("Logs view.")
			} else {
				state.setView("dashboard")
				state.setActivity("Dashboard view.")
			}
		case "n", "use":
			if len(args) != 1 {
				a.writeLine("Usage: n <name> | use <name>")
				continue
			}
			state.setSelectedPRD(args[0])
			state.setActivity(fmt.Sprintf("Selected PRD %s.", args[0]))
			a.writef("Selected PRD: %s\n", args[0])
		case "provider", "agent":
			if len(args) != 1 {
				a.writeLine("Usage: provider <name>")
				continue
			}
			nextProvider := strings.ToLower(strings.TrimSpace(args[0]))
			if nextProvider == "" {
				a.writeLine("provider expects a non-empty name")
				continue
			}
			state.setProvider(nextProvider)
			state.setActivity(fmt.Sprintf("Active provider set to %s.", nextProvider))
			a.writef("Active provider: %s\n", nextProvider)
		case "providers", "agents":
			state.setActivity("Provider adapters list.")
			for _, line := range tuiProviderStatusLines(cfg, snap.provider) {
				a.writeLine(line)
			}
		case "list":
			state.setActivity("Listing PRDs.")
			if err := a.runList(store); err != nil {
				state.setActivity("Failed to list PRDs.")
				a.writef("Error: %v\n", err)
			}
		case "status":
			target := snap.selectedPRD
			if len(args) == 1 {
				target = args[0]
			}
			state.setActivity("Showing PRD status.")
			a.writef("Runtime: state=%s iterations=%d provider=%s last_run_at=%s\n", snap.loopState, snap.iterations, snap.provider, snap.lastRunAt)
			if err := a.runStatus(store, []string{target}); err != nil {
				state.setActivity("Failed to read PRD status.")
				a.writef("Error: %v\n", err)
			}
		case "doctor":
			targets := []string{}
			if len(args) > 0 {
				targets = append(targets, args...)
			}
			state.setActivity("Running ACP doctor checks.")
			if err := a.runDoctor(ctx, cfg, global, targets); err != nil {
				state.setActivity("Doctor checks found issues.")
				a.writef("Error: %v\n", err)
			}
		case "sessions", "session":
			state.setActivity("Showing ACP session observability.")
			if err := a.runSessions(baseDir, args); err != nil {
				state.setActivity("ACP session observability failed.")
				a.writef("Error: %v\n", err)
			}
		case "v", "validate":
			target := snap.selectedPRD
			if len(args) == 1 {
				target = args[0]
			}
			if err := a.runValidate(store, []string{target}); err != nil {
				state.setActivity("Validation failed.")
				a.writef("Error: %v\n", err)
				continue
			}
			state.setActivity("Validation passed.")
		case "s", "run":
			runCtx, cancelRun := context.WithCancel(ctx)
			if !controller.start(cancelRun) {
				cancelRun()
				state.setActivity("Loop is already running.")
				a.writeLine("Loop is already running.")
				continue
			}
			state.setError(nil)
			state.setPauseRequested(false)
			state.setStopRequested(false)
			state.setLoopState("running")
			state.setActivity("Loop started.")
			a.logTUIRuntimeAction(store, baseDir, state.snapshot(), "tui start requested")
			go a.runTUILoopWorker(runCtx, store, cfg, global, baseDir, state, controller)
		case "p", "pause":
			state.setPauseRequested(true)
			controller.requestPause()
			state.setActivity("Pause requested; waiting for current iteration.")
			a.logTUIRuntimeAction(store, baseDir, state.snapshot(), "tui pause requested")
			a.writeLine("Pause requested; current iteration will complete first.")
		case "x", "stop":
			state.setStopRequested(true)
			controller.requestStop()
			state.setActivity("Stop requested; waiting for current iteration.")
			a.logTUIRuntimeAction(store, baseDir, state.snapshot(), "tui stop requested")
			a.writeLine("Stop requested; current iteration will complete first.")
		case "xx", "stop-now":
			state.setStopRequested(true)
			controller.requestStopNow()
			state.setActivity("Immediate stop requested.")
			a.logTUIRuntimeAction(store, baseDir, state.snapshot(), "tui stop-now requested")
			a.writeLine("Immediate stop requested; active iteration was cancelled.")
		case "f", "filter":
			if len(args) != 1 {
				a.writeLine("Usage: f <event-type|all> | filter <event-type|all>")
				continue
			}
			state.setLogFilter(args[0])
			state.setActivity(fmt.Sprintf("Log filter set to %s.", strings.ToLower(strings.TrimSpace(args[0]))))
			a.writef("Log filter set to: %s\n", strings.ToLower(strings.TrimSpace(args[0])))
		case "tail":
			if len(args) != 1 {
				a.writeLine("Usage: tail <n>")
				continue
			}
			n, parseErr := strconv.Atoi(args[0])
			if parseErr != nil || n < 1 {
				a.writeLine("tail expects a positive integer")
				continue
			}
			state.setLogTail(n)
			state.setActivity(fmt.Sprintf("Log tail set to %d.", n))
			a.writef("Log tail set to: %d\n", n)
		case "q", "quit", "exit":
			state.setStopRequested(true)
			controller.requestStopNow()
			state.setActivity("Exiting TUI.")
			return nil
		default:
			state.setActivity(fmt.Sprintf("Unknown command: %s.", cmd))
			a.writef("Unknown command: %s (type ? for help)\n", cmd)
		}
	}
}

func (a App) runTUILoopWorker(
	ctx context.Context,
	store prd.Store,
	cfg config.Config,
	global globalOptions,
	baseDir string,
	state *tuiState,
	controller *loopController,
) {
	defer controller.stopRunning()

	for {
		pause, stop := controller.checkRequests()
		if stop {
			state.setStopRequested(false)
			state.setPauseRequested(false)
			state.setLoopState("stopped")
			state.setActivity("Loop stopped.")
			a.logTUIRuntimeAction(store, baseDir, state.snapshot(), "loop stopped")
			return
		}
		if pause {
			state.setPauseRequested(false)
			state.setLoopState("paused")
			state.setActivity("Loop paused.")
			a.logTUIRuntimeAction(store, baseDir, state.snapshot(), "loop paused")
			return
		}

		snap := state.snapshot()
		runArgs := []string{}
		if strings.TrimSpace(snap.selectedPRD) != "" {
			runArgs = append(runArgs, snap.selectedPRD)
		}
		if strings.TrimSpace(snap.provider) != "" {
			runArgs = append(runArgs, "--provider", strings.TrimSpace(snap.provider))
		}
		state.setActivity(fmt.Sprintf("Running iteration %d with provider %s.", snap.iterations+1, snap.provider))

		if err := a.runLoop(ctx, store, cfg, global, baseDir, runArgs); err != nil {
			if _, stopNow := controller.checkRequests(); stopNow && errors.Is(err, context.Canceled) {
				state.setStopRequested(false)
				state.setLoopState("stopped")
				state.setError(nil)
				state.setActivity("Loop stopped immediately.")
				a.logTUIRuntimeAction(store, baseDir, state.snapshot(), "loop stopped immediately")
				return
			}
			state.setError(err)
			state.setLoopState("error")
			state.setActivity("Loop error: " + err.Error())
			a.logTUIRuntimeAction(store, baseDir, state.snapshot(), "loop error: "+err.Error())
			return
		}
		state.setError(nil)
		state.setStopRequested(false)
		state.setPauseRequested(false)
		state.markIterationSuccess(snap.provider)
		state.setActivity(fmt.Sprintf("Iteration %d completed.", snap.iterations+1))

		name := strings.TrimSpace(snap.selectedPRD)
		resolvedName, err := store.ResolveName(name)
		if err != nil {
			state.setError(err)
			state.setLoopState("error")
			state.setActivity("Failed to resolve PRD: " + err.Error())
			return
		}
		doc, err := store.Load(resolvedName)
		if err != nil {
			state.setError(err)
			state.setLoopState("error")
			state.setActivity("Failed to load PRD: " + err.Error())
			return
		}
		if doc.NextStory() == nil {
			state.setLoopState("completed")
			state.setActivity("All stories completed.")
			a.logTUIRuntimeAction(store, baseDir, state.snapshot(), "loop completed")
			return
		}
	}
}

func (a App) renderTUIView(store prd.Store, cfg config.Config, baseDir string, state tuiSnapshot) {
	a.renderStructuredTUIView(store, cfg, baseDir, state)
}

func (a App) writeLine(text string) {
	_, _ = fmt.Fprintln(a.out, text)
}

func (a App) writef(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(a.out, format, args...)
}

func (a App) writeJSON(payload map[string]interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		a.writef("{\"ok\":false,\"error\":\"failed to encode plugin response: %s\"}\n", err.Error())
		return
	}
	a.writef("%s\n", string(data))
}

func (a App) logTUIRuntimeAction(store prd.Store, baseDir string, snap tuiSnapshot, message string) {
	name, ok := resolveTUIArtifactPRD(store, strings.TrimSpace(snap.selectedPRD))
	if !ok {
		return
	}

	iteration := snap.iterations
	if iteration < 1 {
		iteration = 1
	}
	_ = appendTUIEvent(baseDir, name, iteration, message)
	_ = appendTUILog(baseDir, name, message)
}

func resolveTUIArtifactPRD(store prd.Store, selected string) (string, bool) {
	if selected != "" {
		name, err := store.ResolveName(selected)
		return name, err == nil
	}
	name, err := store.AutoDetectName()
	return name, err == nil
}

func appendTUIEvent(baseDir, name string, iteration int, message string) error {
	path := project.PRDEventsPath(baseDir, name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	payload := map[string]interface{}{
		"type":      "command_output",
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"iteration": iteration,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n'))
	return err
}

func appendTUILog(baseDir, name, message string) error {
	path := project.PRDAgentLogPath(baseDir, name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	_, err = file.WriteString("[tui] " + message + "\n")
	return err
}

type eventLogEntry struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Iteration int    `json:"iteration"`
}

func readEventEntries(path string) ([]eventLogEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return []eventLogEntry{}, nil
	}
	lines := strings.Split(text, "\n")
	entries := make([]eventLogEntry, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var entry eventLogEntry
		if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func filterEventEntries(entries []eventLogEntry, filter string) []eventLogEntry {
	normalized := strings.TrimSpace(strings.ToLower(filter))
	if normalized == "" || normalized == "all" {
		return entries
	}
	filtered := make([]eventLogEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.ToLower(strings.TrimSpace(entry.Type)) == normalized {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func parseTUITabShortcut(command string) (int, bool) {
	if len(command) != 1 {
		return 0, false
	}
	if command[0] < '1' || command[0] > '9' {
		return 0, false
	}
	return int(command[0] - '0'), true
}

func parseGlobalOptions(args []string) (globalOptions, []string, error) {
	options := globalOptions{}
	index := 0

	for index < len(args) {
		token := args[index]
		if !strings.HasPrefix(token, "--") {
			break
		}
		if token == "--help" || token == "--version" {
			break
		}
		if token == "--" {
			index++
			break
		}

		key, value, hasValue, err := splitFlag(token)
		if err != nil {
			return globalOptions{}, nil, err
		}

		switch key {
		case "config":
			if !hasValue {
				index++
				if index >= len(args) {
					return globalOptions{}, nil, fmt.Errorf("--config requires a value")
				}
				value = args[index]
			}
			options.ConfigPath = strings.TrimSpace(value)
		case "provider":
			if !hasValue {
				index++
				if index >= len(args) {
					return globalOptions{}, nil, fmt.Errorf("--provider requires a value")
				}
				value = args[index]
			}
			options.Provider = strings.TrimSpace(value)
			options.ProviderSet = true
		case "worktree":
			flagValue, parseErr := parseOptionalBoolFlag("worktree", value, hasValue)
			if parseErr != nil {
				return globalOptions{}, nil, parseErr
			}
			options.Worktree = flagValue
			options.WorktreeSet = true
		case "max-retries":
			if !hasValue {
				index++
				if index >= len(args) {
					return globalOptions{}, nil, fmt.Errorf("--max-retries requires a value")
				}
				value = args[index]
			}
			retries, parseErr := strconv.Atoi(value)
			if parseErr != nil || retries < 0 {
				return globalOptions{}, nil, fmt.Errorf("--max-retries must be a non-negative integer")
			}
			options.MaxRetries = retries
			options.MaxRetriesSet = true
		case "retry-delays":
			if !hasValue {
				index++
				if index >= len(args) {
					return globalOptions{}, nil, fmt.Errorf("--retry-delays requires a value")
				}
				value = args[index]
			}
			options.RetryDelays = parseCSV(value)
			options.RetryDelaysSet = true
		default:
			return globalOptions{}, nil, fmt.Errorf("unknown global flag: --%s", key)
		}
		index++
	}

	return options, args[index:], nil
}

func parseRunOptions(args []string) (runOptions, error) {
	options := runOptions{}

	for i := 0; i < len(args); i++ {
		token := args[i]
		if !strings.HasPrefix(token, "--") {
			if options.Name == "" {
				options.Name = token
				continue
			}
			return runOptions{}, fmt.Errorf("unexpected argument: %s", token)
		}

		key, value, hasValue, err := splitFlag(token)
		if err != nil {
			return runOptions{}, err
		}

		switch key {
		case "provider":
			if !hasValue {
				i++
				if i >= len(args) {
					return runOptions{}, fmt.Errorf("--provider requires a value")
				}
				value = args[i]
			}
			options.Provider = strings.TrimSpace(value)
			options.ProviderSet = true
		case "worktree":
			flagValue, parseErr := parseOptionalBoolFlag("worktree", value, hasValue)
			if parseErr != nil {
				return runOptions{}, parseErr
			}
			options.Worktree = flagValue
			options.WorktreeSet = true
		case "max-retries":
			if !hasValue {
				i++
				if i >= len(args) {
					return runOptions{}, fmt.Errorf("--max-retries requires a value")
				}
				value = args[i]
			}
			retries, parseErr := strconv.Atoi(value)
			if parseErr != nil || retries < 0 {
				return runOptions{}, fmt.Errorf("--max-retries must be a non-negative integer")
			}
			options.MaxRetries = retries
			options.MaxRetriesSet = true
		case "retry-delays":
			if !hasValue {
				i++
				if i >= len(args) {
					return runOptions{}, fmt.Errorf("--retry-delays requires a value")
				}
				value = args[i]
			}
			options.RetryDelays = parseCSV(value)
			options.RetryDelaysSet = true
		default:
			return runOptions{}, fmt.Errorf("unknown run flag: --%s", key)
		}
	}

	return options, nil
}

func splitFlag(token string) (key, value string, hasValue bool, err error) {
	if !strings.HasPrefix(token, "--") {
		return "", "", false, fmt.Errorf("invalid flag token: %s", token)
	}
	body := strings.TrimPrefix(token, "--")
	if body == "" {
		return "", "", false, fmt.Errorf("invalid flag token: %s", token)
	}

	parts := strings.SplitN(body, "=", 2)
	key = strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", false, fmt.Errorf("invalid flag token: %s", token)
	}
	if len(parts) == 2 {
		return key, parts[1], true, nil
	}
	return key, "", false, nil
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func parseOptionalBoolFlag(name, value string, hasValue bool) (bool, error) {
	if !hasValue {
		return true, nil
	}
	parsed, err := parseBool(value)
	if err != nil {
		return false, fmt.Errorf("--%s must be a boolean value", name)
	}
	return parsed, nil
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}

func resolveRuntimeSettings(cfg config.Config, global globalOptions, run runOptions) (string, int, []time.Duration, bool, error) {
	providerName := cfg.Provider.Default
	if envProvider := strings.TrimSpace(os.Getenv("DAEDALUS_PROVIDER")); envProvider != "" {
		providerName = envProvider
	}
	if global.ProviderSet {
		providerName = global.Provider
	}
	if run.ProviderSet {
		providerName = run.Provider
	}
	if strings.TrimSpace(providerName) == "" {
		return "", 0, nil, false, fmt.Errorf("provider is required")
	}

	maxRetries := cfg.Retry.MaxRetries
	if envRetries := strings.TrimSpace(os.Getenv("DAEDALUS_MAX_RETRIES")); envRetries != "" {
		value, err := strconv.Atoi(envRetries)
		if err != nil || value < 0 {
			return "", 0, nil, false, fmt.Errorf("DAEDALUS_MAX_RETRIES must be a non-negative integer")
		}
		maxRetries = value
	}
	if global.MaxRetriesSet {
		maxRetries = global.MaxRetries
	}
	if run.MaxRetriesSet {
		maxRetries = run.MaxRetries
	}

	retryDelayStrings := cfg.Retry.Delays
	if envDelayCSV := strings.TrimSpace(os.Getenv("DAEDALUS_RETRY_DELAYS")); envDelayCSV != "" {
		retryDelayStrings = parseCSV(envDelayCSV)
	}
	if global.RetryDelaysSet {
		retryDelayStrings = global.RetryDelays
	}
	if run.RetryDelaysSet {
		retryDelayStrings = run.RetryDelays
	}

	if maxRetries > 0 && len(retryDelayStrings) == 0 {
		return "", 0, nil, false, fmt.Errorf("retry delays must not be empty when max retries is greater than zero")
	}

	retryDelays, err := config.ParseRetryDelays(retryDelayStrings)
	if err != nil {
		return "", 0, nil, false, err
	}

	useWorktree := cfg.Worktree.Enabled
	if envWorktree := strings.TrimSpace(os.Getenv("DAEDALUS_WORKTREE")); envWorktree != "" {
		parsed, parseErr := parseBool(envWorktree)
		if parseErr != nil {
			return "", 0, nil, false, fmt.Errorf("DAEDALUS_WORKTREE must be a boolean value")
		}
		useWorktree = parsed
	}
	if global.WorktreeSet {
		useWorktree = global.Worktree
	}
	if run.WorktreeSet {
		useWorktree = run.Worktree
	}

	return providerName, maxRetries, retryDelays, useWorktree, nil
}

func resolveIterationOptions(cfg config.Config, providerName string) loop.IterationOptions {
	providerCfg := providerConfigForKey(cfg, providerName)
	approvalPolicy := strings.TrimSpace(providerCfg.ApprovalPolicy)
	if approvalPolicy == "" {
		approvalPolicy = "on-failure"
	}
	sandboxPolicy := strings.TrimSpace(providerCfg.SandboxPolicy)
	if sandboxPolicy == "" {
		sandboxPolicy = "workspace-write"
	}
	model := strings.TrimSpace(providerCfg.Model)
	if model == "" {
		model = "default"
	}

	return loop.IterationOptions{
		ApprovalPolicy: approvalPolicy,
		SandboxPolicy:  sandboxPolicy,
		Model:          model,
	}
}

func providerConfigForKey(cfg config.Config, providerName string) config.GenericProviderConfig {
	switch strings.ToLower(strings.TrimSpace(providerName)) {
	case "codex":
		return cfg.Providers.Codex
	case "claude":
		return cfg.Providers.Claude
	case "gemini":
		return cfg.Providers.Gemini
	case "opencode":
		return cfg.Providers.OpenCode
	case "copilot":
		return cfg.Providers.Copilot
	case "qwen":
		return cfg.Providers.Qwen
	case "pi":
		return cfg.Providers.Pi
	default:
		return cfg.Providers.Codex
	}
}

func validateProviderFilter(provider string) error {
	trimmed := strings.TrimSpace(strings.ToLower(provider))
	if trimmed == "" {
		return nil
	}
	for _, known := range providers.KnownProviderKeys() {
		if known == trimmed {
			return nil
		}
	}
	return providers.NewUnknownProviderError(trimmed)
}

func filterPersistedSessionsByProvider(records []providers.ACPPersistedSessionInfo, provider string) []providers.ACPPersistedSessionInfo {
	trimmed := strings.TrimSpace(strings.ToLower(provider))
	if trimmed == "" {
		return records
	}
	filtered := make([]providers.ACPPersistedSessionInfo, 0, len(records))
	for _, record := range records {
		if strings.EqualFold(record.ProviderKey, trimmed) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterActiveSessionsByProvider(records []providers.ACPActiveSessionInfo, provider string) []providers.ACPActiveSessionInfo {
	trimmed := strings.TrimSpace(strings.ToLower(provider))
	if trimmed == "" {
		return records
	}
	filtered := make([]providers.ACPActiveSessionInfo, 0, len(records))
	for _, record := range records {
		if strings.EqualFold(record.ProviderKey, trimmed) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}
