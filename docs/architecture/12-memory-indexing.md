## 12. Memory indexing (continuous, queue-based)

Because memory is **RAG**, there is **no idle-timeout session.** We index continuously — every message is embedded and stored *as it happens* — instead of buffering a session in Redis and distilling it at the end. This also **dissolves the deferred scheduling problem**: there's no end-of-session job to fire, so no keyspace-notification / sweeper machinery to design.

### 12.1 Two layers (was three — the session layer is gone)

| Layer | Store | Lifetime | Holds |
|---|---|---|---|
| **Transcripts** | Postgres | forever | every message (`messages`) |
| **Indexed memory** | Postgres + pgvector | forever | per-message embeddings, recall via filtered vector search (`memory_index`) |

The optional curated profile lives in `clients.memory_md` (always-loaded prefix, updated opportunistically). Redis keeps only **caching / rate-limiting / the indexing job queue** — no session TTL, no `dirty` flag.

### 12.2 Per-turn indexing — twice per turn

Each turn produces **two** index jobs:

```
client message arrives  → enqueue index(user message)
Claude response done     → enqueue index(assistant message)
```

Each job: embed the message text (Voyage) → insert into `memory_index` with `owner_sub`. This §12.2–12.4 machinery is the **write path** only. The **read path** (recall) is tiered and partly model-driven — see §12.5 — **not** an unconditional top-K prepend every turn.

### 12.3 Queue-based — to protect the tiny server

Indexing is **async and bounded** so embedding work can't blow the RAM budget, trip the Voyage rate limit, or add latency to the reply:

- **Enqueue, don't block:** the chat stream returns immediately; index jobs run in the background. Indexing never sits on the response path.
- **Bounded worker pool (1–2 workers):** caps concurrent Voyage calls + in-flight memory → natural backpressure on a small box.
- **Queue = a Redis list** (`LPUSH` / `BRPOP`): survives restarts, reuses Redis we already run. *(An in-process buffered channel is the simpler fallback if durability isn't needed.)*
- **Idempotent by `message_id`:** retries / duplicate deliveries don't double-insert.
- **Shed, don't OOM:** if the queue saturates, apply backpressure or drop-with-log rather than let memory grow unbounded.

### 12.4 Why this fits the hardware

No resident session store, no scheduler, no sweeper, no keyspace notifications. Memory upkeep is just a couple of small embed-and-insert jobs per turn flowing through a bounded queue — predictable memory, predictable load. The scheduling discussion we'd deferred is **resolved: not needed.**

### 12.5 Recall policy — three tiers (the read path)

Recall is **not** an unconditional top-K prepend on every turn. That version pays full price *every turn* for memory the turn may not need (RAG recall sits **below** the §13 cache breakpoint, so it can never be cached), injects nearest-but-irrelevant rows on throwaway turns ("thanks!"), and can never **fetch more** when the top-K misses — it's one-shot, the model has no handle to pull. Instead, three tiers matched to how much the turn actually needs:

| Tier | Trigger | Cost | Source |
|---|---|---|---|
| **1. Curated profile** | always | cached (stable prefix) | `clients.memory_md` |
| **2. Thresholded hint** | every turn, but **inject only if the top hit clears a similarity cutoff** | one cheap embed + scan; injected tokens only when something is actually relevant | `memory_index` top-K (§11.2) |
| **3. Agentic `recall`** | the model calls the tool when it knows it's missing context | a tool round-trip; the result then lives in history and **caches** on later turns | `memory_index`, model-chosen query + `k` |

- **Tier 1** carries baseline continuity ("who this person is") in the cached prefix — stable, so it's cheap.
- **Tier 2** keeps the common case zero-extra-hops, and keeps the cached prefix *clean* on turns where nothing matched (kills the "thanks!" pollution for free).
- **Tier 3 is the "fetch more" answer.** The model issues a targeted `recall(query, k)`, reads, and can call again with a wider `k` or a sharper query — iterative refinement a fixed prepend structurally can't do. Because the tool result lands in the message **history**, it's cached on subsequent turns within the TTL (the opposite of the always-uncached prepend).

This is just the agent loop's normal tool round-trip (§3.2 step 5) — `recall` is one more entry in the registry (§6). Tier 2 is the only auto-retrieval; everything deeper is model-driven.

**`recall` is an in-process custom tool (§6), not an MCP server.** The Anthropic Messages API tool loop is what makes agentic retrieval possible — Claude Code uses the *same* loop, nothing special. MCP is merely one *transport* for tools (a separate, decoupled server speaking a protocol). Our memory is our own Postgres in the same binary, so wrapping it in MCP would add a second process (RAM + lifecycle + IPC hop) on a box that already rejected a dedicated vector DB for exactly that reason (§11). Registration is one line — `registry.Register(recallTool)`, `Execute()` runs the §11.2 query. MCP only earns its place when a tool must be **reused across apps**, owned by **another process/language**, or pulled from the **third-party ecosystem** — none true for our own memory store.


