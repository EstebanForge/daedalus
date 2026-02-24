# Daedalus Provider Contract v1

## Goal
Keep core runtime provider-agnostic so Codex, Claude, Gemini, OpenCode, and Copilot CLI integrate as modules without refactoring loop/business logic.

## Provider keys
- `codex` (v1 target)
- `claude` (CLI-backed)
- `gemini` (CLI-backed)
- `opencode` (CLI-backed)
- `copilot` (CLI-backed)
- `qwen` (CLI-backed)
- `pi` (CLI-backed)

All providers use the same non-interactive CLI pattern (`-p` / `--print`) for execution.

## Required interface
- `Name() string`
- `Capabilities() ProviderCapabilities`
- `RunIteration(ctx, request) -> (<-chan Event, IterationResult, error)`

`RunIteration` semantics (v1):
- `error` is for pre-start failures only (invalid request/config, executable missing, launch failure).
- On pre-start failure, returned event channel must be `nil` and `IterationResult.success` must be `false`.
- In-flight execution failures are emitted as normalized `error` events.
- Core runtime treats the event stream (including terminal `error` and `iteration_finished`) as the source of truth for started runs.
- `IterationResult` may be partial for started runs and is primarily advisory.
- Provider must close the event channel exactly once for every successful start.
- On `ctx` cancellation, provider should stop quickly, emit a terminal `error` event with cancellation context when possible, then close the channel.
- Core consumers must drain events until channel close before evaluating final iteration success/failure.

## Iteration request
- `workDir: string`
- `prompt: string`
- `contextFiles: []string`
- `approvalPolicy: string`
- `sandboxPolicy: string`
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

## Provider notes

### Claude
- Integration uses the `claude` CLI (`-p/--print` mode) as the execution surface.
- Daedalus does not depend on Claude SDK usage for agent execution.
- Daedalus does not require Claude OAuth integration; Claude authentication is handled by the local CLI environment.
- **ToS Note:** OAuth tokens from Free/Pro/Max plans are NOT permitted for Agent SDK use. However, using `claude -p` (CLI mode) is allowed as it uses OAuth for Claude Code itself.

### Gemini
- Integration uses the `gemini` CLI (`-p/--print` mode) as the execution surface.
- Requires API key authentication (OAuth via Antigravity is NOT supported).
- **ToS Warning:** Using OAuth tokens from Google AI Ultra/Pro with third-party tools violates Google's ToS and may result in account bans. Always use API keys from Google Cloud Console.

### Codex
- Integration uses `codex "prompt"` or `codex exec` for non-interactive execution.
- Uses OpenAI API under the hood (subscription-based).
- No additional authentication notes.

### OpenCode
- Integration uses `opencode -p "prompt"` or `opencode "prompt"` for non-interactive execution.
- Supports custom model configuration via `--model` flag.
- Uses API key authentication.

### Copilot
- Integration uses `copilot -p "prompt"` or `copilot --prompt "prompt"` for non-interactive execution.
- Requires GitHub authentication (via `gh auth login`).
- No additional authentication notes.

### Qwen Code
- Integration uses `qwen -p "prompt"` for non-interactive execution.
- Forked from Gemini CLI, optimized for Qwen3-Coder models.
- Supports multiple authentication methods:
  - **Qwen OAuth** (free tier, 1,000 requests/day)
  - **API keys** (required for headless/CI environments)
- Supports multiple providers: OpenAI, Anthropic, Google Gemini, Alibaba Cloud.
- **Note:** OAuth flow requires browser — use API key for CI/automation.

### Pi
- Integration uses `pi -p "prompt"` for non-interactive execution.
- Supports multiple providers via `--provider` flag (e.g., `openai`, `anthropic`).
- Also supports model prefixes: `pi --model openai/gpt-4o "prompt"`.
- Has RPC mode for stdin/stdout integration.
- Requires authentication based on selected provider.

## Test requirements (all providers)
- [x] Pass shared contract test suite (`internal/providers/contract_test.go` — 7 properties × 7 providers).
- [ ] Pass event mapping golden tests.
- [ ] Pass one integration run in fixture repository.
- [x] Support cancellation and graceful shutdown behavior.
