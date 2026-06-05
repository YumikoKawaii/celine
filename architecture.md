# Celine — Architecture

A personal assistant ("JARVIS-style") with **Claude as the brain**, a **Go backend**, and a **React web UI**.

---

## 1. Core idea

At heart, Celine is a single loop:

> **You talk → Celine thinks (Claude) → Celine acts (tools) → Celine reports back.**

The intelligence is just Claude with a good system prompt and a set of tools it is allowed to call. Everything else is plumbing around that loop.

---

## 2. High-level architecture

```
┌──────────────────┐  Connect  ┌─────────────────────────────────┐
│   React UI        │   RPC     │        Go backend               │
│                   │  server   │                                 │
│  Chat window      │ streaming │  ┌──────────────────────────┐  │
│  Streaming tokens │◀─────────▶│  │  Agent loop (orchestrator)│  │
│  Tool-call cards  │  (typed,  │  │  - calls Claude           │  │
└──────────────────┘  protobuf)│  │  - runs tools             │  │
        ▲                      │  │  - streams back           │  │
        │ generated TS client  │  └──────────┬───────────────┘  │
        │ from .proto          │             │                   │
                             │   ┌─────────▼──────────┐       │
                             │   │ Anthropic SDK (Go) │──────▶│ Claude API
                             │   └────────────────────┘       │
                             │   ┌────────────────────┐       │
                             │   │ Tool registry      │       │
                             │   │ web/memory/files…  │       │
                             │   └────────────────────┘       │
                             │   ┌────────────────────┐       │
                             │   │ Postgres │ Redis    │       │  durable store │ cache·queue·rate-limit
                             │   └────────────────────┘       │
                             └─────────────────────────────────┘
```

---

## 3. The four pieces that matter

### 3.1 The brain — Claude via the Anthropic API
- Use **`anthropic-sdk-go`** (official SDK) with the Messages API.
- **Tool use (function calling):** Claude decides *when* to reach for a tool; the Go code executes it and feeds the result back.
- **Prompt caching** on the system prompt + tool definitions, so we don't pay full price every turn.
- **Streaming** responses so the UI feels alive.
- Default model: latest Claude (e.g. `claude-opus-4-8` for quality, `claude-sonnet-4-6` for cheaper/faster turns). Make it configurable.

### 3.2 The agent loop — our code (the heart we own)
```
1. Receive user message via the Chat server-streaming RPC (client identified by Google sub)
2. Recall context (tiered, §12.5): curated profile (always) + a thresholded memory hint
   (top-K vector search, injected only above a similarity cutoff) + recent transcript;
   enqueue an index job for the incoming user message. Deeper memory = the `recall` tool.
3. Call Claude with: system prompt + recalled memory + tools + history (prompt-cached)
4. Stream ChatEvent messages back to the browser as they arrive
5. If Claude emits tool_use blocks:
      run each tool → append tool_result → go back to step 3
   else:
      final answer — persist to Postgres, enqueue an index job for the assistant message,
      close the stream
```

### 3.3 Tools — what makes it an *assistant*, not just chat
Each tool = a function + a JSON schema description. Start tiny, grow later.

| Tool                | Purpose                                  | Milestone |
|---------------------|------------------------------------------|-----------|
| `web_search`        | Current info from the internet           | early     |
| `remember` / `recall` | Persistent long-term memory — **agentic, on-demand; in-process tool, not MCP** (§12.5) | early |
| `manage_reminders`  | Todos / alarms                           | mid       |
| `read_files`        | Read local files (sandboxed)             | mid       |
| `run_command`       | Touch the machine (sandbox carefully!)   | later     |
| calendar / email / Spotify / smart home | Quality-of-life          | later     |

### 3.4 Memory — so she's *yours* (per-client)
Celine is **one bot serving many clients**; memory is keyed per client (Google `sub`). Memory is **RAG-based**: every message is embedded and indexed *as it happens*, then recalled by filtered vector search. There is no session-buffer-then-distill step (see §12).
- **Indexed memory (RAG):** every user and assistant message is embedded and stored in pgvector, scoped by `owner_sub`. Recall = top-K filtered vector search over *this client's* rows. Indexing is per-turn and queue-based (see §12).
- **Conversation transcripts:** per-conversation history in Postgres — *what we said.* Replayed as message history for the active thread.
- **Curated profile (`memory.md`, optional):** a small always-loaded summary of *who this person is*, injected into the cached prefix so core facts are present even without a query match. Decoupled from sessions — updated opportunistically, not at any session end. Optional; RAG recall covers most needs.

