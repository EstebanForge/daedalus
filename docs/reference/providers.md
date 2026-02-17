# Daedalus Provider Contract v1

## Goal
Keep core runtime provider-agnostic so Codex, Claude, and Gemini integrate as modules without refactoring loop/business logic.

## Provider keys
- `codex` (v1 target)
- `claude` (planned)
- `gemini` (planned)

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

## Test requirements (all providers)
- Pass shared contract test suite.
- Pass event mapping golden tests.
- Pass one integration run in fixture repository.
- Support cancellation and graceful shutdown behavior.

