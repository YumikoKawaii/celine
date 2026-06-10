## 14. Response shape — chat-bubble delivery

Real conversation isn't one wall of text; it's a burst of short messages. Celine
replies the same way: a **sequence of short bubbles**. The bubble is the unit —
each is sent *whole* as soon as it is complete, not token-by-token.

> **Design note (applied):** an earlier version simulated a "typing beat" — a
> `Typing` event plus backend `sleep()` before/between bubbles. That fake
> latency was removed (see `docs/plan-simplify-llm-turn.md`). There is **no
> `Typing` event** and **no artificial delay**: humans type a complete thought
> and send it; we do the same.

### 14.1 Two halves

- **Segmentation (model side)** — Celine writes several short messages,
  separated by a blank line.
- **Splitting (backend side)** — the `llm` layer cuts the response into bubbles
  on `\n\n` boundaries and the agent sends each one as it completes. No pacer,
  no timing, no goroutine.

### 14.2 Model side — how she segments

A response-contract instruction (a §13 layer-2 rule) tells her:

- Reply as a sequence of short chat messages, like texting — **not** one paragraph.
- One thought per message; separate messages with a **blank line**.
- Keep each bubble short; don't over-fragment; group tightly-related clauses.
- Keep block content (code fences, lists) **inside a single bubble**.

The blank line (`\n\n`) is the delimiter — natural for the model, unambiguous
against single-line wrapping, and it survives streaming. No blank lines → one
bubble (graceful fallback).

### 14.3 Backend — split, no pace (`internal/llm/claude.go`)

`llm.Chat` streams tokens from Claude only to **accumulate** them; it emits a
bubble each time a `\n\n` boundary completes and returns
`Turn{Bubbles []string, Uses []ToolUse}`. The agent loop iterates `turn.Bubbles`
and sends each as a `Message` event immediately — no delay between them.

```
for each completed bubble (split on \n\n, code-fence-aware):
    sink.Bubble(seq, text)      // sent whole, in order; seq increments
```

- The splitter is **code-fence-aware** (`bubbleBoundary`): never break inside a
  ``` ``` ``` block.
- First bubble can fire at the **first** `\n\n` while the stream is still open —
  streaming is kept inside `llm` purely so a long answer needn't fully generate
  before its first bubble lands.
- Bubble text is tiny, so the per-turn buffer is a RAM non-issue on the box.

### 14.4 Mood & knobs shape the segmentation

There is no pacing to tune, but §13 still shapes *what* she says and *how many*
bubbles result (driven model-side via the response contract):

- **focused** → fewer, longer bubbles.
- **playful** → more, shorter bubbles; emoji.
- The **verbosity** knob caps bubble count.

*(Forward-looking — the persona knobs/mood are designed but not yet wired, §13.)*

### 14.5 Cost / latency

One model generation, segmented backend-side — **no extra token cost** and no
artificial delay. The only state added is a small per-turn bubble slice.
