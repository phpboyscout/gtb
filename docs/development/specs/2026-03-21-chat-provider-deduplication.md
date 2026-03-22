---
title: "Chat Provider Deduplication Specification"
description: "Extract duplicated tool execution and ReAct loop logic from Claude, OpenAI, and Gemini providers into shared helpers."
date: 2026-03-21
status: IMPLEMENTED
tags:
  - specification
  - chat
  - refactor
  - deduplication
author:
  - name: Matt Cockayne
    email: matt@phpboyscout.com
  - name: Claude (claude-opus-4-6)
    role: AI drafting assistant
---

# Chat Provider Deduplication Specification

Authors
:   Matt Cockayne, Claude (claude-opus-4-6) *(AI drafting assistant)*

Date
:   21 March 2026

Status
:   IMPLEMENTED

---

## Overview

The three AI providers (Claude, OpenAI, Gemini) each implement their own `executeTool` function and ReAct loop structure in `Chat()`. The tool execution logic is nearly identical across all three:

1. Look up tool by name from the registered tools map
2. Marshal input arguments to JSON
3. Call `tool.Function(ctx, input)`
4. Return result string or error

The ReAct loop structure is also similar: iterate up to `maxSteps`, call the provider API, check for tool calls in the response, execute tools, append results, and repeat until no tool calls remain or the step limit is reached.

This duplication means bug fixes (e.g., error handling improvements) must be applied three times, and new features (e.g., tool call logging, timeout enforcement) require three implementations.

---

## Design Decisions

**Shared `executeTool` helper**: Extract a single `executeTool` function into `pkg/chat/tools.go` that all providers call. This is the lowest-risk deduplication since the function signature and behaviour are already identical.

**Provider-specific ReAct loops**: The ReAct loop structure is similar but not identical — each provider has different request/response types and tool call formats. Rather than forcing a complex abstraction, keep the loop in each provider but extract the shared tool dispatch logic. This preserves readability while eliminating the most impactful duplication.

**Tool registry as map**: All providers already store tools as `map[string]Tool`. The shared helper accepts this map directly.

---

## Public API Changes

None. This is a purely internal refactoring.

---

## Internal Implementation

### New File: `pkg/chat/tools.go`

```go
package chat

import (
    "context"
    "encoding/json"

    "github.com/cockroachdb/errors"
)

// executeTool looks up and executes a tool by name from the provided registry.
// Returns the tool's string result or an error if the tool is not found or execution fails.
func executeTool(ctx context.Context, tools map[string]Tool, name string, input json.RawMessage) (string, error) {
    tool, ok := tools[name]
    if !ok {
        return "", errors.Newf("tool %q not found", name)
    }

    result, err := tool.Function(ctx, input)
    if err != nil {
        return "", errors.Wrapf(err, "executing tool %q", name)
    }

    return result, nil
}

// toolResultOrError executes a tool and returns the result string.
// If an error occurs, it returns the error message as the result string
// (for feeding back into the AI conversation) and nil error.
// This matches the existing provider behaviour where tool errors become
// conversation content rather than aborting the ReAct loop.
func toolResultOrError(ctx context.Context, tools map[string]Tool, name string, input json.RawMessage) string {
    result, err := executeTool(ctx, tools, name, input)
    if err != nil {
        return err.Error()
    }
    return result
}
```

### Updated Claude Provider

```go
// Before (in claude.go):
func (c *Claude) executeTool(ctx context.Context, name string, input json.RawMessage) string {
    tool, ok := c.tools[name]
    if !ok {
        return fmt.Sprintf("tool %s not found", name)
    }
    result, err := tool.Function(ctx, input)
    if err != nil {
        return fmt.Sprintf("error: %v", err)
    }
    return result
}

// After:
// Remove the method entirely. In Chat(), replace:
//   result := c.executeTool(ctx, tc.Name, inputJSON)
// With:
//   result := toolResultOrError(ctx, c.tools, tc.Name, inputJSON)
```

### Updated OpenAI Provider

```go
// Before (in openai.go):
func (a *OpenAI) executeTool(ctx context.Context, name string, args string) string {
    tool, ok := a.tools[name]
    if !ok {
        return fmt.Sprintf("tool %s not found", name)
    }
    result, err := tool.Function(ctx, json.RawMessage(args))
    if err != nil {
        return fmt.Sprintf("error: %v", err)
    }
    return result
}

// After:
// Remove the method. In Chat(), replace:
//   result := a.executeTool(ctx, tc.Function.Name, tc.Function.Arguments)
// With:
//   result := toolResultOrError(ctx, a.tools, tc.Function.Name, json.RawMessage(tc.Function.Arguments))
```

