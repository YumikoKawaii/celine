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

### 5.1 Current implementation (as built — Step 2, "brain")

The wiring that exists today. The one loop reads top-to-bottom on the left
(request → Claude) and bottom-to-top on the right (deltas → paced bubbles →
browser). The two arrows between `agent.go` and `stream.go` are a goroutine
boundary: `StreamChat` produces text deltas on a channel while `paceBubbles`
consumes them and emits typed events back through `streamSink`.

```
                          BROWSER (eidos/)
                  generated TS client · useChatStream
                                │  ▲
                ChatRequest     │  │   stream of ChatEvent
                {conv_id,text}  │  │   (Typing · Message · Error · Done)
                                ▼  │
══════════════════════════════════════════════════════════ basis/  (Go binary)
                                │  │
  cmd/celine/main.go            │  │   h2c + devCORS, mux.Handle(path)
   ├─ env: ANTHROPIC_API_KEY    │  │   reads CELINE_ADDR
   ├─ llm.New(key, CELINE_MODEL)│  │
   └─ agent.New(brain, prompt)  │  │
                                ▼  │
  internal/rpc/chat_service.go (CelineService.Chat)
        │                          ▲
        │ agent.Chat(ctx,          │ streamSink (EventSink adapter)
        │   convID, text, sink)    │   Typing(ms) / Bubble(seq,text)
        ▼                          │   → stream.Send(ChatEvent)
  internal/agent/agent.go  ── the loop we own (§3.2) ──
        │                          ▲
        │  history[convID]         │  paced bubbles
        │  (in-memory STOPGAP      │
        │   → Postgres in Step 4)  │
        │                          │
        │  ┌─ goroutine ───────┐   │
        │  │ llm.StreamChat    │   │
        ▼  ▼                   │   │
  internal/llm/claude.go       │   │      internal/agent/stream.go
   (anthropic-sdk-go)          │   │       paceBubbles  (§14.3)
        │   System = celine.md │   │        • accumulate deltas
        │   (cache breakpoint) │   │        • split on \n\n
        │   Messages = history │   │          (code-fence aware)
        │                      │   │        • typing → sleep →
        ▼                      │   │          bubble → pause
   ┌──────────┐  text deltas   │   │              ▲
   │ Claude   │ ───────────────┴───┼── chan string┘
   │ API      │   (stream.Next)    │
   └──────────┘                    │
                                   │
  internal/agent/prompts/celine.md ┘  (go:embed → SystemPrompt)
        persona + §14.2 segmentation contract + §13.1 invariants
```

**Not wired yet:** tools/registry (§6, Step 3) · Postgres+pgvector & Redis
queue (§11/§12, Step 4) · persona knobs + mood (§13, Step 5) · Google OIDC
auth (§9). The in-memory `history[convID]` map is a deliberate stand-in for the
Postgres `messages` table (§12.1).


