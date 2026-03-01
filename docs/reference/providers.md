# Daedalus Provider Contract v1

## Goal

Keep core runtime provider-agnostic so Codex, Claude, Gemini, OpenCode, Copilot, Qwen Code, and Pi integrate without loop/business-logic refactors.

## Transport

All providers run through Agent Client Protocol (ACP) transport.

## Provider keys

- `codex`
- `claude`
- `gemini`
- `opencode`
- `copilot`
- `qwen`
- `pi`

## Required interface

- `Name() string`
- `Capabilities() Capabilities`
- `RunIteration(ctx, request) -> (<-chan Event, IterationResult, error)`

`RunIteration` semantics:
- Returned `error` is pre-start only (invalid request/config, process launch failure).
- On pre-start failure: event channel must be `nil` and `IterationResult.Success=false`.
- In-flight failures must be emitted as `error` events.
- Provider must close event channel exactly once for every successful start.
- On context cancellation, provider should cancel the ACP session quickly and close the channel.

## Iteration request

- `workDir: string`
- `prompt: string`
- `contextFiles: []string`
- `approvalPolicy: string`
- `sandboxPolicy: string`
- `model: string`
- `metadata: map[string]string` (optional)

Rules:
- Core owns request shape.
- Provider modules may interpret metadata but must not mutate caller-owned request state.

## Iteration result

- `success: bool`
- `summary: string`
- `providerRunID: string` (optional)

## Normalized provider events

- `iteration_started`
- `assistant_text`
- `tool_started`
- `tool_finished`
- `command_output`
- `iteration_finished`
- `error`

In-memory provider event payload:
- `type`
- `message`

Persistence layer (`events.jsonl`) enriches events with runtime fields (`timestamp`, `iteration`, `storyID`, optional metadata).

## Capabilities model

- `streaming: bool`
- `toolCalls: bool`
- `sandboxControl: bool`
- `approvalModes: []string`
- `modelSelection: bool`
- `supportedModels: []string`
- `maxContextHint: int` (advisory)

Capabilities are negotiated from ACP `initialize` results when the provider exposes
server capability metadata. Daedalus uses negotiated capabilities to validate
requested approval policy, sandbox policy, and model selection before dispatching
`session/prompt`.

## Error model

Provider errors must map into categories:

- `configuration_error`
- `authentication_error`
- `rate_limit_error`
- `timeout_error`
- `transient_error`
- `fatal_error`

Retry guidance:
- Retry only: `rate_limit_error`, `timeout_error`, `transient_error`.
- Do not retry: `configuration_error`, `authentication_error`.

## Registry contract

- Resolve provider by key.
- Return explicit error for unknown provider key.
- Return explicit error when provider key is disabled by config.

## Provider ACP support

| Provider | Native ACP | Adapter Required | Notes |
|----------|-----------|------------------|-------|
| OpenCode | Yes | No | Native ACP |
| Gemini CLI | Yes | No | Native ACP |
| Qwen Code | Yes | No | Native ACP |
| Copilot | Yes (preview) | No | Preview support |
| Claude Agent | No | Yes | Zed adapter |
| Codex CLI | No | Yes | `codex-acp` |
| Pi | No | Yes | `pi-acp` |

Note: ACP support evolves; validate against https://agentclientprotocol.com/get-started/agents.md when adding/changing providers.

## Provider command resolution

Default ACP command behavior:
- Native providers run `<provider> acp`.
- Adapter-backed providers run adapter binaries by default.

Config may override ACP command per provider via `[providers.<key>].acp_command`.

## Test requirements

- [x] ACP provider contract tests for active runtime path.
- [x] Event mapping golden tests.
- [x] Env-gated real-provider integration test scaffolding.
- [ ] One fixture integration run per provider family (native + adapter-backed).
- [x] Cancellation/shutdown behavior tests under retry conditions.