### Updated Gemini Provider

```go
// Before (in gemini.go):
func (g *Gemini) executeTool(ctx context.Context, name string, args map[string]any) string {
    tool, ok := g.tools[name]
    if !ok {
        return fmt.Sprintf("tool %s not found", name)
    }
    inputJSON, _ := json.Marshal(args)
    result, err := tool.Function(ctx, inputJSON)
    if err != nil {
        return fmt.Sprintf("error: %v", err)
    }
    return result
}

// After:
// Remove the method. In Chat(), replace with:
//   inputJSON, err := json.Marshal(args)
//   if err != nil { ... }
//   result := toolResultOrError(ctx, g.tools, name, inputJSON)
```

Note: Gemini's `executeTool` takes `map[string]any` args which need marshalling. The JSON marshalling step stays at the call site since it's Gemini-specific.

---

## Project Structure

```
pkg/chat/
├── tools.go           ← NEW: shared executeTool, toolResultOrError
├── tools_test.go      ← NEW: tests for shared helpers
├── claude.go          ← MODIFIED: remove executeTool method, use shared helper
├── openai.go          ← MODIFIED: remove executeTool method, use shared helper
├── gemini.go          ← MODIFIED: remove executeTool method, use shared helper
```

---

## Testing Strategy

| Test | Scenario |
|------|----------|
| `TestExecuteTool_Found` | Tool exists → result returned |
| `TestExecuteTool_NotFound` | Tool missing → error with tool name |
| `TestExecuteTool_FunctionError` | Tool.Function returns error → wrapped error |
| `TestToolResultOrError_Success` | Tool succeeds → result string |
| `TestToolResultOrError_NotFound` | Tool missing → error message as string |
| `TestToolResultOrError_FunctionError` | Tool fails → error message as string |
| Existing provider tests | All existing `Chat()` tests pass unchanged |

### Test Setup

```go
func TestExecuteTool_Found(t *testing.T) {
    tools := map[string]Tool{
        "echo": {
            Name: "echo",
            Function: func(ctx context.Context, input json.RawMessage) (string, error) {
                return string(input), nil
            },
        },
    }

    result, err := executeTool(context.Background(), tools, "echo", json.RawMessage(`"hello"`))
    assert.NoError(t, err)
    assert.Equal(t, `"hello"`, result)
}
```

### Coverage
- Target: 100% for `pkg/chat/tools.go` (small, critical helper).
- Target: 90%+ for `pkg/chat/` overall.

---

## Linting

- `golangci-lint run --fix` must pass.
- No new `nolint` directives.
- Removing duplicated methods reduces cyclomatic complexity in provider files.

---

## Documentation

- Godoc for `executeTool` and `toolResultOrError` explaining their roles.
- No user-facing documentation changes.

---

## Backwards Compatibility

- No breaking changes. This is an internal refactoring.
- All provider behaviour is preserved — tool errors still become conversation content.

---

## Future Considerations

- **Tool call logging**: With a single execution point, adding structured logging for tool calls becomes trivial.
- **Tool call timeout**: A per-tool timeout could be enforced in the shared helper.
- **ReAct loop extraction**: If providers converge further (e.g., after a unified request/response abstraction), the loop itself could be extracted. This is premature now.

---

## Implementation Phases

### Phase 1 — Extract Shared Helper
1. Create `pkg/chat/tools.go` with `executeTool` and `toolResultOrError`
2. Add comprehensive tests in `tools_test.go`

### Phase 2 — Migrate Providers
1. Update Claude to use shared helper
2. Update OpenAI to use shared helper
3. Update Gemini to use shared helper (with JSON marshalling at call site)
4. Remove provider-specific `executeTool` methods

### Phase 3 — Verify
1. Run full test suite
2. Verify no behaviour changes via existing integration tests

---

## Verification

```bash
go build ./...
go test -race ./pkg/chat/...
go test ./...
golangci-lint run --fix

# Verify no provider-specific executeTool methods remain
grep -n 'func.*executeTool' pkg/chat/claude.go pkg/chat/openai.go pkg/chat/gemini.go
# Should return no results

# Verify shared helper exists
grep -n 'func executeTool' pkg/chat/tools.go
# Should return one result
```
