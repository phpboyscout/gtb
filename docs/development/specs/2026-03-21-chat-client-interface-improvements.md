---
title: "ChatClient Interface Improvements Specification"
description: "Add context.Context to Add and Ask methods, document thread safety contract, and specify the full ChatClient interface behavioural contract."
date: 2026-03-21
status: DRAFT
tags:
  - specification
  - chat
  - interfaces
  - breaking-change
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# ChatClient Interface Improvements Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   21 March 2026

Status
:   DRAFT

---

## Overview

The `ChatClient` interface has three deficiencies:

1. **Missing `context.Context`**: `Add()` and `Ask()` lack context parameters while `Chat()` has one. This prevents callers from controlling cancellation or deadlines for individual calls. All four provider implementations store `context.Context` as a struct field, which the Go documentation explicitly warns against.

2. **Undocumented thread safety**: All providers mutate message history slices without synchronisation. Whether this is intentional or an oversight is unclear to consumers.

3. **Underspecified contract**: The interface lacks documentation about behaviour when `Ask()` is called without a `ResponseSchema`, whether `Add()` messages persist across `Chat()` calls, and error/retry semantics.

---

## Design Decisions

**Context on every method**: All methods that perform I/O or could block accept `context.Context` as the first parameter. This follows standard Go library conventions.

**Not goroutine-safe (documented)**: Rather than adding mutex overhead, we document that `ChatClient` implementations are not safe for concurrent use — consistent with `http.Request`, `json.Decoder`, and most Go types. Each goroutine should create its own client instance.

**No retry logic**: The interface does not specify retry behaviour. Rate limit errors and transient failures surface directly to the caller. Retry logic belongs in a higher-level wrapper if needed.

---

## Public API Changes

### Modified: `ChatClient` Interface

```go
// ChatClient defines the interface for interacting with a chat service.
//
// Implementations are NOT safe for concurrent use by multiple goroutines.
// Each goroutine should use its own ChatClient instance.
//
// Message history from Add() calls persists across Chat() and Ask() calls
// within the same client instance. To start a fresh conversation, create
// a new client via chat.New().
type ChatClient interface {
    // Add appends a user message to the conversation history without
    // triggering a completion. The message persists for subsequent
    // Chat() or Ask() calls.
    Add(ctx context.Context, prompt string) error

    // Ask sends a question and unmarshals the structured response into
    // target. If Config.ResponseSchema was set during construction, the
    // provider enforces that schema. If no schema is set, the provider
    // returns the raw text content unmarshalled into target (which must
    // be a *string or implement json.Unmarshaler).
    Ask(ctx context.Context, question string, target any) error

    // SetTools configures the tools available to the AI. This replaces
    // (not appends to) any previously set tools.
    SetTools(tools []Tool) error

    // Chat sends a message and returns the response content. If tools
    // are configured, the provider handles tool calls internally via a
    // ReAct loop bounded by Config.MaxSteps (default 20).
    Chat(ctx context.Context, prompt string) (string, error)
}
```

### Removed: Stored Context from Provider Structs

Each provider struct loses its `ctx context.Context` field:

```go
// Before:
type Claude struct {
    ctx      context.Context  // REMOVED
    client   anthropic.Client
    // ...
}

// After:
type Claude struct {
    client   anthropic.Client
    // ...
}
```

---

## Internal Implementation

### Provider Changes (All Four)

For each provider (Claude, OpenAI, Gemini, ClaudeLocal):

