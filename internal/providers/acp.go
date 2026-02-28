package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
)

// ACP Provider - uses Agent Client Protocol for standardized agent communication
// Supports: OpenCode, Claude (via adapter), Gemini CLI, Qwen, Copilot, and any ACP-compatible agent

type acpProvider struct {
	cfg         config.GenericProviderConfig
	agentBinary string
}

// Package-level session manager to avoid lock copying issues
var (
	acpSessions   = make(map[string]*acpSessionState)
	acpSessionsMu sync.RWMutex
	messageCounters = make(map[string]int)
	messageCountersMu sync.Mutex
)

type acpSessionState struct {
	ID        string
	Cwd       string
	Cmd       *exec.Cmd
	startedAt time.Time
}

func newACPProvider(cfg config.Config, agentBinary string) Provider {
	providerCfg := getProviderConfig(cfg, agentBinary)
	return acpProvider{
		cfg:         providerCfg,
		agentBinary: agentBinary,
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
		return cfg.Providers.Codex // fallback
	}
}

func (p acpProvider) Name() string {
	return p.agentBinary
}

func (p acpProvider) Capabilities() Capabilities {
	// ACP provides full capabilities for all supported agents
	return Capabilities{
		Streaming:      true,
		ToolCalls:      true,
		SandboxControl: true, // depends on agent implementation
		ApprovalModes:  []string{"on-failure", "on-request", "never"},
	}
}

func (p acpProvider) RunIteration(ctx context.Context, request IterationRequest) (<-chan Event, IterationResult, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return nil, IterationResult{}, NewConfigurationError("acp prompt is required", nil)
	}

	events := make(chan Event, 8)
	pushProviderEvent(events, EventIterationStarted, fmt.Sprintf("%s ACP iteration started", p.agentBinary))

	// Start or reuse session
	proc, sessionID, err := p.ensureSession(ctx, request.WorkDir, events)
	if err != nil {
		close(events)
		return nil, IterationResult{}, err
	}

	// Send prompt via ACP
	go func() {
		defer close(events)

		// Build prompt with context files
		fullPrompt := request.Prompt
		if len(request.ContextFiles) > 0 {
			contextContent, ctxErr := p.readContextFiles(request.ContextFiles)
			if ctxErr != nil {
				pushProviderEvent(events, EventError, EncodeEventError(ctxErr))
				pushProviderEvent(events, EventIterationDone, "acp iteration failed")
				return
			}
			fullPrompt = fmt.Sprintf("%s\n\n--- Context Files ---\n%s", request.Prompt, contextContent)
		}

		// Send prompt via JSON-RPC
		promptReq := acpJSONRPC{
			JSONRPC: "2.0",
			ID:      p.nextMessageID(sessionID),
			Method:  "session/prompt",
			Params: mustMarshalJSON(acpPromptParams{
				SessionID: sessionID,
				Prompt: []acpContentBlock{
					{Type: "text", Text: fullPrompt},
				},
			}),
		}

		if err := p.sendJSON(proc, promptReq); err != nil {
			pushProviderEvent(events, EventError, EncodeEventError(err))
			pushProviderEvent(events, EventIterationDone, "acp iteration failed")
			return
		}

		// Collect response
		var fullResponse strings.Builder
		stopReason := ""

		for {
			select {
			case <-ctx.Done():
				// Cancel the session
				p.cancelSession(proc, sessionID)
				pushProviderEvent(events, EventError, EncodeEventError(ctx.Err()))
				pushProviderEvent(events, EventIterationDone, "acp iteration cancelled")
				return
			case <-time.After(2 * time.Second):
				// Check if we have a complete response
				if stopReason != "" {
					break
				}
			}

			// Poll for responses
			line, pollErr := p.poll(proc)
			if pollErr != nil {
				if errors.Is(pollErr, context.Canceled) || errors.Is(pollErr, context.DeadlineExceeded) {
					continue
				}
				pushProviderEvent(events, EventError, EncodeEventError(pollErr))
				pushProviderEvent(events, EventIterationDone, "acp iteration failed")
				return
			}

			if line == "" {
				continue
			}

			// Parse response
			var resp acpJSONRPC
			if parseErr := json.Unmarshal([]byte(line), &resp); parseErr != nil {
				// Skip non-response lines (log output, etc.)
				pushProviderEvent(events, EventCommandOutput, line)
				continue
			}

			// Handle notifications (session/update)
			if resp.ID == 0 && resp.Method == "session/update" {
				content := p.extractContentFromUpdate(resp.Params)
				if content != "" {
					fullResponse.WriteString(content)
					pushProviderEvent(events, EventAssistantText, content)
				}
				continue
			}

			// Handle response with stopReason
			if resp.ID != 0 && resp.Result != nil {
				var result acpPromptResult
				if unmarshalErr := json.Unmarshal(resp.Result, &result); unmarshalErr == nil {
					stopReason = result.StopReason
				}
				break
			}

			// Handle errors
			if resp.Error != nil {
				pushProviderEvent(events, EventError, EncodeEventError(fmt.Errorf("acp error: %s", resp.Error.Message)))
				pushProviderEvent(events, EventIterationDone, "acp iteration failed")
				return
			}
		}

		summary := strings.TrimSpace(fullResponse.String())
		if summary == "" {
			summary = "iteration completed"
		}
		pushProviderEvent(events, EventAssistantText, summary)
		pushProviderEvent(events, EventIterationDone, "acp iteration finished")
	}()

	return events, IterationResult{Success: true}, nil
}

