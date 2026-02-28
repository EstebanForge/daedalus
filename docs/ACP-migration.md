# ACP Migration Plan

> **Status:** Planning  
> **Created:** 2026-02-26  
> **Target:** Daedalus v2.0

## Summary

Migrate all agent providers from CLI-based invocation to the Agent Client Protocol (ACP). This removes fragmented provider-specific CLI interfaces and replaces them with a standardized JSON-RPC interface.

## Problem Statement

### Current State

Each provider uses a different CLI invocation pattern:

| Provider | Invocation | Interface |
|----------|-----------|-----------|
| Codex | `codex exec --sandbox <policy> --skip-git-repo-check --output-last-message <path> <prompt>` | CLI args + temp file |
| OpenCode | `opencode -p <prompt>` | CLI args |
| Claude | `claude -p <prompt>` | CLI args |
| Gemini CLI | `gemini -p <prompt>` | CLI args |
| Qwen | `qwen coder -p <prompt>` | CLI args |
| Copilot | `copilot --prompt "<prompt>"` | CLI args |
| Pi | `pi agent -p <prompt>` | CLI args |

Issues with current approach:
1. **Fragmented interfaces** — each provider has different flags, argument order, output formats
2. **No session state** — each invocation starts fresh; no conversation continuity
3. **Fragile parsing** — stdout/stderr line-by-line parsing is brittle
4. **Hard to maintain** — provider CLI changes require updates to each adapter
5. **No standardized error handling** — each provider maps errors differently

### Why ACP?

The Agent Client Protocol (ACP) provides:
- **Standardized JSON-RPC interface** — same messages for all agents
- **Session management** — resume conversations, maintain context across iterations
- **Structured events** — no fragile stdout parsing
- **Rich prompts** — structured content blocks, not just CLI args
- **Future-proof** — 25+ agents support ACP (OpenCode, Claude via adapter, Gemini CLI, Qwen, Copilot, etc.)

## Goals

1. **Unified interface** — one provider implementation handles all ACP-compatible agents
2. **Session persistence** — maintain agent conversation across story iterations
3. **Structured communication** — JSON-RPC instead of CLI argument parsing
4. **Provider-agnostic core** — ACP is the abstraction; agent-specific details stay in config

## Non-Goals

- Support non-ACP agents (CLI fallback)
- Backward compatibility with CLI transport
- Supporting providers that don't have ACP implementations

## Architecture

### Provider Contract (Unchanged)

```
RunIteration(ctx, request) -> (<-chan Event, IterationResult, error)
Capabilities() -> Capabilities
Name() -> string
```

### New Transport Layer

```
ACP Provider
├── Session Manager (package-level)
│   ├── process lifecycle
│   ├── session state (agent session ID)
│   └── message counter
├── JSON-RPC Handler
│   ├── initialize
│   ├── session/new
│   ├── session/prompt
│   └── session/cancel
└── Event Normalizer
    ├── session/update -> EventAssistantText
    ├── error -> EventError
    └── tool_* -> EventTool*
```

### Session Lifecycle

```
1. Start agent process: <agent> acp
2. Initialize: {method: "initialize", params: {protocolVersion, clientCapabilities, clientInfo}}
3. Create session: {method: "session/new", params: {cwd, mcpServers}}
4. Send prompt: {method: "session/prompt", params: {sessionId, prompt: [{type: "text", text}]}}
5. Collect responses via session/update notifications
6. On completion: session ends, process terminates
```

### Configuration

```toml
[providers.codex]
enabled = true
model = "default"
# ACP-specific (future)
# transport = "acp"  # implicit, CLI no longer supported

[providers.opencode]
enabled = true
model = "default"
```

## Implementation Plan

### Phase 1: ACP Provider Implementation (DONE)

- [x] Create `internal/providers/acp.go`
- [x] Implement session management (package-level state)
- [x] Implement JSON-RPC message types
- [x] Implement initialize/session_new/session_prompt flow
- [x] Update registry to route all providers through ACP
- [x] Build and vet passes

### Phase 2: Polling Implementation

- [ ] Implement stdout polling for JSON-RPC responses
- [ ] Handle session/update notifications (streaming)
- [ ] Handle tool call events
- [ ] Implement proper cancellation

### Phase 3: Session Reuse (Future)

- [ ] Persist session ID across iterations
- [ ] Resume session instead of creating new one
- [ ] Handle session expiration/timeout

### Phase 4: Testing

- [ ] Test with OpenCode (native ACP support)
- [ ] Test with Codex (requires `codex-acp` adapter)
- [ ] Test with Gemini CLI
- [ ] Integration tests per provider
- [ ] Error handling tests

### Phase 5: Cleanup

- [ ] Remove old CLI-based provider files (codex.go, opencode.go, etc.)
- [ ] Update ARCHITECTURE.md
- [ ] Update provider documentation
- [ ] Update AGENTS.md

## Provider ACP Support Matrix

| Provider | Native ACP | Adapter Required |
|----------|-----------|------------------|
| OpenCode | ✅ | No |
| Gemini CLI | ✅ | No |
| Claude Agent | ❌ | Yes (Zed adapter) |
| Codex CLI | ❌ | Yes (codex-acp) |
| Copilot | ❌ | Unknown |
| Qwen Code | ✅ | No |
| Pi | ❌ | Yes (pi-acp) |

**Note:** Provider support is evolving. Check https://agentclientprotocol.com/get-started/agents.md for current list.

## Risks

1. **Adapter dependency** — Some agents require adapters (Codex, Claude) which adds maintenance surface
2. **Protocol changes** — ACP is evolving; must track version changes
3. **Process lifecycle** — Session reuse requires careful error handling
4. **Testing complexity** — Each provider + adapter combination needs verification

## Open Questions

1. **Session reuse strategy?** — Keep session alive between stories or create fresh each time?
2. **MC servers?** — Should Daedalus expose MCP servers to agents via ACP?
3. **Tool call handling?** — How to surface tool calls in TUI/events?
4. **Provider config migration?** — How to handle provider-specific ACP vs CLI-only config?

## References

- ACP Specification: https://agentclientprotocol.com/
- ACP Agents List: https://agentclientprotocol.com/get-started/agents.md
- ACP Libraries: https://agentclientprotocol.com/libraries/
- OpenCode ACP: https://github.com/sst/opencode
- Codex ACP Adapter: https://github.com/zed-industries/codex-acp
