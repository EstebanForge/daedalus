package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
	"github.com/EstebanForge/daedalus/internal/project"
)

const acpInitTimeout = 30 * time.Second
const (
	acpSessionIdleTimeout = 20 * time.Minute
	acpSessionMaxAge      = 2 * time.Hour
	acpPersistedMaxAge    = 24 * time.Hour
)

type acpProvider struct {
	cfg         config.GenericProviderConfig
	providerKey string
	command     acpCommand
}

type acpCommand struct {
	Binary string
	Args   []string
}

var (
	acpSessions   = make(map[string]*acpSessionState)
	acpSessionsMu sync.RWMutex

	acpPersistenceMu sync.Mutex

	acpCapabilities   = make(map[string]Capabilities)
	acpCapabilitiesMu sync.RWMutex
)

type acpReadResult struct {
	line string
	err  error
}

type acpSessionState struct {
	ID          string
	Cwd         string
	Cmd         *exec.Cmd
	Stdin       io.WriteCloser
	ReadResults <-chan acpReadResult
	StderrLines <-chan string
	startedAt   time.Time
	lastUsedAt  time.Time

	requestMu sync.Mutex
	writeMu   sync.Mutex
	idMu      sync.Mutex
	useMu     sync.Mutex
	capMu     sync.RWMutex
	messageID int

	capabilities Capabilities
}

type acpSessionCache struct {
	Version  int                              `json:"version"`
	Sessions map[string]acpSessionCacheRecord `json:"sessions"`
}