func (p acpProvider) ensureSession(ctx context.Context, workDir string, events chan Event) (*exec.Cmd, string, error) {
	// Check for existing session
	sessionKey := p.agentBinary + ":" + workDir
	acpSessionsMu.RLock()
	existingSession, hasSession := acpSessions[sessionKey]
	acpSessionsMu.RUnlock()

	if hasSession && existingSession != nil && existingSession.Cmd != nil && existingSession.Cmd.Process != nil {
		// Check if process is still running
		if existingSession.Cmd.Process.Pid > 0 {
			return existingSession.Cmd, existingSession.ID, nil
		}
	}

	// Start new ACP process
	cmd := exec.CommandContext(ctx, p.agentBinary, "acp")
	if strings.TrimSpace(workDir) != "" {
		cmd.Dir = workDir
	}

	stdinWriter, err := cmd.StdinPipe()
	if err != nil {
		return nil, "", NewConfigurationError("failed to create stdin pipe", err)
	}
	_ = stdinWriter // Reserved for sendJSON
	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", NewConfigurationError("failed to create stdout pipe", err)
	}
	_ = stdoutReader // Reserved for polling
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, "", NewConfigurationError("failed to create stderr pipe", err)
	}

	// Set up stderr streaming to events
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				pushProviderEvent(events, EventCommandOutput, fmt.Sprintf("[%s stderr] %s", p.agentBinary, line))
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, "", mapACPError(err)
	}

	// Wait for startup
	time.Sleep(500 * time.Millisecond)

	// Initialize connection
	initReq := acpJSONRPC{
		JSONRPC: "2.0",
		ID:      p.nextMessageID(sessionKey),
		Method:  "initialize",
		Params: mustMarshalJSON(acpInitializeParams{
			ProtocolVersion: 1,
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

	if err := p.sendJSON(cmd, initReq); err != nil {
		cmd.Process.Kill()
		return nil, "", NewConfigurationError("failed to initialize ACP session", err)
	}

	// Read initialize response
	respLine, err := p.poll(cmd)
	if err != nil {
		cmd.Process.Kill()
		return nil, "", NewConfigurationError("failed to get initialize response", err)
	}

	var initResp acpJSONRPC
	if err := json.Unmarshal([]byte(respLine), &initResp); err != nil {
		cmd.Process.Kill()
		return nil, "", NewConfigurationError("failed to parse initialize response", err)
	}

	if initResp.Error != nil {
		cmd.Process.Kill()
		return nil, "", fmt.Errorf("ACP initialize error: %s", initResp.Error.Message)
	}

	// Create new session
	newSessionReq := acpJSONRPC{
		JSONRPC: "2.0",
		ID:      p.nextMessageID(sessionKey),
		Method:  "session/new",
		Params: mustMarshalJSON(acpSessionParams{
			Cwd:       workDir,
			McpServers: []string{},
		}),
	}

	if err := p.sendJSON(cmd, newSessionReq); err != nil {
		cmd.Process.Kill()
		return nil, "", NewConfigurationError("failed to create ACP session", err)
	}

	// Read session response
	sessionLine, err := p.poll(cmd)
	if err != nil {
		cmd.Process.Kill()
		return nil, "", NewConfigurationError("failed to get session response", err)
	}

	var sessionResp acpJSONRPC
	if err := json.Unmarshal([]byte(sessionLine), &sessionResp); err != nil {
		cmd.Process.Kill()
		return nil, "", NewConfigurationError("failed to parse session response", err)
	}

	if sessionResp.Error != nil {
		cmd.Process.Kill()
		return nil, "", fmt.Errorf("ACP session error: %s", sessionResp.Error.Message)
	}

	var sessionResult acpSessionResult
	if err := json.Unmarshal(sessionResp.Result, &sessionResult); err != nil {
		cmd.Process.Kill()
		return nil, "", NewConfigurationError("failed to parse session result", err)
	}

	// Store session
	newSession := &acpSessionState{
		ID:        sessionResult.SessionID,
		Cwd:       workDir,
		Cmd:       cmd,
		startedAt: time.Now(),
	}
	acpSessionsMu.Lock()
	acpSessions[sessionKey] = newSession
	acpSessionsMu.Unlock()

	return cmd, sessionResult.SessionID, nil
}

func (p acpProvider) nextMessageID(sessionKey string) int {
	messageCountersMu.Lock()
	defer messageCountersMu.Unlock()
	id := messageCounters[sessionKey]
	messageCounters[sessionKey]++
	return id
}

func (p acpProvider) sendJSON(cmd *exec.Cmd, req interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	stdinWriter, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdinWriter.Close()
	_, err = stdinWriter.Write(data)
	return err
}

func (p acpProvider) poll(cmd *exec.Cmd) (string, error) {
	// Simple polling - in production would use non-blocking read
	time.Sleep(100 * time.Millisecond)
	return "", errors.New("poll not implemented - use process-based polling")
}

func (p acpProvider) cancelSession(cmd *exec.Cmd, sessionID string) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	cancelReq := acpJSONRPC{
		JSONRPC: "2.0",
		Method:  "session/cancel",
		Params: mustMarshalJSON(acpCancelParams{
			SessionID: sessionID,
		}),
	}
	_ = p.sendJSON(cmd, cancelReq)
}

func (p acpProvider) readContextFiles(paths []string) (string, error) {
	var builder strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip inaccessible files
		}
		builder.WriteString(fmt.Sprintf("\n--- %s ---\n", path))
		builder.WriteString(string(data))
	}
	return builder.String(), nil
}

