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

### 5.1 Current implementation (as built — Step 2 + store + auth)

```
                          BROWSER (eidos/)
                  generated TS client · useChatStream
                                │  ▲
              LaleoRequest      │  │   stream of LaleoEvent
              {conv_id, text}   │  │   (Typing · Message · ToolCall
                                ▼  │    ToolResult · Done · Error)
══════════════════════════════════════════════════════════ basis/  (two binaries)

  cmd/celine/main.go  ── config.LoadServer() ──────────────────────────────┐
   ├─ mneme.NewPool(DBDsn)          Postgres                               │
   ├─ mneme.NewRedis(RedisAddr)     Redis                                  │
   ├─ hermes.NewAuthInterceptor     JWT verify, sub → ctx                  │
   ├─ llm.New(AnthropicKey, Model)                                         │
   └─ agent.New(brain, prompt, convs, msgs)                                │
                                │  │                                       │
  internal/rpc/chat_service.go  │  │  h2c + devCORS                       │
  (Celine.Laleo)                │  │  sub ← hermes.SubFromContext          │
        │                       ▼  │                                       │
        │  agent.Chat(ctx, sub, convID, text, sink)                        │
        ▼                          ▲                                       │
  internal/agent/agent.go          │  streamSink → stream.Send(LaleoEvent) │
   ├─ convs.GetOrCreate            │                                       │
   ├─ msgs.GetHistory              │  internal/agent/stream.go             │
   ├─ msgs.Save(user)              │   paceBubbles (§14.3)                 │
   ├─ ── goroutine ──────────────┐ │   · split on \n\n                    │
   │   llm.StreamChat            │ │   · typing → sleep → bubble           │
   ▼                             │ │              ▲                        │
  internal/llm/claude.go         │ │   chan string┘                        │
   (anthropic-sdk-go)            │─┘                                       │
   System = celine.md (cached)   │                                         │
   Messages  = history           │                                         │
        │                        │                                         │
        ▼                        │                                         │
   ┌──────────┐  text deltas ────┘                                         │
   │ Claude   │  (stream.Next() — tool_use not yet handled)                │
   └──────────┘                                                            │
                                                                           │
  internal/mneme/          Postgres store                                  │
   ├─ ConversationStore     GetOrCreate · List                             │
   ├─ MessageStore          Save · GetHistory · Enqueue → Redis queue      │
   └─ ClientStore           Upsert · Get                                   │
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

**Not wired yet:** `ergon/` tool registry (§6, Step 3) · memory recall read path (§12.5, Step 4) · persona knobs + mood (§13, Step 5) · eidos frontend catch-up (Greek method names, auth flow).