---

## 4. Tech choices for this stack

| Concern              | Choice                          | Why                                                    |
|----------------------|---------------------------------|--------------------------------------------------------|
| Brain SDK            | `anthropic-sdk-go` (official)   | Tool use + streaming, first-class support              |
| Backend language     | Go                              | Single binary, fast, clean concurrency                 |
| Frontend             | React + Vite + TypeScript       | Fast dev, typed, flexible UI                            |
| Transport / API      | **Connect RPC** (`connectrpc.com/connect`) | Schema-first, typed, server-streaming over HTTP |
| Browser streaming    | **Connect server-streaming RPC**| One typed stream of `ChatEvent`s — no hand-rolled SSE   |
| Schema / codegen     | **Protobuf + Buf** (`buf generate`) | One `.proto` → Go handlers + TS client, always in sync |
| Auth                 | **Google OAuth / OIDC only**    | Verify Google ID token; key everything on the `sub` claim |
| Durable storage      | **Postgres** (`pgx`)            | Clients, conversations, memory, projects — on target server |
| Hot / ephemeral state| **Redis**                       | Indexing job queue, caching, rate limiting, fan-out |
| Secrets              | env / `.env` (never in repo)    | Keep `ANTHROPIC_API_KEY` + OAuth creds out of source control |

---

## 5. Proposed repo layout

Monorepo: the **Go module lives in `backend/` (not at the root)**, the React app in `web/`, and the proto + Buf config sit at the top so codegen fans out into both.

```
celine/
├── proto/
│   └── celine/v1/celine.proto    # service + message definitions (source of truth)
├── buf.yaml                      # Buf module config
├── buf.gen.yaml                  # codegen: Go → backend/gen, TS client → web/src/gen
├── backend/                      # the Go module (go.mod lives HERE, not at root)
│   ├── go.mod                    # module github.com/YumikoKawaii/celine/backend (go 1.25)
│   ├── gen/                      # generated Go (checked in or build-step)
│   │   └── celine/v1/...
│   ├── cmd/celine/main.go        # entrypoint, wires everything
│   └── internal/
│       ├── rpc/                  # Connect service impl (handlers)
│       │   └── chat_service.go   # implements CelineService
│       ├── agent/                # the loop: Claude ↔ tools
│       │   ├── loop.go
│       │   └── stream.go
│       ├── tools/                # tool registry + each tool
│       │   ├── registry.go
│       │   ├── tool.go           # the Tool interface
│       │   ├── websearch.go
│       │   └── memory.go         # the in-process recall tool (§12.5)
│       ├── auth/                 # Google OIDC token verification + interceptor
│       │   └── google.go
│       ├── store/                # Postgres (durable) + Redis (cache/queue)
│       │   ├── postgres.go
│       │   └── redis.go
│       └── llm/                  # anthropic client wrapper + caching
│           └── claude.go
├── web/                          # React app (Vite + TS)
│   ├── src/
│   │   ├── gen/                  # generated TS client (from buf)
│   │   ├── App.tsx
│   │   ├── components/Chat.tsx
│   │   ├── components/ToolCard.tsx   # render tool calls nicely
│   │   └── hooks/useChatStream.ts    # consumes the Chat stream
│   └── ...
├── architecture.md
└── README.md
```

---

## 6. The Tool interface (so growth is trivial)

```go
type Tool interface {
    Name() string
    Description() string
    Schema() map[string]any        // JSON schema for inputs
    Execute(ctx context.Context, input json.RawMessage) (string, error)
}
```

Add a tool → implement these methods → `registry.Register(...)`. Claude sees it automatically.

---

## 7. RPC API (Connect, initial)

Defined in `proto/celine/v1/celine.proto` — one source of truth, codegen for both sides via `buf generate`.