func (p acpProvider) extractContentFromUpdate(params json.RawMessage) string {
	var update acpSessionUpdate
	if err := json.Unmarshal(params, &update); err != nil {
		return ""
	}
	return update.Content
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
	ProtocolVersion    int                  `json:"protocolVersion"`
	ClientCapabilities acpClientCapabilities `json:"clientCapabilities"`
	ClientInfo        acpClientInfo        `json:"clientInfo"`
}

type acpClientCapabilities struct {
	FS       acpFSCapabilities `json:"fs,omitempty"`
	Terminal bool             `json:"terminal,omitempty"`
	MCP      bool             `json:"mcp,omitempty"`
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
	SessionID string           `json:"sessionId"`
	Prompt    []acpContentBlock `json:"prompt"`
}

type acpContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type acpSessionParams struct {
	Cwd       string   `json:"cwd"`
	McpServers []string `json:"mcpServers"`
}

type acpCancelParams struct {
	SessionID string `json:"sessionId"`
}

type acpPromptResult struct {
	StopReason string `json:"stopReason"`
	Usage      any    `json:"usage,omitempty"`
}

type acpSessionResult struct {
	SessionID string `json:"sessionId"`
}

type acpSessionUpdate struct {
	Content    string `json:"content,omitempty"`
	StopReason string `json:"stopReason,omitempty"`
}

func mapACPError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ProviderError{Category: ErrorTimeout, Message: err.Error(), Err: err}
	case errors.Is(err, exec.ErrNotFound):
		return NewConfigurationError(fmt.Sprintf("%s ACP binary not found in PATH", "agent"), err)
	case strings.Contains(strings.ToLower(err.Error()), "executable file not found"):
		return NewConfigurationError(fmt.Sprintf("%s ACP binary not found in PATH", "agent"), err)
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "authentication"), strings.Contains(message, "auth"), strings.Contains(message, "token"):
		return ProviderError{Category: ErrorAuthentication, Message: err.Error(), Err: err}
	case strings.Contains(message, "rate limit"), strings.Contains(message, "429"):
		return ProviderError{Category: ErrorRateLimit, Message: err.Error(), Err: err}
	case strings.Contains(message, "timeout"), strings.Contains(message, "deadline exceeded"):
		return ProviderError{Category: ErrorTimeout, Message: err.Error(), Err: err}
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
