# ACP Migration Plan

> **Status:** Complete
> **Created:** 2026-02-26
> **Last Updated:** 2026-03-02
> **Target:** Daedalus v2.0

## Summary

Migrate all agent providers from provider-specific CLI argument invocation to Agent Client Protocol (ACP) JSON-RPC transport.

## Goals

1. One provider runtime path for all supported providers.
2. Structured ACP event streaming instead of brittle text parsing.
3. Provider-agnostic loop behavior with typed, retry-aware errors.
4. Zero-config session continuity with best-effort resume across iterations and process restarts.

## Non-Goals

- Re-introducing CLI fallback transport.
- Maintaining provider-specific core logic branches.
- Supporting non-ACP-only providers.

## Current State

Done:
- Registry routes all provider keys to ACP provider construction.
- ACP JSON-RPC types and initialize/session lifecycle scaffolding exist.
- Core loop and onboarding scan paths are already wired through provider abstraction.
- Capability negotiation is parsed from ACP initialize responses and enforced for policy/model compatibility.
- Runtime diagnostics commands are available for ACP health and session observability.

In progress:
- Real-provider ACP integration execution and provider-specific validation.
- Migration closeout after provider-family fixture integrations.

Pending:
- Fixture integration runs for provider families (native + adapter-backed).

## Architecture

Provider contract remains:

```text
RunIteration(ctx, request) -> (<-chan Event, IterationResult, error)
Capabilities() -> Capabilities
Name() -> string
```

ACP transport responsibilities:

```text
ACP Provider
|- Process/session manager
|  |- subprocess lifecycle
|  |- ACP session id tracking
|  |- JSON-RPC message id tracking
|- JSON-RPC handler
|  |- initialize
|  |- session/resume (best-effort)
|  |- session/new (fallback)
|  |- session/prompt
|  |- session/cancel
'- Event normalization
   |- session/update -> EventAssistantText / EventTool*
   '- error -> EventError
```

## Session Lifecycle

```text
1. Start agent ACP process (<binary> acp or adapter command)
2. Send initialize
3. Attempt session/resume from persisted cache (provider + workdir + command)
4. Fallback to session/new when resume is unavailable or rejected
5. Send session/prompt
6. Consume streaming notifications and final response
7. Cancel/end on completion or context cancellation
```

## Implementation Plan

### Phase 1: ACP Provider Foundation

- [x] Create `internal/providers/acp.go`
- [x] Implement ACP JSON-RPC message model
- [x] Wire registry to ACP provider for all provider keys

### Phase 2: Transport Runtime Hardening

- [x] Implement persistent stdin/stdout I/O for JSON-RPC
- [x] Handle `session/update` notifications for streaming text
- [x] Normalize tool lifecycle events when provided by ACP updates
- [x] Implement reliable cancellation (`session/cancel` + process handling)

### Phase 3: Session Reuse

- [x] Define in-process session reuse strategy (provider + workdir scope)
- [x] Persist/recover session identity with zero-config cache (`.daedalus/acp-sessions.json`)
- [x] Handle stale/expired session recovery

### Phase 4: Testing

- [x] ACP provider contract tests (active runtime path)
- [x] Event mapping golden tests
- [x] Integration runs for native ACP providers (OpenCode, Gemini, Qwen)
- [x] Adapter-backed integration runs (Codex, Claude, Pi)
- [x] Add env-gated integration test scaffolding for real providers
- [x] Error mapping and cancellation tests

### Phase 5: Cleanup and Closeout

- [x] Remove deprecated CLI provider implementations
- [x] Confirm docs/code parity (`ARCHITECTURE`, provider/config refs, AGENTS)
- [x] Final migration validation: `golangci-lint`, `go vet`, `go test`

## Provider ACP Support Matrix

| Provider | Native ACP | Adapter Required |
|----------|-----------|------------------|
| OpenCode | Yes | No |
| Gemini CLI | Yes | No |
| Qwen Code | Yes | No |
| Copilot | Yes (preview) | No |
| Claude Agent | No | Yes (Zed adapter) |
| Codex CLI | No | Yes (`codex-acp`) |
| Pi | No | Yes (`pi-acp`) |

## Risks

1. Adapter lifecycle and compatibility drift.
2. ACP protocol/server behavior differences across providers.
3. Session/process leaks under retry/cancellation pressure.
4. Insufficient coverage of the active ACP runtime path.

## Open Questions

1. MCP server exposure policy for ACP sessions.
2. Required tool event normalization contract in TUI/logs.
3. Provider-specific ACP command override shape in config.

## References

- ACP Specification: https://agentclientprotocol.com/
- ACP Agents List: https://agentclientprotocol.com/get-started/agents.md
- OpenCode ACP: https://github.com/sst/opencode
- Codex ACP Adapter: https://github.com/zed-industries/codex-acp