```proto
service CelineService {
  // Send a message; server streams the reply (tokens + tool activity).
  rpc Chat(ChatRequest) returns (stream ChatEvent);

  // Load a conversation's messages.
  rpc GetHistory(GetHistoryRequest) returns (GetHistoryResponse);

  // List conversations.
  rpc ListConversations(ListConversationsRequest) returns (ListConversationsResponse);
}

message ChatRequest {
  string conversation_id = 1;  // empty = start a new conversation
  string text = 2;
}

// One streamed event. `oneof` keeps the stream typed end to end.
// Chat persona delivers whole bubbles with typing beats, not token deltas (see §14).
message ChatEvent {
  oneof event {
    Typing      typing       = 1;  // "…" indicator before a bubble
    Message     message      = 2;  // one complete chat bubble
    ToolCall    tool_call    = 3;  // tool started
    ToolResult  tool_result  = 4;  // tool finished
    Done        done         = 5;  // final, stream closes after
    string      error        = 6;
  }
}

message Typing  { int32 ms_hint = 1; }            // how long to show the indicator
message Message { int32 seq = 1; string text = 2; } // a whole bubble, in order
```

- **`Chat`** is a **server-streaming RPC**: the browser sends one request, the backend streams `ChatEvent`s until done. This replaces SSE entirely — the stream is typed, generated, and identical on both ends.
- The React side consumes it with the generated client (`for await (const event of client.chat(req))`), switching on the `oneof`.
- Connect speaks plain HTTP/JSON in the browser (no gRPC infra needed), and the same service is callable via gRPC/grpc-web later for free.

---

## 8. Build order (each step is demoable)

1. **Plumbing** — define `celine.proto`, run `buf generate`, stand up the Connect server with a `Chat` RPC that streams hardcoded `Message` bubbles (with `Typing` beats); React chat window consumes the generated client and renders the bubbles. *No Claude yet — prove the typed pipe.*
2. **Brain** — wire `anthropic-sdk-go`, real streaming chat. Celine talks.
3. **Tool loop** — add the registry + one tool (`web_search`), handle the tool-use round-trip, render tool cards in the UI.
4. **Memory** — embed every message into pgvector per client (queue-based, twice per turn), recall via filtered vector search. Optional `clients.memory_md` profile in the cached prefix. She remembers each person.
5. **Personality** — the layered persona that makes her *Celine*: base prose + tunable knobs + scheduled mood (see §13).
6. **Grow** — reminders, files, calendar, then voice / auth / deploy whenever.

---

## 9. Open decisions (defaults chosen, easy to revisit)

- **Connect server-streaming RPC** for chat (typed, schema-first). Bidi streaming later if we add voice / interrupts.
- **Postgres** (durable) + **Redis** (hot session state) — both already on the target server.
- **Auth: Google OAuth / OIDC only.** Verify the ID token server-side; key all data on the `sub` claim. Multi-tenant from day one.
- **Per-client memory:** RAG over indexed messages (pgvector, filtered by `owner_sub`), indexed per-turn via a bounded queue. Optional curated `memory.md` profile, decoupled from sessions.
- **Recall is tiered, not an always-on prepend (§12.5):** curated profile (cached) + thresholded auto-hint + an **agentic `recall` tool** for on-demand "fetch more." `recall` is an **in-process custom tool, not MCP.**
- **No idle-timeout sessions:** indexing runs continuously per request, so the previously-deferred end-of-session scheduling problem is gone (see §12).
- **API key + OAuth creds in env / `.env`**, never committed.
- **Model:** configurable; default to latest Claude.

---

## 10. Security notes

- Keep `ANTHROPIC_API_KEY` in env, never in the repo.
- `run_command` / file tools must be **sandboxed** (allowlist, working-dir jail, confirmation step) before they touch anything real.
- If ever exposed beyond localhost: add auth + rate limiting.

---

## 11. Projects & Knowledge Base (claude.ai "Projects" equivalent)

**Equation:** a Project = a **custom system prompt** + a **knowledge base** (a pile of docs). When you chat "inside" a project, Celine answers with that persona and that context loaded.

### 11.1 Two retrieval strategies (start simple, grow into the second)

| Strategy | When | How |
|---|---|---|
| **A. Stuff-it-all** (MVP) | KB fits in context (~up to tens of K tokens) | Concatenate every doc into one **prompt-cached** context block, prepended to each request. Dead simple, no embeddings. |
| **B. Retrieval (RAG)** | KB too big to fit | Chunk docs → embed → vector search → inject only the **top-K relevant** chunks per turn. |

Build **A first** — with prompt caching it's cheap and covers most personal projects. Add **B** only when a KB outgrows the context window. The chat code barely changes: both strategies just produce "the context block to prepend."

