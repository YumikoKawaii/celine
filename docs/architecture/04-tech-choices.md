## 4. Tech choices for this stack

| Concern              | Choice                          | Why                                                    |
|----------------------|---------------------------------|--------------------------------------------------------|
| Brain SDK            | `anthropic-sdk-go` (official)   | Tool use + streaming, first-class support              |
| Backend language     | Go                              | Single binary, fast, clean concurrency                 |
| Frontend             | React + Vite + TypeScript       | Fast dev, typed, flexible UI                            |
| Transport / API      | **Connect RPC** (`connectrpc.com/connect`) | Schema-first, typed, server-streaming over HTTP |
| Browser streaming    | **Connect server-streaming RPC**| One typed stream of `ChatEvent`s — no hand-rolled SSE   |
| Schema / codegen     | **Protobuf + Buf** (`buf generate`) | One `.proto` → Go handlers + TS client, always in sync |
| Auth                 | **Google OIDC, client-side redirect** | Browser runs the Google sign-in flow and holds the ID token; a server interceptor verifies the `Bearer` token on every RPC and keys all data on the `sub` claim. No server-side redirect / `/callback`. |
| Durable storage      | **Postgres** (`pgx`)            | Clients, conversations, memory, projects — on target server |
| Hot / ephemeral state| **Redis**                       | Indexing job queue, caching, rate limiting, fan-out |
| Secrets              | env / `.env` (never in repo)    | Keep `ANTHROPIC_API_KEY` + OAuth creds out of source control |


