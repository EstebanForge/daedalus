package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
	"github.com/EstebanForge/daedalus/internal/prd"
	"github.com/EstebanForge/daedalus/internal/project"
)

func TestRunNoCommandStartsTUIAndQuits(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DAEDALUS_CONFIG", filepath.Join(tmp, "missing-config.toml"))
	t.Chdir(tmp)

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader("quit\n"),
		out:     &out,
	}

	if err := application.Run(context.Background(), nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(out.String(), "Daedalus TUI") {
		t.Fatalf("expected TUI output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Loop: ready") {
		t.Fatalf("expected loop state in dashboard, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Iterations: 0") {
		t.Fatalf("expected initial iteration count, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Provider: codex") {
		t.Fatalf("expected provider in dashboard, got: %s", out.String())
	}
}

func TestRunNoCommandSwitchesToSettingsView(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DAEDALUS_CONFIG", filepath.Join(tmp, "missing-config.toml"))
	t.Chdir(tmp)

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader(",\nq\n"),
		out:     &out,
	}

	if err := application.Run(context.Background(), nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(out.String(), "[Settings]") {
		t.Fatalf("expected settings view output, got: %s", out.String())
	}
}

func TestRunNoCommandAcceptsPauseAndStopCommands(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DAEDALUS_CONFIG", filepath.Join(tmp, "missing-config.toml"))
	t.Chdir(tmp)

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader("p\nx\nq\n"),
		out:     &out,
	}

	if err := application.Run(context.Background(), nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(out.String(), "Pause requested") {
		t.Fatalf("expected pause output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Stop requested") {
		t.Fatalf("expected stop output, got: %s", out.String())
	}
}

func TestRunNoCommandSupportsProviderSelection(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DAEDALUS_CONFIG", filepath.Join(tmp, "missing-config.toml"))
	t.Chdir(tmp)

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader("provider claude\nq\n"),
		out:     &out,
	}

	if err := application.Run(context.Background(), nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "Active provider: claude") {
		t.Fatalf("expected provider command output, got: %s", output)
	}
	if !strings.Contains(output, "Agent: claude") {
		t.Fatalf("expected header to show selected provider, got: %s", output)
	}
}

func TestTUIWritesPauseStopActionsToArtifacts(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DAEDALUS_CONFIG", filepath.Join(tmp, "missing-config.toml"))
	t.Chdir(tmp)

	store := prd.NewStore(tmp)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader("n main\np\nx\nq\n"),
		out:     &out,
	}

	if err := application.Run(context.Background(), nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	events, err := os.ReadFile(project.PRDEventsPath(tmp, "main"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !strings.Contains(string(events), "tui pause requested") {
		t.Fatalf("expected pause event in events.jsonl, got: %s", string(events))
	}
	if !strings.Contains(string(events), "tui stop requested") {
		t.Fatalf("expected stop event in events.jsonl, got: %s", string(events))
	}

	agentLog, err := os.ReadFile(project.PRDAgentLogPath(tmp, "main"))
	if err != nil {
		t.Fatalf("read agent log: %v", err)
	}
	if !strings.Contains(string(agentLog), "[tui] tui pause requested") {
		t.Fatalf("expected pause entry in agent.log, got: %s", string(agentLog))
	}
	if !strings.Contains(string(agentLog), "[tui] tui stop requested") {
		t.Fatalf("expected stop entry in agent.log, got: %s", string(agentLog))
	}
}

func TestParseRunOptionsSupportsWorktreeFlag(t *testing.T) {
	t.Parallel()

	options, err := parseRunOptions([]string{"main", "--worktree"})
	if err != nil {
		t.Fatalf("parse run options: %v", err)
	}
	if options.Name != "main" {
		t.Fatalf("expected name main, got %q", options.Name)
	}
	if !options.WorktreeSet || !options.Worktree {
		t.Fatalf("expected worktree flag true, got set=%v value=%v", options.WorktreeSet, options.Worktree)
	}
}

func TestResolveRuntimeSettingsWorktreePrecedence(t *testing.T) {
	cfg := config.Defaults()
	cfg.Worktree.Enabled = false
	t.Setenv("DAEDALUS_WORKTREE", "true")

	providerName, maxRetries, delays, useWorktree, err := resolveRuntimeSettings(
		cfg,
		globalOptions{WorktreeSet: true, Worktree: false},
		runOptions{WorktreeSet: true, Worktree: true},
	)
	if err != nil {
		t.Fatalf("resolve runtime settings: %v", err)
	}
	if providerName != "codex" {
		t.Fatalf("unexpected provider: %s", providerName)
	}
	if maxRetries != 3 {
		t.Fatalf("unexpected max retries: %d", maxRetries)
	}
	wantDelays := []time.Duration{0, 5 * time.Second, 15 * time.Second}
	if !reflect.DeepEqual(wantDelays, delays) {
		t.Fatalf("unexpected delays: %v", delays)
	}
	if !useWorktree {
		t.Fatal("expected run-level worktree override to be true")
	}
}

func TestResolveIterationOptionsUsesProviderConfig(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Providers.Gemini.Model = "gemini-2.5-pro"
	cfg.Providers.Gemini.ApprovalPolicy = "never"
	cfg.Providers.Gemini.SandboxPolicy = "workspace-write"

	options := resolveIterationOptions(cfg, "gemini")
	if options.Model != "gemini-2.5-pro" {
		t.Fatalf("unexpected model: %q", options.Model)
	}
	if options.ApprovalPolicy != "never" {
		t.Fatalf("unexpected approval policy: %q", options.ApprovalPolicy)
	}
	if options.SandboxPolicy != "workspace-write" {
		t.Fatalf("unexpected sandbox policy: %q", options.SandboxPolicy)
	}
}

func TestRunDoctorReturnsErrorForUnhealthyProvider(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Codex.ACPCommand = "definitely-missing-acp-binary"

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader(""),
		out:     &out,
	}

	err := application.runDoctor(context.Background(), cfg, globalOptions{}, []string{"codex"})
	if err == nil {
		t.Fatal("expected doctor error")
	}
	if !strings.Contains(err.Error(), "unhealthy provider") {
		t.Fatalf("unexpected error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "codex: FAIL") {
		t.Fatalf("expected failure output, got: %s", text)
	}
}

func TestRunSessionsListNoData(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader(""),
		out:     &out,
	}

	if err := application.runSessions(baseDir, nil); err != nil {
		t.Fatalf("run sessions: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Persisted ACP sessions:") {
		t.Fatalf("expected persisted section, got: %s", text)
	}
	if !strings.Contains(text, "Active ACP sessions:") {
		t.Fatalf("expected active section, got: %s", text)
	}
}

func TestRunSessionsStatusReadsPersistedCache(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	cachePath := project.ACPSessionsPath(baseDir)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	cacheJSON := `{"version":1,"sessions":{"codex:/tmp/repo":{"providerKey":"codex","workDir":"/tmp/repo","command":"codex-acp","sessionId":"sess-1","createdAt":"2026-02-28T10:00:00Z","updatedAt":"2026-02-28T10:05:00Z"}}}`
	if err := os.WriteFile(cachePath, []byte(cacheJSON), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader(""),
		out:     &out,
	}

	if err := application.runSessions(baseDir, []string{"status"}); err != nil {
		t.Fatalf("run sessions status: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "persisted: 1") {
		t.Fatalf("expected persisted count in status, got: %s", text)
	}
}

func TestRunSessionsUnknownSubcommand(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader(""),
		out:     &out,
	}

	err := application.runSessions(t.TempDir(), []string{"unknown"})
	if err == nil {
		t.Fatal("expected sessions subcommand error")
	}
	if !strings.Contains(err.Error(), "unknown sessions subcommand") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunEditUsesConfiguredEditor(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DAEDALUS_CONFIG", filepath.Join(tmp, "missing-config.toml"))
	t.Setenv("DAEDALUS_EDITOR", "code -w")
	t.Chdir(tmp)

	store := prd.NewStore(tmp)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	var gotCommand string
	var gotArgs []string
	application := App{
		version: "test",
		in:      strings.NewReader(""),
		out:     &bytes.Buffer{},
		runEdit: func(_ context.Context, command string, args []string) error {
			gotCommand = command
			gotArgs = append([]string(nil), args...)
			return nil
		},
	}

	if err := application.Run(context.Background(), []string{"edit", "main"}); err != nil {
		t.Fatalf("run edit: %v", err)
	}
	if gotCommand != "code" {
		t.Fatalf("expected editor command code, got %q", gotCommand)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "-w" {
		t.Fatalf("unexpected editor args: %v", gotArgs)
	}
	if gotArgs[1] != project.PRDMarkdownPath(tmp, "main") {
		t.Fatalf("unexpected edit target path: %s", gotArgs[1])
	}
}

func TestRunPluginRequiresSubcommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DAEDALUS_CONFIG", filepath.Join(tmp, "missing-config.toml"))
	t.Chdir(tmp)

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader(""),
		out:     &out,
	}

	err := application.Run(context.Background(), []string{"plugin"})
	if err == nil {
		t.Fatal("expected plugin error without subcommand")
	}
	if !strings.Contains(err.Error(), "requires a subcommand") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPluginUnknownSubcommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DAEDALUS_CONFIG", filepath.Join(tmp, "missing-config.toml"))
	t.Chdir(tmp)

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader(""),
		out:     &out,
	}

	err := application.Run(context.Background(), []string{"plugin", "unknown"})
	if err == nil {
		t.Fatal("expected plugin error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown plugin subcommand") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTUILogsViewReadsEventStreamWithFilterAndTail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DAEDALUS_CONFIG", filepath.Join(tmp, "missing-config.toml"))
	t.Chdir(tmp)

	store := prd.NewStore(tmp)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	eventsPath := project.PRDEventsPath(tmp, "main")
	eventsData := strings.Join([]string{
		`{"type":"iteration_started","message":"started","timestamp":"2026-02-22T20:00:00Z","iteration":1}`,
		`{"type":"command_output","message":"running tests","timestamp":"2026-02-22T20:00:01Z","iteration":1}`,
		`{"type":"error","message":"timeout","timestamp":"2026-02-22T20:00:02Z","iteration":1}`,
	}, "\n") + "\n"
	if err := os.WriteFile(eventsPath, []byte(eventsData), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}

	var out bytes.Buffer
	application := App{
		version: "test",
		in:      strings.NewReader("n main\nl\nf error\ntail 1\nl\nq\n"),
		out:     &out,
	}
	if err := application.Run(context.Background(), nil); err != nil {
		t.Fatalf("run tui: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Source:") {
		t.Fatalf("expected events source in logs view, got: %s", output)
	}
	if !strings.Contains(output, "events.jsonl") {
		t.Fatalf("expected events file path in logs view, got: %s", output)
	}
	if !strings.Contains(output, "filter=error tail=1") {
		t.Fatalf("expected filtered events source details in logs view, got: %s", output)
	}
	if !strings.Contains(output, "Log filter set to: error") {
		t.Fatalf("expected filter update output, got: %s", output)
	}
	if !strings.Contains(output, "Log tail set to: 1") {
		t.Fatalf("expected tail update output, got: %s", output)
	}
	if !strings.Contains(output, "[error] timeout") {
		t.Fatalf("expected filtered error event in logs, got: %s", output)
	}
}