> ⚠️ **Embeddings note:** Anthropic does **not** provide an embeddings API. For strategy B, use a third-party embedder — Anthropic recommends **Voyage AI**; default to **`voyage-3-lite`** here (512-dim → 2 KB/vector, ~3× cheaper, recall loss irrelevant at personal scale). OpenAI embeddings or a local model also work. Vector store: **Postgres + `pgvector`** (already have Postgres), or plain cosine in Go at small scale.

> 🧭 **Why not a dedicated vector DB (Qdrant, etc.)?** Overkill at this scale, and costly on a tiny box — another process with its own RAM, deployment, and backups. `pgvector` reuses the Postgres we already run, keeps vectors *next to* their rows (filter + JOIN in one SQL query), and stays in sync for free. At personal scale (≪ 1M vectors) use **brute-force exact scan, no HNSW** — the index's resident-RAM appetite is the one thing the box can't spare; add **IVFFlat** only past ~20–50K vectors. Reach for a dedicated store only at millions of vectors / high QPS / sharding. **Per-tenant isolation = a `WHERE owner_sub = $1` filter, not a separate "collection."**

### 11.2 Data model (Postgres) — everything keyed per client

```sql
clients(
  sub TEXT PRIMARY KEY,          -- Google `sub` claim (stable identity)
  email, display_name,
  memory_md TEXT,                -- the per-client durable profile (agent-maintained)
  preferences JSONB,             -- per-client persona knobs + archetype (§13.2)
  persona_note TEXT,             -- free-text "how to treat you" (§13.2.1; capped, guarded)
  created_at, updated_at
)

projects(
  id, owner_sub REFERENCES clients(sub),  -- projects are per-client
  name, system_prompt,                    -- system_prompt = the project's free-text persona (§13.2.1)
  preferences JSONB,                      -- per-project persona knobs + archetype (§13.2)
  created_at
)

documents(
  id, project_id, name, content, token_count, created_at
)

-- strategy B (project KB RAG): tenant key denormalized onto chunks for fast filtered search
chunks(
  id,
  document_id REFERENCES documents(id),
  owner_sub  TEXT NOT NULL,        -- denormalized → filter without a join
  project_id BIGINT,               -- denormalized → project-scoped search
  ordinal, content,
  embedding  vector(512)           -- voyage-3-lite
)
CREATE INDEX ON chunks (owner_sub, project_id);   -- btree, cheap; narrows the scan first

-- per-client memory RAG: every message embedded + indexed (see §12)
memory_index(
  id,
  owner_sub  TEXT NOT NULL REFERENCES clients(sub),  -- tenant isolation
  message_id REFERENCES messages(id),
  role,                            -- user | assistant
  content,                         -- the text that was embedded
  embedding  vector(512),
  created_at
)
CREATE INDEX ON memory_index (owner_sub);

conversations(
  id, owner_sub REFERENCES clients(sub),
  project_id NULLABLE,           -- nullable = no project
  title, created_at
)

messages(
  id, conversation_id, role, content, created_at
)
```

**Per-tenant isolation = a column + a `WHERE`, not a separate store.** Both RAG stores carry a denormalized `owner_sub`; "search only this client's data" is one filtered query:

```sql
-- project KB recall (scoped to a client + project)
SELECT content FROM chunks
WHERE owner_sub = $1 AND project_id = $2
ORDER BY embedding <=> $3 LIMIT 8;

-- per-client memory recall
SELECT content FROM memory_index
WHERE owner_sub = $1
ORDER BY embedding <=> $2 LIMIT 8;
```

With brute-force scan (no ANN index) the `WHERE` is applied *during* the scan — so it's both **exact** (no filtered-ANN recall gotcha) and **faster** (cosine runs only over that client's rows). The btree index narrows to those rows first.

### 11.3 How a project-scoped chat is assembled

```
1. Chat request arrives with a project_id
2. Load project.system_prompt
3. Build the KB context block:
     - Strategy A: concatenate all project documents
     - Strategy B: embed the user's message → top-K chunks
4. Send to Claude:
     system = [ project.system_prompt,
                {kb_block, cache_control: ephemeral} ]   ← cached prefix
     messages = conversation history + new user message
5. Stream reply back (same agent loop as always — tools still work)
```

The KB block goes in the **cached** part of the system prompt, so re-sending it each turn is cheap (you pay full price once, ~90% less on cache hits within the TTL).

