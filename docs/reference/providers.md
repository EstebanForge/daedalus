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
- Pass shared contract test suite.
- Pass event mapping golden tests.
- Pass one integration run in fixture repository.
- Support cancellation and graceful shutdown behavior.