type acpSessionCacheRecord struct {
	ProviderKey string `json:"providerKey"`
	WorkDir     string `json:"workDir"`
	Command     string `json:"command"`
	SessionID   string `json:"sessionId"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// CloseAllSessions terminates all active ACP sessions managed by this package.
func CloseAllSessions() {
	acpSessionsMu.Lock()
	sessions := make([]*acpSessionState, 0, len(acpSessions))
	for key, session := range acpSessions {
		delete(acpSessions, key)
		sessions = append(sessions, session)
	}
	acpSessionsMu.Unlock()

	for _, session := range sessions {
		closeACPSession(session)
	}

	acpCapabilitiesMu.Lock()
	clear(acpCapabilities)
	acpCapabilitiesMu.Unlock()
}

func newACPProvider(cfg config.Config, providerKey string) Provider {
	key := strings.ToLower(strings.TrimSpace(providerKey))
	providerCfg := getProviderConfig(cfg, key)

	return acpProvider{
		cfg:         providerCfg,
		providerKey: key,
		command:     resolveACPCommand(key, providerCfg.ACPCommand),
	}
}

func getProviderConfig(cfg config.Config, agent string) config.GenericProviderConfig {
	agent = strings.ToLower(agent)
	switch agent {
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

func resolveACPCommand(providerKey, override string) acpCommand {
	raw := strings.TrimSpace(override)
	if raw != "" {
		parts := strings.Fields(raw)
		if len(parts) > 0 {
			return acpCommand{Binary: parts[0], Args: parts[1:]}
		}
	}

	switch providerKey {
	case "codex":
		return acpCommand{Binary: "codex-acp"}
	case "claude":
		return acpCommand{Binary: "claude-agent-acp"}
	case "pi":
		return acpCommand{Binary: "pi-acp"}
	default:
		return acpCommand{Binary: providerKey, Args: []string{"acp"}}
	}
}

func (p acpProvider) Name() string {
	return p.providerKey
}

func (p acpProvider) Capabilities() Capabilities {
	if negotiated, ok := p.loadNegotiatedCapabilities(); ok {
		return negotiated
	}
	return p.defaultCapabilities()
}

func (p acpProvider) defaultCapabilities() Capabilities {
	return Capabilities{
		Streaming:       true,
		ToolCalls:       true,
		SandboxControl:  true,
		ApprovalModes:   []string{"on-failure", "on-request", "never"},
		ModelSelection:  true,
		SupportedModels: nil,
		MaxContextHint:  0,
	}
}

func (p acpProvider) capabilitiesKey() string {
	return p.providerKey + "\x00" + p.commandFingerprint()
}

func (p acpProvider) saveNegotiatedCapabilities(c Capabilities) {
	acpCapabilitiesMu.Lock()
	acpCapabilities[p.capabilitiesKey()] = c
	acpCapabilitiesMu.Unlock()
}

func (p acpProvider) loadNegotiatedCapabilities() (Capabilities, bool) {
	acpCapabilitiesMu.RLock()
	defer acpCapabilitiesMu.RUnlock()
	c, ok := acpCapabilities[p.capabilitiesKey()]
	if !ok {
		return Capabilities{}, false
	}
	return c, true
}

func (p acpProvider) RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, IterationResult{}, NewConfigurationError("acp prompt is required", nil)
	}

	session, sessionKey, err := p.ensureSession(request.WorkDir)
	if err != nil {
		return nil, IterationResult{}, err
	}
	if err := p.validateRequest(request, session.getCapabilities()); err != nil {
		return nil, IterationResult{}, err
	}

	events := make(chan Event, 16)
	pushProviderEvent(events, EventIterationStarted, fmt.Sprintf("%s ACP iteration started", p.providerKey))

	go func() {
		defer close(events)

		session.requestMu.Lock()
		defer session.requestMu.Unlock()

		stderrStop := make(chan struct{})
		go p.forwardStderr(session, events, stderrStop)
		defer close(stderrStop)

		fullPrompt, promptErr := p.buildPromptWithContext(request.WorkDir, request.Prompt, request.ContextFiles)
		if promptErr != nil {
			pushProviderEvent(events, EventError, EncodeEventError(promptErr))
			pushProviderEvent(events, EventIterationDone, "acp iteration failed")
			return
		}

		var responseText strings.Builder
		promptReq := acpJSONRPC{
			JSONRPC: "2.0",
			ID:      session.nextMessageID(),
			Method:  "session/prompt",
			Params: mustMarshalJSON(acpPromptParams{
				SessionID: session.ID,
				Prompt: []acpContentBlock{
					{Type: "text", Text: fullPrompt},
				},
			}),
		}

		resp, reqErr := p.requestRPC(ctx, session, promptReq, events, &responseText)
		if reqErr != nil {
			mappedErr := mapACPError(p.command.Binary, reqErr)
			pushProviderEvent(events, EventError, EncodeEventError(mappedErr))
			pushProviderEvent(events, EventIterationDone, "acp iteration failed")
			if errors.Is(reqErr, io.EOF) || strings.Contains(strings.ToLower(reqErr.Error()), "broken pipe") {
				p.invalidateSession(sessionKey, session)
				_ = p.deletePersistedSession(session.Cwd, sessionKey)
			}
			return
		}

		if resp.Error != nil {
			pushProviderEvent(events, EventError, EncodeEventError(fmt.Errorf("acp error: %s", resp.Error.Message)))
			pushProviderEvent(events, EventIterationDone, "acp iteration failed")
			return
		}

		if resultText := p.extractContentFromPromptResult(resp.Result); resultText != "" {
			responseText.WriteString(resultText)
		}

		summary := strings.TrimSpace(responseText.String())
		if summary == "" {
			summary = "iteration completed"
		}
		now := time.Now()
		session.markUsed(now)
		_ = p.savePersistedSession(session.Cwd, sessionKey, session.ID, session.startedAt, now)
		pushProviderEvent(events, EventAssistantText, summary)
		pushProviderEvent(events, EventIterationDone, "acp iteration finished")
	}()

	return events, IterationResult{Success: true}, nil
}

func (p acpProvider) ensureSession(workDir string) (*acpSessionState, string, error) {
	sessionKey := p.sessionKey(workDir)

	acpSessionsMu.RLock()
	existing := acpSessions[sessionKey]
	acpSessionsMu.RUnlock()
	now := time.Now()
	if existing != nil && isProcessAlive(existing.Cmd) && !existing.isExpired(now) {
		existing.markUsed(now)
		p.saveNegotiatedCapabilities(existing.getCapabilities())
		_ = p.savePersistedSession(existing.Cwd, sessionKey, existing.ID, existing.startedAt, now)
		return existing, sessionKey, nil
	}
	if existing != nil {
		p.invalidateSession(sessionKey, existing)
	}

	session, err := p.startSession(workDir)
	if err != nil {
		return nil, "", err
	}

	initCtx, cancel := context.WithTimeout(context.Background(), acpInitTimeout)
	defer cancel()
	if err := p.initializeTransport(initCtx, session); err != nil {
		p.closeSession(session)
		return nil, "", err
	}

	persistedSessionID, hasPersisted := p.loadPersistedSession(workDir, sessionKey)
	if hasPersisted {
		if err := p.resumeSession(initCtx, session, persistedSessionID); err == nil {
			now := time.Now()
			session.markUsed(now)
			_ = p.savePersistedSession(workDir, sessionKey, session.ID, session.startedAt, now)
			acpSessionsMu.Lock()
			acpSessions[sessionKey] = session
			acpSessionsMu.Unlock()
			return session, sessionKey, nil
		}
		_ = p.deletePersistedSession(workDir, sessionKey)
	}

	if err := p.createSession(initCtx, session, workDir); err != nil {
		p.closeSession(session)
		return nil, "", err
	}

	now = time.Now()
	session.markUsed(now)
	_ = p.savePersistedSession(workDir, sessionKey, session.ID, session.startedAt, now)
	acpSessionsMu.Lock()
	acpSessions[sessionKey] = session
	acpSessionsMu.Unlock()

	return session, sessionKey, nil
}

func (p acpProvider) startSession(workDir string) (*acpSessionState, error) {
	resolvedWorkDir := canonicalWorkDir(workDir)

	cmd := exec.Command(p.command.Binary, p.command.Args...)
	if strings.TrimSpace(resolvedWorkDir) != "" {
		cmd.Dir = resolvedWorkDir
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, NewConfigurationError("failed to create ACP stdin pipe", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, NewConfigurationError("failed to create ACP stdout pipe", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, NewConfigurationError("failed to create ACP stderr pipe", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, mapACPError(p.command.Binary, err)
	}

	// Process is reaped in background when it exits.
	go func() {
		_ = cmd.Wait()
	}()

	now := time.Now()
	defaultCapabilities := p.defaultCapabilities()
	return &acpSessionState{
		Cwd:          resolvedWorkDir,
		Cmd:          cmd,
		Stdin:        stdin,
		ReadResults:  startJSONLineReader(stdout),
		StderrLines:  startTextLineReader(stderr),
		startedAt:    now,
		lastUsedAt:   now,
		messageID:    1,
		capabilities: defaultCapabilities,
	}, nil
}

func (p acpProvider) initializeTransport(ctx context.Context, session *acpSessionState) error {
	initReq := acpJSONRPC{
		JSONRPC: "2.0",
		ID:      session.nextMessageID(),
		Method:  "initialize",
		Params: mustMarshalJSON(acpInitializeParams{
			ProtocolVersion: "1",
			ClientCapabilities: acpClientCapabilities{
				FS: acpFSCapabilities{
					ReadTextFile:  true,
					WriteTextFile: true,
				},
				Terminal: true,
			},
			ClientInfo: acpClientInfo{
				Name:    "daedalus",
				Title:   "Daedalus",
				Version: "1.0.0",
			},
		}),
	}

	initResp, err := p.requestRPC(ctx, session, initReq, nil, nil)
	if err != nil {
		return NewConfigurationError("failed to initialize ACP session", err)
	}
	if initResp.Error != nil {
		return fmt.Errorf("ACP initialize error: %s", initResp.Error.Message)
	}

	capabilities := p.defaultCapabilities()
	if negotiated, ok := parseInitializeCapabilities(initResp.Result, capabilities); ok {
		capabilities = negotiated
	}
	session.setCapabilities(capabilities)
	p.saveNegotiatedCapabilities(capabilities)

	return nil
}

func (p acpProvider) createSession(ctx context.Context, session *acpSessionState, workDir string) error {
	sessionReq := acpJSONRPC{
		JSONRPC: "2.0",
		ID:      session.nextMessageID(),
		Method:  "session/new",
		Params: mustMarshalJSON(acpSessionParams{
			Cwd:        workDir,
			McpServers: []string{},
		}),
	}

	sessionResp, err := p.requestRPC(ctx, session, sessionReq, nil, nil)
	if err != nil {
		return NewConfigurationError("failed to create ACP session", err)
	}
	if sessionResp.Error != nil {
		return fmt.Errorf("ACP session error: %s", sessionResp.Error.Message)
	}

	var sessionResult acpSessionResult
	if err := json.Unmarshal(sessionResp.Result, &sessionResult); err != nil {
		return NewConfigurationError("failed to parse ACP session result", err)
	}
	if strings.TrimSpace(sessionResult.SessionID) == "" {
		return NewConfigurationError("ACP session did not return sessionId", nil)
	}

	session.ID = sessionResult.SessionID
	return nil
}

func (p acpProvider) resumeSession(ctx context.Context, session *acpSessionState, persistedSessionID string) error {
	resumeID := strings.TrimSpace(persistedSessionID)
	if resumeID == "" {
		return NewConfigurationError("missing ACP session id for resume", nil)
	}

	resumeReq := acpJSONRPC{
		JSONRPC: "2.0",
		ID:      session.nextMessageID(),
		Method:  "session/resume",
		Params:  mustMarshalJSON(acpResumeParams{SessionID: resumeID}),
	}

	resumeResp, err := p.requestRPC(ctx, session, resumeReq, nil, nil)
	if err != nil {
		return err
	}
	if resumeResp.Error != nil {
		return fmt.Errorf("ACP resume error: %s", resumeResp.Error.Message)
	}

	resolvedID := strings.TrimSpace(parseSessionIDFromResult(resumeResp.Result))
	if resolvedID == "" {
		resolvedID = resumeID
	}
	session.ID = resolvedID
	return nil
}

func (p acpProvider) requestRPC(
	ctx context.Context,
	session *acpSessionState,
	req acpJSONRPC,
	events chan Event,
	responseText *strings.Builder,
) (acpJSONRPC, error) {
	if err := p.sendJSON(session, req); err != nil {
		return acpJSONRPC{}, err
	}

	for {
		line, err := p.readLine(ctx, session)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				p.cancelSession(session, session.ID)
			}
			return acpJSONRPC{}, err
		}

		var resp acpJSONRPC
		if unmarshalErr := json.Unmarshal([]byte(line), &resp); unmarshalErr != nil {
			pushProviderEvent(events, EventCommandOutput, line)
			continue
		}

		if resp.Method == "session/update" {
			p.handleSessionUpdate(resp.Params, events, responseText)
			continue
		}

		if resp.ID == req.ID {
			return resp, nil
		}

		if resp.ID == 0 && resp.Error != nil {
			return acpJSONRPC{}, fmt.Errorf("acp error: %s", resp.Error.Message)
		}
	}
}

func (p acpProvider) sendJSON(session *acpSessionState, req acpJSONRPC) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	_, err = session.Stdin.Write(data)
	return err
}

func (p acpProvider) readLine(ctx context.Context, session *acpSessionState) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case result, ok := <-session.ReadResults:
			if !ok {
				return "", io.EOF
			}
			if result.err != nil {
				return "", result.err
			}
			line := strings.TrimSpace(result.line)
			if line == "" {
				continue
			}
			return line, nil
		}
	}
}

func (p acpProvider) cancelSession(session *acpSessionState, sessionID string) {
	if session == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	cancelReq := acpJSONRPC{
		JSONRPC: "2.0",
		Method:  "session/cancel",
		Params:  mustMarshalJSON(acpCancelParams{SessionID: sessionID}),
	}
	_ = p.sendJSON(session, cancelReq)
}

func (p acpProvider) forwardStderr(session *acpSessionState, events chan Event, stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case line, ok := <-session.StderrLines:
			if !ok {
				return
			}
			if strings.TrimSpace(line) != "" {
				pushProviderEvent(events, EventCommandOutput, fmt.Sprintf("[%s stderr] %s", p.providerKey, line))
			}
		}
	}
}

func (p acpProvider) handleSessionUpdate(params json.RawMessage, events chan Event, responseText *strings.Builder) {
	var update acpSessionUpdate
	if err := json.Unmarshal(params, &update); err != nil {
		return
	}

	text := firstNonEmptyRaw(update.Content, update.Text, update.Message)
	if strings.TrimSpace(text) != "" {
		if responseText != nil {
			responseText.WriteString(text)
		}
		pushProviderEvent(events, EventAssistantText, text)
	}

	toolName := firstNonEmpty(update.Tool, update.ToolName, update.Name)
	phase := strings.ToLower(firstNonEmpty(update.Event, update.Status, update.State, update.Type))
	if toolName == "" || phase == "" {
		return
	}

	switch {
	case containsAny(phase, "start", "begin", "call", "running", "tool_call"):
		pushProviderEvent(events, EventToolStarted, toolName)
	case containsAny(phase, "finish", "end", "done", "complete", "result", "tool_result"):
		pushProviderEvent(events, EventToolFinished, toolName)
	}
}

func (p acpProvider) extractContentFromPromptResult(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var result acpPromptResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return ""
	}

	if text := extractContentBlocksText(result.Output); text != "" {
		return text
	}
	if text := extractContentBlocksText(result.Content); text != "" {
		return text
	}
	return strings.TrimSpace(result.Message)
}

func extractContentBlocksText(blocks []acpContentBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		sb.WriteString(text)
	}
	return strings.TrimSpace(sb.String())
}

func (p acpProvider) buildPromptWithContext(workDir, prompt string, paths []string) (string, error) {
	fullPrompt := prompt
	if len(paths) == 0 {
		return fullPrompt, nil
	}
	contextContent, err := p.readContextFiles(workDir, paths)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(contextContent) == "" {
		return fullPrompt, nil
	}
	return fmt.Sprintf("%s\n\n--- Context Files ---\n%s", prompt, contextContent), nil
}

func (p acpProvider) readContextFiles(workDir string, paths []string) (string, error) {
	var builder strings.Builder
	for _, path := range paths {
		resolved := path
		if !filepath.IsAbs(path) && strings.TrimSpace(workDir) != "" {
			resolved = filepath.Join(workDir, path)
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			continue
		}
		builder.WriteString("\n--- ")
		builder.WriteString(path)
		builder.WriteString(" ---\n")
		builder.Write(data)
	}
	return builder.String(), nil
}

func (p acpProvider) invalidateSession(sessionKey string, session *acpSessionState) {
	acpSessionsMu.Lock()
	if current, ok := acpSessions[sessionKey]; ok && current == session {
		delete(acpSessions, sessionKey)
	}
	acpSessionsMu.Unlock()
	closeACPSession(session)
}

func (p acpProvider) closeSession(session *acpSessionState) {
	closeACPSession(session)
}

func (p acpProvider) sessionKey(workDir string) string {
	if strings.TrimSpace(workDir) == "" {
		return p.providerKey
	}
	return p.providerKey + ":" + canonicalWorkDir(workDir)
}

func (s *acpSessionState) nextMessageID() int {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	id := s.messageID
	s.messageID++
	return id
}

func (s *acpSessionState) markUsed(now time.Time) {
	s.useMu.Lock()
	defer s.useMu.Unlock()
	s.lastUsedAt = now
}

func (s *acpSessionState) setCapabilities(capabilities Capabilities) {
	s.capMu.Lock()
	defer s.capMu.Unlock()
	s.capabilities = capabilities
}

func (s *acpSessionState) getCapabilities() Capabilities {
	s.capMu.RLock()
	defer s.capMu.RUnlock()
	return s.capabilities
}

func (s *acpSessionState) isExpired(now time.Time) bool {
	s.useMu.Lock()
	lastUsed := s.lastUsedAt
	started := s.startedAt
	s.useMu.Unlock()

	if now.Sub(lastUsed) > acpSessionIdleTimeout {
		return true
	}
	if now.Sub(started) > acpSessionMaxAge {
		return true
	}
	return false
}

func closeACPSession(session *acpSessionState) {
	if session == nil {
		return
	}
	if session.Stdin != nil {
		_ = session.Stdin.Close()
	}
	if session.Cmd != nil && session.Cmd.Process != nil {
		_ = session.Cmd.Process.Kill()
	}
}

func startJSONLineReader(reader io.Reader) <-chan acpReadResult {
	out := make(chan acpReadResult, 64)
	go func() {
		defer close(out)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			out <- acpReadResult{line: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			out <- acpReadResult{err: err}
		}
	}()
	return out
}

func startTextLineReader(reader io.Reader) <-chan string {
	out := make(chan string, 64)
	go func() {
		defer close(out)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				out <- line
			}
		}
	}()
	return out
}

func isProcessAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return false
	}
	return cmd.Process.Signal(syscall.Signal(0)) == nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptyRaw(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func containsAny(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}

func (p acpProvider) validateRequest(request IterationRequest, capabilities Capabilities) error {
	approval := strings.TrimSpace(strings.ToLower(request.ApprovalPolicy))
	if approval != "" && len(capabilities.ApprovalModes) > 0 && !containsStringFold(capabilities.ApprovalModes, approval) {
		return NewConfigurationError(
			fmt.Sprintf(
				"provider %q does not support approval policy %q (supported: %s)",
				p.providerKey,
				request.ApprovalPolicy,
				strings.Join(capabilities.ApprovalModes, ", "),
			),
			nil,
		)
	}

	sandbox := strings.TrimSpace(request.SandboxPolicy)
	if sandbox != "" && !capabilities.SandboxControl {
		return NewConfigurationError(
			fmt.Sprintf("provider %q does not support sandbox policy overrides", p.providerKey),
			nil,
		)
	}

	model := strings.TrimSpace(request.Model)
	if model == "" || strings.EqualFold(model, "default") {
		return nil
	}
	if !capabilities.ModelSelection {
		return NewConfigurationError(
			fmt.Sprintf("provider %q does not support explicit model selection", p.providerKey),
			nil,
		)
	}
	if len(capabilities.SupportedModels) > 0 && !containsStringFold(capabilities.SupportedModels, model) {
		return NewConfigurationError(
			fmt.Sprintf(
				"provider %q does not support model %q (supported: %s)",
				p.providerKey,
				model,
				strings.Join(capabilities.SupportedModels, ", "),
			),
			nil,
		)
	}
	return nil
}

func containsStringFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func parseInitializeCapabilities(raw json.RawMessage, fallback Capabilities) (Capabilities, bool) {
	if len(raw) == 0 {
		return fallback, false
	}

	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return fallback, false
	}

	capabilityNode := root
	if nested, ok := root["serverCapabilities"].(map[string]interface{}); ok {
		capabilityNode = nested
	} else if nested, ok := root["capabilities"].(map[string]interface{}); ok {
		capabilityNode = nested
	}

	parsed := fallback
	updated := false

	if value, ok := boolFromAny(capabilityNode["streaming"]); ok {
		parsed.Streaming = value
		updated = true
	}
	if value, ok := boolFromAny(firstKnownKey(capabilityNode, "toolCalls", "tools")); ok {
		parsed.ToolCalls = value
		updated = true
	}
	if value, ok := boolFromAny(firstKnownKey(capabilityNode, "sandboxControl", "sandbox")); ok {
		parsed.SandboxControl = value
		updated = true
	}

	if modes, ok := stringSliceFromAny(firstKnownKey(capabilityNode, "approvalModes", "approvalPolicies")); ok {
		parsed.ApprovalModes = modes
		updated = true
	} else if approvalNode, ok := capabilityNode["approval"].(map[string]interface{}); ok {
		if modes, ok := stringSliceFromAny(firstKnownKey(approvalNode, "modes", "approvalModes")); ok {
			parsed.ApprovalModes = modes
			updated = true
		}
	}

	if value, ok := boolFromAny(firstKnownKey(capabilityNode, "modelSelection", "modelControl")); ok {
		parsed.ModelSelection = value
		updated = true
	}
	if models, ok := stringSliceFromAny(firstKnownKey(capabilityNode, "supportedModels", "models")); ok {
		parsed.SupportedModels = models
		updated = true
	}
	if value, ok := intFromAny(firstKnownKey(capabilityNode, "maxContextHint", "maxContextTokens")); ok {
		parsed.MaxContextHint = value
		updated = true
	}

	return parsed, updated
}

func firstKnownKey(m map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value
		}
	}
	return nil
}

func boolFromAny(value interface{}) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		switch normalized {
		case "1", "true", "yes", "on":
			return true, true
		case "0", "false", "no", "off":
			return false, true
		}
	}
	return false, false
}

func intFromAny(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case json.Number:
		number, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(number), true
	}
	return 0, false
}

func stringSliceFromAny(value interface{}) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, raw := range typed {
			switch value := raw.(type) {
			case string:
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					out = append(out, trimmed)
				}
			case map[string]interface{}:
				if idValue, ok := value["id"].(string); ok {
					if trimmed := strings.TrimSpace(idValue); trimmed != "" {
						out = append(out, trimmed)
						continue
					}
				}
				if nameValue, ok := value["name"].(string); ok {
					if trimmed := strings.TrimSpace(nameValue); trimmed != "" {
						out = append(out, trimmed)
					}
				}
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	default:
		return nil, false
	}
}

func parseSessionIDFromResult(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var typed acpSessionResult
	if err := json.Unmarshal(raw, &typed); err == nil && strings.TrimSpace(typed.SessionID) != "" {
		return typed.SessionID
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return ""
	}
	if value, ok := generic["sessionId"].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	if value, ok := generic["id"].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return ""
}

func (p acpProvider) commandFingerprint() string {
	parts := append([]string{p.command.Binary}, p.command.Args...)
	return strings.Join(parts, "\x00")
}

func (p acpProvider) loadPersistedSession(workDir, sessionKey string) (string, bool) {
	cachePath, err := resolveACPSessionCachePath(workDir)
	if err != nil {
		return "", false
	}

	acpPersistenceMu.Lock()
	defer acpPersistenceMu.Unlock()

	cache, err := readSessionCache(cachePath)
	if err != nil {
		return "", false
	}
	record, exists := cache.Sessions[sessionKey]
	if !exists {
		return "", false
	}
	if !p.isPersistedRecordValid(record, workDir) {
		delete(cache.Sessions, sessionKey)
		_ = writeSessionCache(cachePath, cache)
		return "", false
	}
	return record.SessionID, true
}

func (p acpProvider) savePersistedSession(workDir, sessionKey, sessionID string, startedAt, updatedAt time.Time) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}

	cachePath, err := resolveACPSessionCachePath(workDir)
	if err != nil {
		return err
	}

	acpPersistenceMu.Lock()
	defer acpPersistenceMu.Unlock()

	cache, err := readSessionCache(cachePath)
	if err != nil {
		return err
	}
	if cache.Sessions == nil {
		cache.Sessions = make(map[string]acpSessionCacheRecord)
	}

	existing := cache.Sessions[sessionKey]
	createdAt := existing.CreatedAt
	if strings.TrimSpace(createdAt) == "" {
		createdAt = startedAt.UTC().Format(time.RFC3339)
	}
	cache.Sessions[sessionKey] = acpSessionCacheRecord{
		ProviderKey: p.providerKey,
		WorkDir:     canonicalWorkDir(workDir),
		Command:     p.commandFingerprint(),
		SessionID:   sessionID,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt.UTC().Format(time.RFC3339),
	}

	return writeSessionCache(cachePath, cache)
}

func (p acpProvider) deletePersistedSession(workDir, sessionKey string) error {
	cachePath, err := resolveACPSessionCachePath(workDir)
	if err != nil {
		return err
	}

	acpPersistenceMu.Lock()
	defer acpPersistenceMu.Unlock()

	cache, err := readSessionCache(cachePath)
	if err != nil {
		return err
	}
	if cache.Sessions == nil {
		return nil
	}
	if _, ok := cache.Sessions[sessionKey]; !ok {
		return nil
	}
	delete(cache.Sessions, sessionKey)
	return writeSessionCache(cachePath, cache)
}

func (p acpProvider) isPersistedRecordValid(record acpSessionCacheRecord, workDir string) bool {
	if strings.TrimSpace(record.SessionID) == "" {
		return false
	}
	if record.ProviderKey != p.providerKey {
		return false
	}
	if record.Command != p.commandFingerprint() {
		return false
	}
	if record.WorkDir != canonicalWorkDir(workDir) {
		return false
	}

	updatedAt, err := time.Parse(time.RFC3339, record.UpdatedAt)
	if err != nil {
		return false
	}
	if time.Since(updatedAt) > acpPersistedMaxAge {
		return false
	}
	return true
}

func resolveACPSessionCachePath(workDir string) (string, error) {
	base := canonicalWorkDir(workDir)
	if strings.TrimSpace(base) == "" {
		resolved, err := os.Getwd()
		if err != nil {
			return "", err
		}
		base = resolved
	}
	return project.ACPSessionsPath(base), nil
}

func canonicalWorkDir(workDir string) string {
	trimmed := strings.TrimSpace(workDir)
	if trimmed == "" {
		return ""
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return trimmed
	}
	return abs
}

func readSessionCache(path string) (acpSessionCache, error) {
	cache := acpSessionCache{
		Version:  1,
		Sessions: make(map[string]acpSessionCacheRecord),
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cache, nil
		}
		return acpSessionCache{}, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return cache, nil
	}
	if err := json.Unmarshal(raw, &cache); err != nil {
		return acpSessionCache{}, err
	}
	if cache.Sessions == nil {
		cache.Sessions = make(map[string]acpSessionCacheRecord)
	}
	return cache, nil
}

func writeSessionCache(path string, cache acpSessionCache) error {
	cache.Version = 1
	if cache.Sessions == nil {
		cache.Sessions = make(map[string]acpSessionCacheRecord)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

// ACP JSON-RPC types

type acpJSONRPC struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *acpError       `json:"error,omitempty"`
}

type acpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type acpInitializeParams struct {
	ProtocolVersion    string                `json:"protocolVersion"`
	ClientCapabilities acpClientCapabilities `json:"clientCapabilities"`
	ClientInfo         acpClientInfo         `json:"clientInfo"`
}

type acpClientCapabilities struct {
	FS       acpFSCapabilities `json:"fs,omitempty"`
	Terminal bool              `json:"terminal,omitempty"`
	MCP      bool              `json:"mcp,omitempty"`
}

type acpFSCapabilities struct {
	ReadTextFile  bool `json:"readTextFile,omitempty"`
	WriteTextFile bool `json:"writeTextFile,omitempty"`
}

type acpClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type acpPromptParams struct {
	SessionID string            `json:"sessionId"`
	Prompt    []acpContentBlock `json:"prompt"`
}

type acpContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type acpSessionParams struct {
	Cwd        string   `json:"cwd"`
	McpServers []string `json:"mcpServers"`
}

type acpCancelParams struct {
	SessionID string `json:"sessionId"`
}

type acpResumeParams struct {
	SessionID string `json:"sessionId"`
}

type acpPromptResult struct {
	StopReason string            `json:"stopReason,omitempty"`
	Message    string            `json:"message,omitempty"`
	Output     []acpContentBlock `json:"output,omitempty"`
	Content    []acpContentBlock `json:"content,omitempty"`
}

type acpSessionResult struct {
	SessionID string `json:"sessionId"`
}

type acpSessionUpdate struct {
	Type       string `json:"type,omitempty"`
	Content    string `json:"content,omitempty"`
	Text       string `json:"text,omitempty"`
	Message    string `json:"message,omitempty"`
	Tool       string `json:"tool,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	Name       string `json:"name,omitempty"`
	Status     string `json:"status,omitempty"`
	State      string `json:"state,omitempty"`
	Event      string `json:"event,omitempty"`
	StopReason string `json:"stopReason,omitempty"`
}

