## 14. Response shape — chat-bubble delivery

Real conversation isn't one wall of text; it's a burst of short messages with little typing pauses between them. Celine replies the same way: a **sequence of short bubbles**, each preceded by a typing indicator and a human-feeling delay. The bubble is the unit — they pop in *whole* after a typing beat, not token-by-token.

### 14.1 Two halves

- **Segmentation (model side)** — Celine writes several short messages, separated by a blank line.
- **Pacing (backend side)** — a delivery pacer turns each completed bubble into *typing indicator → delay → whole bubble → pause*.

### 14.2 Model side — how she segments

A response-contract instruction (a §13 layer-2 rule) tells her:

- Reply as a sequence of short chat messages, like texting — **not** one paragraph.
- One thought per message; separate messages with a **blank line**.
- Keep each bubble short; don't over-fragment; group tightly-related clauses.
- Keep block content (code fences, lists) **inside a single bubble**.

The blank line (`\n\n`) is the delimiter — natural for the model, unambiguous against single-line wrapping, and it survives streaming. No blank lines → one bubble (graceful fallback).

### 14.3 Backend — segment + pace (`internal/agent/stream.go`)

Stream tokens from Claude → accumulate the current bubble → on a `\n\n` boundary the bubble is complete → hand it to the pacer:

```
for each completed bubble:
    emit Typing{ ms_hint = typingDelay(bubble) }
    sleep(ms_hint)
    emit Message{ seq, text = bubble }
    sleep(interBubblePause)
```

- `typingDelay(b) = clamp(BASE + PER_CHAR·len(b), MIN, MAX)` — e.g. 400 ms + 25 ms/char, capped ~2.5 s.
- `interBubblePause` ≈ 300–800 ms.
- Generation runs **ahead** of delivery (bubbles buffer); text is tiny, so RAM is a non-issue on the box.
- First bubble fires at the **first** `\n\n`, not after full generation → stays responsive.
- The splitter is **code-fence-aware**: never break inside a ``` block.

### 14.4 Mood & knobs drive the rhythm

Pacing reads §13, so mood shapes not just *what* she says but *how it lands*:

- **focused** → fewer, longer bubbles; snappier delays.
- **playful** → more, shorter bubbles; emoji; slightly longer pauses.
- **mellow** → slower pacing.
- The **verbosity** knob caps bubble count.

### 14.5 Cost / latency

One model generation, segmented backend-side — **no extra token cost**. The pacing delay is *intentional* (the human feel); the `MAX` clamp guarantees it never drags. The only state added is a small per-stream bubble buffer.
