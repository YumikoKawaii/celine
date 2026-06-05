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


