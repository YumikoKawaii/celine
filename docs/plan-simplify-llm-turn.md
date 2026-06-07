# Plan: Simplify LLM turn handling

## Goal

Replace the streaming/pacing machinery with a simple call-and-send model.
Claude generates the full response, we split it into bubbles by `\n\n`, send
them immediately. No fake delays, no goroutine/channel, no `Typing` event.

## Motivation

The current `stream.go` pacing engine simulates "typing" by:
1. Sleeping on the backend for `typingDelayMs` before each bubble
2. Sleeping 500ms between bubbles

This is artificial. Humans type complete thoughts and send them; readers read
the whole message at once. The fake delays add latency without improving the
experience.

---

## Phase 1 — Simplify `llm`

**File:** `basis/internal/llm/claude.go`

- Remove `deltas chan<- string` parameter from `StreamChat`
- Switch from `Messages.NewStreaming` to `Messages.New` (non-streaming) — the
  token-by-token feed is no longer needed
- Rename `StreamChat` → `Chat`
- New signature:

```go
func (c *Client) Chat(
    ctx context.Context,
    system string,
    history []Message,
    tools []ToolDef,
) (Turn, error)
```

**Impact:** `brain` interface in `agent/interfaces.go` updates to match:

```go
type brain interface {
    Chat(ctx context.Context, system string, history []llm.Message, tools []llm.ToolDef) (llm.Turn, error)
}
```

---

## Phase 2 — Gut `agent`

**Delete:** `basis/internal/agent/stream.go` entirely.
  Removes: `paceBubbles`, `paceBubblesWith`, `firstBoundary`, `typingDelayMs`,
  `sleep`, all timing constants.

**Delete:** `stream_test.go` (tests for the deleted file).

**Simplify `agent/interfaces.go`:**
- Remove `Typing` from `EventSink`

```go
type EventSink interface {
    Bubble(seq int32, text string) error
    ToolCall(id, name, inputJSON string) error
    ToolResult(id, output string, isError bool) error
}
```

**Simplify `agent/agent.go`:**
- Remove the goroutine + `deltas` channel + `done` channel from the turn loop
- Remove `countingSink` entirely
- Direct call: `turn, err := a.brain.Chat(ctx, a.system, hist, a.tools.Defs())`
- After the tool loop resolves, split `turn.Text` on `\n\n` and send each
  non-empty piece as a `Bubble` immediately

Rough shape of the new loop body:

```go
for {
    turn, err := a.brain.Chat(ctx, a.system, hist, a.tools.Defs())
    if err != nil {
        return convID, err
    }

    if len(turn.Uses) == 0 {
        for i, bubble := range splitBubbles(turn.Text) {
            if err := sink.Bubble(int32(i), bubble); err != nil {
                return convID, err
            }
        }
        finalText = turn.Text
        break
    }

    // tool handling unchanged ...
}
```

`splitBubbles` is a small helper: `strings.Split` on `\n\n`, trim whitespace,
drop empty strings.

---

## Phase 3 — Update proto

**File:** `proto/celine/v1/celine.proto`

- Remove `Typing` message
- Remove `typing` field from `LaleoEvent` oneof

```proto
message LaleoEvent {
  oneof event {
    Message   message     = 2;
    ToolCall  tool_call   = 3;
    ToolResult tool_result = 4;
    Done      done        = 5;
    string    error       = 6;
  }
}
```

Run `buf generate` from `proto/` after the edit.

---

## Phase 4 — Update `rpc`

**File:** `basis/internal/rpc/celine.go`

- Remove `streamSink.Typing` method
- `streamSink` shrinks to 3 methods: `Bubble`, `ToolCall`, `ToolResult`

---

## What stays the same

- `agent` package still exists — transport (`rpc`) and turn logic (history +
  LLM + tool loop + persist) stay separated
- Tool loop logic in `agent.go` unchanged
- Embedding pipeline (`graphe`, `taxis`) unchanged
- `Laleo` proto shape unchanged (server-streaming, one request → stream of events)
- `Anamnesis` unchanged

---

## Trade-off: non-streaming means no partial response

Switching to `Messages.New` means the user sees nothing until Claude finishes
the entire response (~5–15s for long answers). The previous streaming approach
showed the first bubble as soon as Claude produced its first `\n\n` (~2–3s).

**Options:**

| Option | Latency to first bubble | Complexity |
|---|---|---|
| A. `Messages.New` (non-streaming) | Full generation time | Simplest |
| B. Keep `NewStreaming`, accumulate internally, emit on `\n\n` boundary | First `\n\n` in stream | Moderate |
| C. Keep `NewStreaming`, emit complete bubbles immediately | First `\n\n` in stream | Moderate |

Option B/C keeps the goroutine + channel in `llm` only (not in `agent`), which
is a reasonable place for it since streaming is a transport concern. `agent`
stays simple regardless.

**Recommendation:** Option C — keep streaming in `llm` (it's already there,
it's one file, it's not the part that was complicated), remove everything above
that layer. `Chat` in `llm` accumulates complete bubbles from the stream and
returns them as `[]string` alongside tool uses. `agent` stays dumb: receive
bubbles, send them, done.

This would change `Turn`:

```go
type Turn struct {
    Bubbles []string  // complete \n\n-split bubbles, ready to send
    Uses    []ToolUse
}
```

And `llm.Chat` splits on `\n\n` internally as it streams, emitting each bubble
only when complete. `agent` just iterates `turn.Bubbles`.
