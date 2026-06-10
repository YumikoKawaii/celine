## 4. Tech choices for this stack

| Concern              | Choice                          | Why                                                    |
|----------------------|---------------------------------|--------------------------------------------------------|
| Brain SDK            | `anthropic-sdk-go` (official)   | Tool use + streaming, first-class support              |
| Backend language     | Go                              | Single binary, fast, clean concurrency                 |
| Frontend             | React + Vite + TypeScript       | Fast dev, typed, flexible UI                            |
| Transport / API      | **Connect RPC** (`connectrpc.com/connect`) | Schema-first, typed, server-streaming over HTTP |
| Browser streaming    | **Connect server-streaming RPC**| One typed stream of `LaleoEvent`s — no hand-rolled SSE  |
| Schema / codegen     | **Protobuf + Buf** (`buf generate`) | One `.proto` → Go handlers + TS client, always in sync |
| Auth                 | **Google OIDC, server-side code exchange** | Browser gets the sign-in URL (`Eisodos`), redirects, then hands the `code` to the server (`Metabole`); the server swaps it with Google and issues its **own** HS256 JWT (claims: `sub`, `prosopon_id`, `conversation_id`). A server interceptor verifies that `Bearer` JWT on every RPC and keys all data on `sub`. Unset `CELINE_JWT_SECRET` = dev mode (anon, no auth). |
| Durable storage      | **Postgres** (`pgx`)            | Clients, conversations, memory, projects — on target server |
| Hot / ephemeral state| **Redis**                       | Indexing job queue, caching, rate limiting, fan-out |
| Secrets              | env / `.env` (never in repo)    | Keep `ANTHROPIC_API_KEY` + OAuth creds out of source control |