### 11.4 RPC additions (`celine.proto`)

```proto
rpc CreateProject(CreateProjectRequest) returns (Project);
rpc ListProjects(ListProjectsRequest)   returns (ListProjectsResponse);
rpc UpdateProject(UpdateProjectRequest) returns (Project);   // edit system prompt
rpc AddDocument(AddDocumentRequest)     returns (Document);   // upload to KB
rpc ListDocuments(ListDocumentsRequest) returns (ListDocumentsResponse);
rpc DeleteDocument(DeleteDocumentRequest) returns (DeleteDocumentResponse);

// ChatRequest gains an optional project_id (already supports conversation_id).
```

### 11.5 Where it lands in the build order

Slots in as a milestone after **Memory (4)** and before/around **Personality (5)** — projects are essentially "scoped personalities + scoped memory," so they reuse both the system-prompt and the Postgres plumbing already built.

1. `projects` + `documents` tables, `CreateProject` / `AddDocument` RPCs, minimal UI (project list + a textarea for the system prompt + file upload).
2. Wire project context into the chat assembly (**Strategy A**, prompt-cached).
3. *(Later)* Add `chunks` + embeddings + top-K retrieval (**Strategy B**) once a KB gets large.

---

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

---

## 13. Personality (Celine's characteristic)

Celine's character is **the system prompt**, but structured as composable layers, never one prose blob. This keeps the "soul" stable and code-reviewed while letting behaviour flex per client, per project, and **per mood on a schedule** — without rewriting prose. Configurability runs along two axes: **depth** (what you can change — bounded dials → guarded free text → the code-only soul) and **scope** (who it's for — global → project → client → mood).

```
   invariant boundaries (§13.1)         ✗ nothing below can override these — the floor
        ▲ under everything
base persona (immutable prose)          ← the soul, version-controlled, code-reviewed
        ▲ placeholders filled by
   knobs (enums) + archetype macro      ← Depth 1: bounded dials; global → project → client
        ▲ optionally extended by
   free-text personalization            ← Depth 2: per-client / per-project prose, guarded (capped, below base)
        ▲ overlaid by
   mood (scheduled overlay)             ← transient: nudges knobs + adds a flavor line
        ↓
   final system-prompt prefix  (cached — see §3.2 / §11.3 assembly)
```

### 13.1 Base persona — the character sheet

A version-controlled prose file (e.g. `internal/agent/prompts/celine.md`, embedded via Go `embed`). Edited rarely, code-reviewed like code. It contains `{{placeholders}}` the knobs fill in. Sections:

- **Identity** — who she is, her name, her role as a personal assistant.
- **Voice & tone** — how she speaks at rest (the default the knobs modulate).
- **Relationship to the user** — JARVIS-style: loyal, proactive, anticipates needs, addresses the client by name (from `clients.display_name`).
- **Principles & boundaries** — *honesty: never fabricate; say "I don't know"*; report tool failures plainly; safety / refusal stance. These are **invariant** — knobs and moods can't override them.
- **Signature quirks** — the small consistent touches that make her *her*.

### 13.2 Knobs — tunable dials

A small struct of **enumerated** dials rendered *into* the prose, so behaviour tunes without prompt surgery. Enums (not free text) are the point: a client can set any of these and still can't break coherence or jailbreak the persona — a dial only has valid positions.

```go
type Persona struct {
    // Voice
    Verbosity   string // "concise" | "balanced" | "detailed"
    Formality   string // "casual" | "neutral" | "formal"
    Warmth      string // "reserved" | "neutral" | "warm"
    Humor       string // "none" | "dry" | "playful"
    Emoji       bool
    Language    string // "" = match the user
    // Manner
    Proactivity string // "passive" | "responsive" | "anticipatory"  — how JARVIS-like
    Candor      string // "diplomatic" | "balanced" | "blunt"
    Curiosity   string // "none" | "light" | "inquisitive"           — asks follow-ups?
    Opinionated string // "neutral" | "leans" | "takes-a-stance"
    Address     string // "name" | "nickname" | "honorific"          — how she calls the client
    // Action
    Initiative  string // "confirm-first" | "act-then-report" | "autonomous" — tool boldness, still under §10 safety
    ToolNarrate bool   // announce tool use, or stay silent
}
```

**Archetype macro — a preset of presets.** Rather than set a dozen dials, a client or project picks one named archetype that expands to a full `Persona`; explicit knobs then override individual fields on top.

