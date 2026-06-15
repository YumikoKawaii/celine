# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

**Steps 1–5 live.** The design docs in **`docs/architecture/`** (the **source of truth** — read [`docs/architecture/README.md`](docs/architecture/README.md) first, keep it in sync) are one-file-per-section; the `§N` references throughout this file and the code map to the numbered files there (e.g. `§3.2` → `03-the-four-pieces.md`).

What's running: typed Connect stream end-to-end (proto → Go server → React UI), real Claude brain with tool loop (`web_search` via Brave API), Postgres+pgvector store, async memory indexing pipeline (Redis queue → Ollama embedder → pgvector), Google OIDC auth (JWT), and the memory recall **read path** (§12.5: tier-1 curated profile, tier-2 thresholded auto-hint, tier-3 agentic `recall` tool). The next steps wire in the persona knobs + scheduled mood (§13) and grow the tool set.

## What Celine is

A personal assistant ("JARVIS-style") with **Claude as the brain**, a **Go backend**, and a **React web UI**. One bot serving many clients, multi-tenant from day one. The whole system is one loop: *user talks → Claude thinks → tools act → Celine reports back.* Everything else is plumbing around that loop.

## Repo layout

Three sibling modules, each named for its role (the names are deliberate — `basis` = the ground it stands on, `eidos` = the visible form, `proto` = the contract):

| Dir | What it is | Stack |
|---|---|---|
| `proto/` | **The contract.** `celine/v1/celine.proto` → Go + TS via Buf. | Protobuf, Buf v2 |
| `basis/` | **Go backend**, two binaries. `cmd/celine` = RPC server, `cmd/worker` = graphe embedding worker. Generated code in `gen/`, hand-written in `internal/`. | Go 1.25, Connect RPC, h2c |
| `eidos/` | **React web UI.** Consumes the generated TS client. Generated code in `src/gen/`. | React 19 + Vite 7 + TS, bun, Connect-ES v2 |

**The `.proto` is the contract.** Both the Go handlers (`basis/gen/`) and the TS client (`eidos/src/gen/`) are generated from it — change the proto, regenerate, **never hand-edit generated code**.

### Internal packages (`basis/internal/`)

| Package | Role |
|---|---|
| `arche` | Global constants (queue topics, shared keys) |
| `agent` | Core agent loop: history → Claude → tool rounds → persist + enqueue |
| `llm` | Thin `anthropic-sdk-go` wrapper — `StreamChat` → delta channel + `Turn` |
| `ergon` | Tool registry + implementations (`web_search` via Brave API) |
| `mneme` | Postgres layer (GORM): `Prosopons`, `Conversations`, `Messages`, `Memories`; defines `Scope` interface + primordial filters (`KataSub`, `KataProsopon`, `KataConversation`, `KataMessage`) |
| `graphe` | Async embedding pipeline: Ollama client + `Worker` (BRPOPs `arche.GrapheQueue`, embeds, writes pgvector) |
| `taxis` | Redis queue thin wrapper — generic `Enqueue`/`Dequeue`; consumers own their data shape |
| `hermes` | Google OAuth exchange, JWT issue/verify, auth interceptor |
| `rpc` | Connect handlers: `CelineService` (`Laleo` stream) + `HermesService` (auth flow) |
| `config` | Env-var loading for both binaries |

## Build & codegen

```bash
# Regenerate Go + TS from the proto (run from proto/; out paths climb to siblings)
cd proto && buf generate

# Backend — RPC server
cd basis && go build ./... && go run ./cmd/celine   # serves on :8080 (CELINE_ADDR to override)

# Backend — embedding worker (separate process)
cd basis && go run ./cmd/worker

# Frontend
cd eidos && bun install && bun dev                   # Vite dev server
cd eidos && bun run typecheck                         # tsc --noEmit
cd eidos && bun run build                             # tsc -b && vite build
```

`buf.yaml` lints `STANDARD` but excepts `RPC_RESPONSE_STANDARD_NAME` — `Laleo` streams a typed `LaleoEvent` oneof, not a `LaleoResponse` (intentional, §7).

## Intended toolchain (per docs/architecture/)

