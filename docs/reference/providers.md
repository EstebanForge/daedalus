# Daedalus Provider Contract v1

## Goal
Keep core runtime provider-agnostic so Codex, Claude, Gemini, OpenCode, Copilot, Qwen Code, and Pi CLI integrate as modules without refactoring loop/business logic.

## Transport
All providers use the **Agent Client Protocol (ACP)** for communication. ACP provides a standardized JSON-RPC interface for agent execution, session management, and structured event streaming.

See `docs/ACP-migration.md` for detailed migration information.

## Provider keys
- `codex` — requires `codex-acp` adapter (see below)
- `claude` — requires Claude Agent SDK ACP adapter
- `gemini` — native ACP support
- `opencode` — native ACP support
- `copilot` — ACP support in public preview
- `qwen` — native ACP support
- `pi` — requires `pi-acp` adapter

## Required interface
- `Name() string`
- `Capabilities() ProviderCapabilities`
- `RunIteration(ctx, request) -> (<-chan, error)`

` Event, IterationResultRunIteration` semantics (v1):
- `error` is for pre-start failures only (invalid request/config, executable missing, launch failure).
- On pre-start failure, returned event channel must be `nil` and `IterationResult.success` must be `false`.
- In-flight execution failures are emitted as normalized `error` events.
- Core runtime treats the event stream (including terminal `error` and `iteration_finished`) as the source of truth for started runs.
- `IterationResult` may be partial for started runs and is primarily advisory.
- Provider must close the event channel exactly once for every successful start.
- On `ctx` cancellation, provider should stop quickly, emit a terminal `error` event with cancellation context when possible, then close the channel.
- Core consumers must drain events until channel close before evaluating final iteration success/failure.

## Session Management
ACP enables session persistence across iterations. Each provider maintains:
- **Process lifecycle** — agent subprocess management
- **Session state** — ACP session ID for the agent conversation
- **Message counter** — JSON-RPC ID tracking

Session behavior:
1. Provider starts agent process with `acp` subcommand
2. Initializes ACP connection via `initialize` method
3. Creates session via `session/new` method
4. Sends prompts via `session/prompt` method
5. Collects responses via `session/update` notifications

## Iteration request
- `workDir: string`
- `prompt: string`
- `contextFiles: []string`
- `approvalPolicy: string` — handled via ACP protocol when supported
- `sandboxPolicy: string` — handled via ACP protocol when supported
- `model: string`
- `metadata: map[string]string` (optional provider hints)

Rules:
- Core controls request shape.
- Provider modules may interpret `metadata`.
- Provider modules must not mutate caller-owned request state.

## Iteration result
- `success: bool`
- `summary: string`
- `usage` (optional token/cost structure)
- `providerRunID` (optional)

`usage` shape (optional):
- `inputTokens: int` (optional, `>= 0`)
- `outputTokens: int` (optional, `>= 0`)
- `totalTokens: int` (optional, `>= 0`)
- `currency: string` (optional, ISO 4217 when set, for example `USD`)
- `estimatedCost: string` (optional decimal string, for example `0.0025`)

Rules:
- Fields are additive and optional because providers may expose different usage fidelity.
- When `totalTokens` is omitted but both `inputTokens` and `outputTokens` are present, consumers may infer `totalTokens = inputTokens + outputTokens`.
- If a provider cannot supply usage safely, `usage` should be omitted rather than guessed.

## Normalized events
- `iteration_started`
- `assistant_text`
- `tool_started`
- `tool_finished`
- `command_output`
- `iteration_finished`
- `error`

Each event contains:
- `type`
- `message`
- `timestamp`
- `iteration`
- `storyID` (optional)
- `metadata: map[string]string` (optional provider detail)

Rules:
- Core logic may use `type`, `iteration`, `storyID`.
- Core logic must not branch on provider-specific metadata keys.

Recommended metadata keys (non-exhaustive):
- `provider` (for example `claude`)
- `model` (provider model alias/name)
- `run_id` (provider-native run/trace identifier)
- `tool` (tool name for tool lifecycle events)
- `command` (shell/tool command label)

## Capabilities model
- `streaming: bool`
- `toolCalls: bool`
- `sandboxControl: bool`
- `approvalModes: []string`
- `maxContextHint: int` (advisory only)

## Error model
Provider modules must map native errors into categories:
- `configuration_error`
- `authentication_error`
- `rate_limit_error`
- `timeout_error`
- `transient_error`
- `fatal_error`

Retry guidance:
- Retry only `rate_limit_error`, `timeout_error`, `transient_error`.
- Never retry `configuration_error` or `authentication_error` without user action.

## Registry contract
- Resolve provider by key.
- Return explicit errors for unknown/unconfigured provider.
- Registry owns provider construction and dependency injection.

## Compatibility policy
- Contract changes are versioned.
- New fields must be additive and optional.
- Breaking contract changes require major version bump and migration notes.

## Provider ACP Support

| Provider | Native ACP | Adapter Required | Notes |
|----------|-----------|------------------|-------|
| OpenCode | ✅ | No | Native ACP support |
| Gemini CLI | ✅ | No | Native ACP support |
| Qwen Code | ✅ | No | Native ACP support |
| Claude Agent | ❌ | Yes | Requires [Zed adapter](https://github.com/zed-industries/claude-agent-acp) |
| Codex CLI | ❌ | Yes | Requires [codex-acp](https://github.com/zed-industries/codex-acp) |
| Copilot | ✅ | No | ACP in public preview |
| Pi | ❌ | Yes | Requires [pi-acp](https://github.com/svkozak/pi-acp) |

**Note:** ACP support evolves rapidly. Check https://agentclientprotocol.com/get-started/agents.md for current list.

## Provider Configuration Notes

### OpenCode
- Binary: `opencode`
- Command: `opencode acp`
- Authentication: API key via environment or config
- Model: configurable via `--model` flag in session

### Gemini CLI
- Binary: `gemini`
- Command: `gemini acp` (or native CLI with ACP flag)
- Authentication: API key required (OAuth not supported)

### Qwen Code
- Binary: `qwen` or `qwen coder`
- Command: `qwen acp`
- Authentication: API key or OAuth

### Claude (with adapter)
- Binary: `claude` + ACP adapter
- Command: adapter spawns `claude` with ACP support
- **ToS Note:** Using Claude Code CLI is allowed; OAuth tokens from Free/Pro/Max plans are NOT permitted for Agent SDK use

### Codex (with adapter)
- Binary: `codex` + `codex-acp` adapter
- Command: adapter spawns `codex` with ACP support
- Uses OpenAI API under the hood (subscription-based)

### Copilot
- Binary: `copilot`
- Command: `copilot acp`
- Authentication: GitHub via `gh auth login`

### Pi (with adapter)
- Binary: `pi` + `pi-acp` adapter
- Command: adapter spawns `pi` with ACP support
- Supports multiple providers via `--provider` flag

## Test requirements (all providers)
- [x] Pass shared contract test suite (`internal/providers/contract_test.go` — 7 properties × 7 providers).
- [ ] Pass event mapping golden tests.
- [ ] Pass one integration run in fixture repository.
- [x] Support cancellation and graceful shutdown behavior.
