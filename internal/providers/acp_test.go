package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
)

func TestACPProviderRunIterationSuccess(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Cleanup(CloseAllSessions)

	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Codex.ACPCommand = helperACPCommand("acp-helper")

	provider := newACPProvider(cfg, "codex")
	events, result, err := provider.RunIteration(context.Background(), IterationRequest{
		WorkDir: t.TempDir(),
		Prompt:  "Implement this story",
	})
	if err != nil {
		t.Fatalf("run iteration: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}

	gotEvents := collectEvents(events)
	if len(gotEvents) < 3 {
		t.Fatalf("expected events, got %v", gotEvents)
	}
	types := eventTypes(gotEvents)
	if types[0] != EventIterationStarted {
		t.Fatalf("expected first event iteration_started, got %v", types[0])
	}
	if types[len(types)-1] != EventIterationDone {
		t.Fatalf("expected last event iteration_finished, got %v", types[len(types)-1])
	}
	if !containsEventType(gotEvents, EventToolStarted) {
		t.Fatalf("expected tool_started event, got %v", types)
	}
	if !containsEventType(gotEvents, EventToolFinished) {
		t.Fatalf("expected tool_finished event, got %v", types)
	}
	if !containsAssistantText(gotEvents, "hello world") {
		t.Fatalf("expected aggregated assistant summary, got events: %+v", gotEvents)
	}
}

func TestACPProviderRunIterationMissingBinary(t *testing.T) {
	t.Parallel()
	t.Cleanup(CloseAllSessions)

	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Codex.ACPCommand = "definitely-missing-acp-binary"

	provider := newACPProvider(cfg, "codex")
	events, result, err := provider.RunIteration(context.Background(), IterationRequest{Prompt: "hello"})
	if err == nil {
		t.Fatal("expected pre-start error")
	}
	if events != nil {
		t.Fatal("expected nil event channel on pre-start error")
	}
	if result.Success {
		t.Fatalf("expected result.Success=false on pre-start error, got %+v", result)
	}
}

func TestResolveACPCommandDefaults(t *testing.T) {
	t.Parallel()

	cases := []struct {
		provider string
		binary   string
		args     []string
	}{
		{provider: "codex", binary: "codex-acp"},
		{provider: "claude", binary: "claude-agent-acp"},
		{provider: "pi", binary: "pi-acp"},
		{provider: "gemini", binary: "gemini", args: []string{"acp"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.provider, func(t *testing.T) {
			t.Parallel()
			cmd := resolveACPCommand(tc.provider, "")
			if cmd.Binary != tc.binary {
				t.Fatalf("binary: got %q want %q", cmd.Binary, tc.binary)
			}
			if strings.Join(cmd.Args, ",") != strings.Join(tc.args, ",") {
				t.Fatalf("args: got %v want %v", cmd.Args, tc.args)
			}
		})
	}
}

func TestACPProviderResumesPersistedSession(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Cleanup(CloseAllSessions)

	workDir := t.TempDir()
	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Codex.ACPCommand = helperACPCommand("acp-resume-helper")

	provider := newACPProvider(cfg, "codex")
	acp, ok := provider.(acpProvider)
	if !ok {
		t.Fatalf("unexpected provider type: %T", provider)
	}

	sessionKey := acp.sessionKey(workDir)
	now := time.Now().UTC()
	if err := acp.savePersistedSession(workDir, sessionKey, "sess-resume-1", now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatalf("save persisted session: %v", err)
	}

	events, _, err := provider.RunIteration(context.Background(), IterationRequest{
		WorkDir: workDir,
		Prompt:  "hello",
	})
	if err != nil {
		t.Fatalf("run iteration: %v", err)
	}
	gotEvents := collectEvents(events)
	if !containsAssistantText(gotEvents, "resumed ok") {
		t.Fatalf("expected resumed response, got events: %+v", gotEvents)
	}

	persistedID, hasPersisted := acp.loadPersistedSession(workDir, sessionKey)
	if !hasPersisted {
		t.Fatal("expected persisted session to remain after successful resume")
	}
	if persistedID != "sess-resume-1" {
		t.Fatalf("unexpected persisted session id: %q", persistedID)
	}
}

func TestACPProviderFallsBackToSessionNewWhenResumeUnsupported(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Cleanup(CloseAllSessions)

	workDir := t.TempDir()
	cfg := config.Defaults()
	cfg.Providers.Codex.Enabled = true
	cfg.Providers.Codex.ACPCommand = helperACPCommand("acp-no-resume-helper")

	provider := newACPProvider(cfg, "codex")
	acp, ok := provider.(acpProvider)
	if !ok {
		t.Fatalf("unexpected provider type: %T", provider)
	}

	sessionKey := acp.sessionKey(workDir)
	now := time.Now().UTC()
	if err := acp.savePersistedSession(workDir, sessionKey, "sess-old", now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatalf("save persisted session: %v", err)
	}

	events, _, err := provider.RunIteration(context.Background(), IterationRequest{
		WorkDir: workDir,
		Prompt:  "hello",
	})
	if err != nil {
		t.Fatalf("run iteration: %v", err)
	}
	gotEvents := collectEvents(events)
	if !containsAssistantText(gotEvents, "new session ok") {
		t.Fatalf("expected new-session response, got events: %+v", gotEvents)
	}

	persistedID, hasPersisted := acp.loadPersistedSession(workDir, sessionKey)
	if !hasPersisted {
		t.Fatal("expected persisted session after fallback")
	}
	if persistedID != "sess-fallback-new" {
		t.Fatalf("expected fallback session id persisted, got %q", persistedID)
	}
}

func TestResolveACPSessionCachePathUsesDaedalusDir(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	path, err := resolveACPSessionCachePath(workDir)
	if err != nil {
		t.Fatalf("resolve cache path: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join(".daedalus", "acp-sessions.json")) {
		t.Fatalf("unexpected cache path: %s", path)
	}
}

func containsEventType(events []Event, target EventType) bool {
	for _, event := range events {
		if event.Type == target {
			return true
		}
	}
	return false
}

func containsAssistantText(events []Event, target string) bool {
	for _, event := range events {
		if event.Type != EventAssistantText {
			continue
		}
		if strings.TrimSpace(event.Message) == target {
			return true
		}
	}
	return false
}

func helperACPCommand(mode string) string {
	return fmt.Sprintf("%s -test.run=TestACPHelperProcess -- %s", os.Args[0], mode)
}

func TestACPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	mode := ""
	for i := range os.Args {
		if os.Args[i] == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
			break
		}
	}
	if mode == "" {
		os.Exit(2)
	}
	if mode != "acp-helper" && mode != "acp-error-helper" && mode != "acp-resume-helper" && mode != "acp-no-resume-helper" {
		os.Exit(2)
	}

	runACPHelperServer(mode)
	os.Exit(0)
}

func runACPHelperServer(mode string) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	writer := bufio.NewWriter(os.Stdout)
	defer func() {
		_ = writer.Flush()
	}()

	resumeSucceeded := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req acpJSONRPC
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		switch req.Method {
		case "initialize":
			writeRPC(writer, acpJSONRPC{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mustMarshalJSON(map[string]string{"status": "ok"}),
			})
		case "session/new":
			if mode == "acp-resume-helper" {
				writeRPC(writer, acpJSONRPC{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &acpError{Code: -32000, Message: "session/new should not be called in resume mode"},
				})
				continue
			}
			if mode == "acp-no-resume-helper" {
				writeRPC(writer, acpJSONRPC{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  mustMarshalJSON(acpSessionResult{SessionID: "sess-fallback-new"}),
				})
				continue
			}
			writeRPC(writer, acpJSONRPC{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mustMarshalJSON(acpSessionResult{SessionID: "sess-test-1"}),
			})
		case "session/resume":
			if mode == "acp-no-resume-helper" {
				writeRPC(writer, acpJSONRPC{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &acpError{Code: -32601, Message: "method not found"},
				})
				continue
			}

			var resumeParams acpResumeParams
			_ = json.Unmarshal(req.Params, &resumeParams)
			resumeID := strings.TrimSpace(resumeParams.SessionID)
			if resumeID == "" {
				resumeID = "sess-resume-1"
			}
			resumeSucceeded = true
			writeRPC(writer, acpJSONRPC{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mustMarshalJSON(acpSessionResult{SessionID: resumeID}),
			})
		case "session/prompt":
			if mode == "acp-error-helper" {
				writeRPC(writer, acpJSONRPC{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   &acpError{Code: -32000, Message: "simulated provider failure"},
				})
				continue
			}
			if mode == "acp-resume-helper" && resumeSucceeded {
				writeRPC(writer, acpJSONRPC{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  mustMarshalJSON(acpPromptResult{StopReason: "end_turn", Output: []acpContentBlock{{Type: "text", Text: "resumed ok"}}}),
				})
				continue
			}
			if mode == "acp-no-resume-helper" {
				writeRPC(writer, acpJSONRPC{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  mustMarshalJSON(acpPromptResult{StopReason: "end_turn", Output: []acpContentBlock{{Type: "text", Text: "new session ok"}}}),
				})
				continue
			}
			writeRPC(writer, acpJSONRPC{
				JSONRPC: "2.0",
				Method:  "session/update",
				Params:  mustMarshalJSON(acpSessionUpdate{Content: "hello "}),
			})
			writeRPC(writer, acpJSONRPC{
				JSONRPC: "2.0",
				Method:  "session/update",
				Params:  mustMarshalJSON(acpSessionUpdate{ToolName: "shell", Event: "start"}),
			})
			writeRPC(writer, acpJSONRPC{
				JSONRPC: "2.0",
				Method:  "session/update",
				Params:  mustMarshalJSON(acpSessionUpdate{ToolName: "shell", Event: "done"}),
			})
			writeRPC(writer, acpJSONRPC{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mustMarshalJSON(acpPromptResult{StopReason: "end_turn", Output: []acpContentBlock{{Type: "text", Text: "world"}}}),
			})
		case "session/cancel":
			// no-op for helper
		default:
			writeRPC(writer, acpJSONRPC{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &acpError{Code: -32601, Message: "method not found"},
			})
		}
	}
}

func writeRPC(writer *bufio.Writer, msg acpJSONRPC) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_, _ = writer.Write(payload)
	_, _ = writer.WriteString("\n")
	_ = writer.Flush()
}
