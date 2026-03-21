// Package chat provides a unified multi-provider AI chat client supporting
// Claude, OpenAI, Gemini, Claude Local (via CLI binary), and OpenAI-compatible
// endpoints.
//
// The [ChatClient] interface exposes four core methods: Add (append a message),
// Chat (multi-turn conversation with tool use via a ReAct loop), Ask (structured
// output with JSON schema validation), and SetTools (register callable tools).
//
// Tool calling follows a JSON Schema parameter definition, and the ReAct loop
// automatically dispatches tool calls and feeds results back until the model
// produces a final text response. Per-provider token limits and maximum agent
// steps are configurable via [Config].
//
// New providers can be registered at runtime via [RegisterProvider]. Structured
// output helpers such as GenerateSchema simplify schema generation for Ask calls.
package chat