```go
// Archetypes expand to a Persona, used as the *starting point* before the override chain.
var Archetypes = map[string]Persona{
    "butler":        {Formality: "formal", Warmth: "warm", Proactivity: "anticipatory", Candor: "diplomatic", Initiative: "confirm-first"},
    "hacker-friend": {Formality: "casual", Humor: "dry",  Candor: "blunt",  Emoji: true, Initiative: "act-then-report"},
    "coach":         {Warmth: "warm", Curiosity: "inquisitive", Proactivity: "anticipatory", Candor: "balanced", Opinionated: "takes-a-stance"},
}
```

**Resolution order (later wins):** `archetype` expands first → **global defaults → per-project → per-client (`preferences`) → mood overlay**. Any explicit dial beats the archetype's implied value at the same-or-narrower scope. Pair with config-level `temperature` (lower = steadier persona) and `model`.

### 13.2.1 Free-text personalization (the expressive layer, Depth 2)

When dials aren't enough, two **free-text** fields let a human (or Celine herself) speak in prose:

- **Per-client** `clients.persona_note` — *"how should Celine treat you?"* (like claude.ai custom instructions). Set by the client, or learned and written by Celine via the `remember` tool over time.
- **Per-project** `projects.system_prompt` — the project's persona/instructions (already in the data model, §11.2).

Both inject into the **cached prefix** (stable per client/project → cheap, §13.4). Because free text is unbounded, it is **guarded**:

- it sits **below** the immutable base persona, so on any conflict the base persona and the **invariant boundaries (§13.1) win** — free text can never loosen honesty/safety;
- a **length cap** keeps it from blowing the prefix / RAM budget;
- it *adds colour*, it doesn't *replace* the soul.

This is the deepest **runtime-configurable** layer. Deeper still is the base persona itself (§13.1) — but that's code-reviewed prose, changed by PR, never runtime config.

### 13.3 Mood — scheduled personality overlay

A **mood** is a named preset that (a) overrides some knobs and (b) adds one short *flavor line* to the prompt. Mood is global to Celine — *she* has a mood — not per-client.

```go
var Moods = map[string]Persona{ /* knob overrides per mood */ }
var MoodFlavor = map[string]string{
    "focused":  "Crisp and to the point right now; skip the small talk.",
    "cheerful": "Bright and encouraging this morning.",
    "mellow":   "Calm, unhurried, low-key — it's late.",
    "playful":  "Feeling playful; a little extra wit is welcome.",
}
```

**Resolution at assembly time:**

```
mood := redis.Get("celine:mood")        // optional override (scheduled or manual)
if mood == "" { mood = moodForClock(now) }   // deterministic default by time-of-day
p := defaults.merge(projectPrefs).merge(clientPrefs).merge(Moods[mood])
prefix := render(basePersona, p) + "\n" + MoodFlavor[mood]
```

- **Default = a pure function of the clock** (`moodForClock`) — morning→cheerful, work hours→focused, late→mellow, weekend→playful. **No background job needed**; it's computed per request.
- **Override = an optional `celine:mood` key in Redis**, set by a tiny cron or by hand (e.g. a celebratory mood on a birthday, with a TTL). This is the *only* place the "on a schedule" mechanism lives — and it's optional sugar on top of the deterministic default.

### 13.4 Caching

The rendered persona sits in the **cached prefix** (layers above the §12 / §11.3 breakpoint). Knobs and mood change slowly — at most a handful of times a day — so a mood flip just invalidates the prefix **once** (one full-price turn, then re-cached for the next window). Cheap. Never put anything per-turn-dynamic (RAG recall) into this block.

### 13.5 Where each piece lives

| Piece | Lives in | Cadence |
|---|---|---|
| Base persona prose | repo file (embedded) | edited rarely, code-reviewed |
| Invariant boundaries | base persona (§13.1) | never overridable |
| Global knob defaults | config | deploy-time |
| Archetype presets | `Archetypes` map (code) | edited rarely |
| Per-project knobs + persona | `projects.preferences` + `projects.system_prompt` | per project |
| Per-client knobs | `clients.preferences` | per client |
| Per-client free-text | `clients.persona_note` (set by client or learned) | per client / opportunistic |
| Current mood | `moodForClock(now)`, override in `celine:mood` | minutes–hours |

---

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