| Concern | Choice | Status |
|---|---|---|
| Backend | Go, two binaries (server + worker) | ✅ `basis/cmd/celine`, `basis/cmd/worker` |
| Transport | **Connect RPC** (`connectrpc.com/connect`), server-streaming `Laleo` | ✅ wired (h2c + dev-CORS) |
| Schema/codegen | **Protobuf + Buf** | ✅ `proto/`, codegen working |
| Frontend | React + Vite + TypeScript, in `eidos/` | ✅ consumes generated client |
| Brain SDK | `anthropic-sdk-go` (Messages API, tool use, streaming, prompt caching) | ✅ `internal/llm/claude.go` |
| Durable store | **Postgres** (GORM + `pgx`) + **pgvector** | ✅ `internal/mneme/` |
| Hot state | **Redis** (indexing job queue via `taxis`) | ✅ `internal/taxis/`, `internal/graphe/` |
| Auth | **Google OIDC** — browser holds the ID token; server interceptor verifies `Bearer` token per RPC, keys data on `sub`. Auth RPCs: `Eisodos` (get URL), `Metabole` (exchange code), `Gnorizo` (current user), `Exodos` (logout) | ✅ `internal/hermes/`, `internal/rpc/hermes_service.go` |

## Architecture you must understand before editing

These span multiple files and are non-obvious:

- **Agent loop** (`basis/internal/agent/`) — the heart we own. Load history → call Claude with cached system prefix + tools → if `tool_use` blocks, run tools and loop; else finalize. After each turn, enqueue both messages (user + assistant) for embedding. See `docs/architecture/03-the-four-pieces.md` §3.2.
- **Response shape — segmentation + pacing (§14):** Celine replies as a **sequence of short bubbles**. **Model side** segments by writing a blank line (`\n\n`) between thoughts; **backend side** (`basis/internal/agent/stream.go`) is a code-fence-aware splitter + pacer: *typing beat → delay → whole bubble → pause*, firing the first bubble at the first `\n\n`. One generation, no extra token cost.
- **Layered system prompt (cached prefix), most-stable → least-stable:** base persona → response contract → tool defs → project prompt → project KB → curated profile (`memory.md`) → **cache breakpoint** → per-turn RAG recall → history. **Never put per-turn-dynamic content (RAG recall) above the breakpoint** — it would break caching every turn. (§13)
- **Memory is RAG, not sessions.** Every message (user *and* assistant — twice per turn) is embedded and indexed into `memories`, scoped by `owner_sub`; recall is a filtered vector search at the next turn's start. Indexing is async via the Redis list at `arche.GrapheQueue` — agent pushes message IDs, graphe worker fetches content, embeds via Ollama, writes to pgvector. (§12)
- **`mneme.Scope` interface** — all repo `Get`/`List` methods accept `Scope` for composable query filtering. Primordial implementations (`KataSub`, `KataProsopon`, `KataConversation`, `KataMessage`) cover common patterns; consumers implement `Scope` for custom logic.
- **Per-tenant isolation = a `WHERE prosopon_id = $1` filter, not a separate store.** Use **pgvector**, not a dedicated vector DB. At this scale: **brute-force exact scan, no HNSW**; add IVFFlat only past ~20–50K vectors. (§11)
- **Personality is three composable layers:** immutable base-persona prose (`internal/agent/prompts/celine.md`) + tunable knobs (config struct) + a **global scheduled mood** (deterministic `moodForClock(now)`, optional `celine:mood` Redis override). Mood is global to Celine, never per-client. (§13)

## Hard constraints

- **Tiny server: ~300–400 MB RAM for the Go binary + RAG** (Postgres and Redis are already deployed separately). Set `GOMEMLIMIT`, cap the GORM/pgx pool, keep each tenant's vector table small.
- **Embeddings use Ollama** (`snowflake-arctic-embed:xs`, 384-dim) via `internal/graphe/ollama.go`. No external embedding API. The DB `memories.embedding` column is `vector(384)`.
- Secrets (`ANTHROPIC_API_KEY`, OAuth creds) live in env / `.env`, never committed.

## Conventions

- **Commit messages:** skip the technical description. Write **one evocative, incantation-style line** (like a tarot card title) — no conventional-commit prefixes, no file lists, no body. Keep the required `Co-Authored-By` trailer.

- **Git workflow:** only commit when explicitly asked. After committing, always `git push` immediately — no need to ask.

- **Go — OOP best practices:**
  - **Small, focused interfaces** — 1–3 methods. Define interfaces in the *consumer* package, not the producer. Accept interfaces, return concrete types.
  - **Composition over embedding chains** — embed for genuine "is-a" reuse; prefer explicit field delegation for "has-a".
  - **Constructor functions** — every exported type gets a `New*` function; no naked struct literals outside the package.
  - **Encapsulation** — unexported fields by default. Export only what callers genuinely need.
  - **Pointer vs value receivers** — pointer receivers for mutating or large structs; value receivers for small read-only types. Be consistent per type.
  - **Dependency injection through constructors** — no package-level globals or `init()` side effects for dependencies. Wire at `main()`.