func mapACPError(binary string, err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ProviderError{Category: ErrorTimeout, Message: err.Error(), Err: err}
	case errors.Is(err, exec.ErrNotFound):
		return NewConfigurationError(fmt.Sprintf("%s ACP binary not found in PATH", binary), err)
	case errors.Is(err, io.EOF):
		return ProviderError{Category: ErrorTransient, Message: err.Error(), Err: err}
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "executable file not found"), strings.Contains(message, "not found in $path"):
		return NewConfigurationError(fmt.Sprintf("%s ACP binary not found in PATH", binary), err)
	case strings.Contains(message, "authentication"), strings.Contains(message, "auth"), strings.Contains(message, "token"):
		return ProviderError{Category: ErrorAuthentication, Message: err.Error(), Err: err}
	case strings.Contains(message, "rate limit"), strings.Contains(message, "429"):
		return ProviderError{Category: ErrorRateLimit, Message: err.Error(), Err: err}
	case strings.Contains(message, "timeout"), strings.Contains(message, "timed out"), strings.Contains(message, "deadline exceeded"):
		return ProviderError{Category: ErrorTimeout, Message: err.Error(), Err: err}
	case strings.Contains(message, "broken pipe"), strings.Contains(message, "temporar"), strings.Contains(message, "unavailable"), strings.Contains(message, "try again"):
		return ProviderError{Category: ErrorTransient, Message: err.Error(), Err: err}
	default:
		return ProviderError{Category: ErrorFatal, Message: err.Error(), Err: err}
	}
}

func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage{}
	}
	return json.RawMessage(data)
}
