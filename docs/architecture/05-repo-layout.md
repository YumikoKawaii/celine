## 5. Proposed repo layout

Monorepo: the **Go module lives in `basis/` (not at the root)**, the React app in `eidos/`, and the proto + Buf config sit at the top so codegen fans out into both.

```
celine/
├── proto/                        # the contract + its codegen config (run buf here)
│   ├── celine/v1/celine.proto    # service + message definitions (source of truth)
│   ├── buf.yaml                  # Buf module config
│   └── buf.gen.yaml              # codegen: Go → ../basis/gen, TS client → ../eidos/src/gen
├── basis/                        # the Go module (go.mod lives HERE, not at root)
│   ├── go.mod                    # module github.com/YumikoKawaii/celine/basis (go 1.25)
│   ├── gen/                      # generated Go (checked in or build-step)
│   │   └── celine/v1/...
│   ├── cmd/celine/main.go        # entrypoint, wires everything
│   └── internal/
│       ├── rpc/                  # Connect service impl (handlers)
│       │   └── celine.go         # implements the Celine service (Laleo, Anamnesis)
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
├── eidos/                        # React app (Vite + TS)
│   ├── src/
│   │   ├── gen/                  # generated TS client (from buf)
│   │   ├── App.tsx
│   │   ├── components/Chat.tsx
│   │   ├── components/ToolCard.tsx   # render tool calls nicely
│   │   └── hooks/useChatStream.ts    # consumes the Chat stream
│   └── ...
├── docs/architecture/            # the design docs — one small file per § (source of truth)
│   ├── README.md                 # index + § → file map
│   └── 01-core-idea.md … 14-response-shape.md
└── README.md
```

### 5.1 Current implementation (as built — Steps 1–5: brain + tool loop + store + auth)

```
                          BROWSER (eidos/)
                  generated TS client · useChatStream
                                │  ▲
              LaleoRequest      │  │   stream of LaleoEvent
              {text}            │  │   (Message · ToolCall
                                ▼  │    ToolResult · Done · Error)
══════════════════════════════════════════════════════════ basis/  (two binaries)

  cmd/celine/main.go  ── config.LoadServer() ──────────────────────────────┐
   ├─ mneme.NewDB(DBDsn)            Postgres                               │
   ├─ redis.NewClient(RedisAddr)    Redis  → taxis.New                     │
   ├─ hermes.NewAuthInterceptor     JWT verify, claims → ctx               │
   ├─ ergon.NewRegistry + Register(NewWebSearch)                           │
   ├─ llm.New(AnthropicKey, Model, MaxTokens)                              │
   └─ agent.New(brain, prompt, prosopons, convs, msgs, queue, tools)       │
                                │  │                                       │
  internal/rpc/celine.go        │  │  h2c + devCORS                       │
  (Celine.Laleo · Anamnesis)    │  │  sub ← hermes.SubFromContext          │
        │                       ▼  │                                       │
        │  agent.Chat(ctx, sub, text, sink)                                │
        ▼                          ▲                                       │
  internal/agent/agent.go          │  streamSink → stream.Send(LaleoEvent) │
   ├─ prosopons.Get(sub)           │   (Bubble · ToolCall · ToolResult)    │
   ├─ convs.GetOrCreate            │                                       │
   ├─ msgs.List (history)          │  ── tool loop ──                      │
   ├─ msgs.Create(user) + enqueue  │   tool_use? → ergon.Execute → append  │
   │       ▼                       │   result to hist[], loop; else break  │
   │   llm.Chat ───────────────────┘   then msgs.Create(asst) + enqueue    │
   ▼                                                                       │
  internal/llm/claude.go                                                   │
   (anthropic-sdk-go) — NewStreaming, accumulate, split bubbles on \n\n    │
   System = celine.md (cached) · Messages = history · Tools = registry     │
        │                                                                  │
        ▼                                                                  │
   ┌──────────┐  Turn{Bubbles, Uses}                                       │
   │ Claude   │  (stop_reason=tool_use → Uses; else end_turn)              │
   └──────────┘                                                            │
                                                                           │
  internal/mneme/          Postgres store                                  │
   ├─ Prosopons             Get · Upsert                                   │
   ├─ Conversations         GetOrCreate · List                             │
   ├─ Messages              Create · List · Get                            │
   └─ Memories              Insert (pgvector)                              │
                                                                           │
  internal/hermes/          auth                                           │
   ├─ GoogleAuth             AuthURL · Exchange (server-side code swap)    │
   ├─ Issuer / Verifier      HS256 JWT, 30-day TTL                        │
   └─ AuthInterceptor        unary + streaming, skips /Hermes/ routes     │
                                                                           │
  ── cmd/worker/main.go  ── config.LoadWorker() ───────────────────────────┘
  internal/graphe/
   ├─ OllamaClient           POST /api/embed → snowflake-arctic-embed:xs
   └─ Worker                 BRPOP → embed → INSERT memory_index ON CONFLICT DO NOTHING
```

**Wired:** typed `Laleo` stream · real Claude brain with the `ergon` tool loop (`web_search`) · Postgres+pgvector store · async indexing **write path** (agent enqueues → `cmd/worker` embeds via Ollama → pgvector) · Google OIDC + JWT auth (`Hermes`).

**Not wired yet:** memory recall **read path** — the §12.5 tiers (curated `memory_md` in the prefix, the thresholded auto-hint, and the agentic `recall` tool) are absent; `agent.Chat` calls Claude with history only · persona knobs + scheduled mood (§13) — `SystemPrompt()` returns the raw `celine.md` blob, no `Persona` struct / archetypes / `moodForClock` · eidos frontend catch-up (dead `typing` state in `useChatStream.ts`, full Hermes auth flow).