1. Remove `ctx context.Context` from struct fields
2. Update `Add(ctx context.Context, prompt string) error`
3. Update `Ask(ctx context.Context, question string, target any) error`
4. Factory functions (`newClaude`, `newOpenAI`, etc.) no longer store `ctx` — the context passed to `New()` is only used for client initialisation (e.g., Gemini's `genai.NewClient`)

#### Claude Example

```go
func (c *Claude) Add(ctx context.Context, prompt string) error {
    c.messages = append(c.messages, anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)))
    return nil
}

func (c *Claude) Ask(ctx context.Context, question string, target any) error {
    c.messages = append(c.messages, anthropic.NewUserMessage(anthropic.NewTextBlock(question)))
    // ...
    resp, err := c.client.Messages.New(ctx, params)  // ctx passed through
    // ...
}
```

#### OpenAI Example

```go
func (a *OpenAI) Ask(ctx context.Context, question string, target any) error {
    // ...
    res, err := a.oai.Chat.Completions.New(ctx, a.params)  // ctx passed through
    // ...
}
```

#### Gemini — Special Case

Gemini's `genai.NewClient` requires a context at construction time. This context is used for the HTTP client setup, not for individual requests. The factory stores the client (which embeds its own transport context), and per-request contexts are passed through:

```go
func newGemini(ctx context.Context, p *props.Props, cfg Config) (ChatClient, error) {
    client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: token})
    // ctx is NOT stored — only used for client init
    return &Gemini{client: client, ...}, nil
}

func (g *Gemini) Ask(ctx context.Context, question string, target any) error {
    chat, err := g.client.Chats.Create(ctx, g.model, askCfg, g.history)
    // ...
}
```

### Caller Updates

All callers of `Add()` and `Ask()` must pass a context. Key callsites:

| File | Method | Change |
|------|--------|--------|
| `internal/generator/docs.go` | `generatePackageDocs` | Pass `ctx` from generator method |
| `internal/generator/commands.go` | `generateWithAI` | Pass `ctx` from generator method |
| `pkg/docs/ask.go` | `AskQuestion` | Pass `ctx` from command context |

### Mock Regeneration

Regenerate mocks via `mockery`:
```bash
mockery
```

The `ChatClient` mock will automatically gain the new method signatures.

---

## Project Structure

```
pkg/chat/
├── client.go          ← MODIFIED: interface + godoc
├── claude.go          ← MODIFIED: remove ctx field, update Add/Ask
├── openai.go          ← MODIFIED: remove ctx field, update Add/Ask
├── gemini.go          ← MODIFIED: remove ctx field, update Add/Ask
├── claude_local.go    ← MODIFIED: remove ctx field, update Add/Ask
├── client_test.go     ← MODIFIED: update test signatures
internal/generator/
├── docs.go            ← MODIFIED: pass ctx to Add/Ask
├── commands.go        ← MODIFIED: pass ctx to Add/Ask
pkg/docs/
├── ask.go             ← MODIFIED: pass ctx to Add/Ask
mocks/
├── (regenerated)
```

---

## Testing Strategy

| Test | Scenario |
|------|----------|
| `TestChatClient_Add_WithContext` | Context cancellation before Add → returns context error or succeeds (Add is local) |
| `TestChatClient_Ask_ContextCancelled` | Cancelled context → API call fails with context error |
| `TestChatClient_Ask_WithDeadline` | Deadline exceeded → appropriate error returned |
| `TestChatClient_MessagePersistence` | Add → Chat → messages from Add present in conversation |
| `TestChatClient_SetTools_Replaces` | SetTools twice → only second set active |
| Existing provider tests | Updated signatures, same assertions |

### Coverage
- Target: 90%+ for `pkg/chat/`.

---

## Linting

- `golangci-lint run --fix` must pass.
- No new `nolint` directives.
- The `contextcheck` linter will now pass for `Add` and `Ask` (previously they used stored contexts).

---

## Documentation

- Comprehensive godoc on `ChatClient` interface (see Public API Changes).
- Godoc on each method specifying behaviour, error conditions, and context usage.
- Update `docs/components/chat.md` with:
  - Thread safety guidance
  - Context usage examples
  - Message persistence explanation

---

## Backwards Compatibility

- **Breaking change**: `Add()` and `Ask()` signatures change. All callers must be updated.
- **Mock regeneration required**: Mocks must be regenerated.
- **Provider factory context**: The `ctx` parameter to `New()` / factory functions is still required for provider initialisation but is no longer stored.

---

## Future Considerations

- **Context-aware Add**: Currently `Add()` is a local append and ignores the context. If a provider needs to validate prompts server-side, the context is already available.
- **Streaming**: When streaming support is added (separate spec), the `Stream()` method will naturally accept `context.Context`.

---

## Implementation Phases

### Phase 1 — Interface Change
1. Update `ChatClient` interface in `client.go`
2. Add comprehensive godoc

### Phase 2 — Provider Updates
1. Update Claude: remove `ctx` field, update `Add`/`Ask`
2. Update OpenAI: same
3. Update Gemini: same
4. Update ClaudeLocal: same

### Phase 3 — Caller Updates
1. Update `internal/generator/docs.go`
2. Update `internal/generator/commands.go`
3. Update `pkg/docs/ask.go`
4. Regenerate mocks

### Phase 4 — Tests
1. Update existing tests for new signatures
2. Add contract tests for documented behaviour
3. Run full suite with race detector

---

## Verification

```bash
go build ./...
go test -race ./pkg/chat/... ./internal/generator/... ./pkg/docs/...
go test ./...
golangci-lint run --fix
mockery  # regenerate mocks

# Verify no stored context in provider structs
grep -n 'ctx.*context\.Context' pkg/chat/claude.go pkg/chat/openai.go pkg/chat/gemini.go pkg/chat/claude_local.go
# Should only appear in method parameters, not struct fields
```
