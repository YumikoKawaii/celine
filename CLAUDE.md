# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

**Scaffolded — first vertical slice live (Step 1, plumbing).** `architecture.md` (the **source of truth** — read it first, keep it in sync) is now backed by a working repo: a typed Connect stream runs end to end (proto → Go server → React UI) with a **hardcoded, paced, multi-bubble reply**. No brain, no DB yet. The next steps wire in Claude (the agent loop) and Postgres/Redis.

## What Celine is

A personal assistant ("JARVIS-style") with **Claude as the brain**, a **Go backend**, and a **React web UI**. One bot serving many clients, multi-tenant from day one. The whole system is one loop: *user talks → Claude thinks → tools act → Celine reports back.* Everything else is plumbing around that loop.

## Repo layout

Three sibling modules, each named for its role (the names are deliberate — `basis` = the ground it stands on, `eidos` = the visible form, `proto` = the contract):

| Dir | What it is | Stack |
|---|---|---|
| `proto/` | **The contract.** `celine/v1/celine.proto` → Go + TS via Buf. | Protobuf, Buf v2 |
| `basis/` | **Go backend**, single binary. Module `github.com/YumikoKawaii/celine/basis`, entrypoint `cmd/celine/main.go`. Generated code in `gen/`, hand-written in `internal/`. | Go 1.25, Connect RPC, h2c |
| `eidos/` | **React web UI.** Consumes the generated TS client. Generated code in `src/gen/`. | React 19 + Vite 7 + TS, bun, Connect-ES v2 |

**The `.proto` is the contract.** Both the Go handlers (`basis/gen/`) and the TS client (`eidos/src/gen/`) are generated from it — change the proto, regenerate, **never hand-edit generated code**.

## Build & codegen

```bash
# Regenerate Go + TS from the proto (run from proto/; out paths climb to siblings)
cd proto && buf generate

# Backend
cd basis && go build ./... && go run ./cmd/celine   # serves on :8080 (CELINE_ADDR to override)

# Frontend
cd eidos && bun install && bun dev                   # Vite dev server
cd eidos && bun run typecheck                         # tsc --noEmit
cd eidos && bun run build                             # tsc -b && vite build
```

`buf.yaml` lints `STANDARD` but excepts `RPC_RESPONSE_STANDARD_NAME` — `Chat` streams a typed `ChatEvent` oneof, not a `ChatResponse` (intentional, §7).

## Intended toolchain (per architecture.md)

| Concern | Choice | Status |
|---|---|---|
| Backend | Go, single binary | ✅ `basis/cmd/celine/main.go` |
| Transport | **Connect RPC** (`connectrpc.com/connect`), server-streaming `Chat` | ✅ wired (h2c + dev-CORS) |
| Schema/codegen | **Protobuf + Buf** | ✅ `proto/`, codegen working |
| Frontend | React + Vite + TypeScript, in `eidos/` | ✅ consumes generated client |
| Brain SDK | `anthropic-sdk-go` (Messages API, tool use, streaming, prompt caching) | ⏳ not yet |
| Durable store | **Postgres** (`pgx`) + **pgvector** | ⏳ not yet |
| Hot state | **Redis** (indexing job queue, caching, rate limiting; no session TTL) | ⏳ not yet |
| Auth | **Google OAuth / OIDC only** — verify ID token; key all data on the `sub` claim | ⏳ not yet |

## Architecture you must understand before editing

These span multiple files (some still future) and are non-obvious:

- **Agent loop** (`basis/internal/agent/`, not yet built) — the heart we own. Recall context → call Claude with the cached system prefix + tools + history → if `tool_use` blocks, run tools and loop; else finalize. See architecture.md §3.2.
- **Response shape — segmentation + pacing (§14, decided):** Celine replies as a **sequence of short bubbles**, not one wall of text. **Model side** segments by writing a blank line (`\n\n`) between thoughts (a §13 response-contract rule); **backend side** (`basis/internal/agent/stream.go`, future) is a code-fence-aware splitter + pacer: *typing beat → delay → whole bubble → pause*, firing the first bubble at the first `\n\n`. One generation, no extra token cost. The current `internal/rpc/chat_service.go` is this pacer in miniature over hardcoded bubbles. (The earlier "forced structured output `messages: string[]`" idea was **rejected** in favor of the blank-line delimiter.)
- **Layered system prompt (cached prefix), most-stable → least-stable:** base persona → response contract → tool defs → project prompt → project KB → curated profile (`memory.md`) → **cache breakpoint** → per-turn RAG recall → history. **Never put per-turn-dynamic content (RAG recall) above the breakpoint** — it would break caching every turn. (§13)
- **Memory is RAG, not sessions.** Every message (user *and* assistant — twice per turn) is embedded and indexed into `memory_index`, scoped by `owner_sub`; recall is a filtered vector search at the next turn's start. There is **no idle-timeout session** and no end-of-session distill job. Indexing is async via a **bounded Redis-list queue** to protect the tiny server. (§12)
- **Per-tenant isolation = a `WHERE owner_sub = $1` filter, not a separate store.** Use **pgvector**, not a dedicated vector DB (Qdrant etc.). At this scale: **brute-force exact scan, no HNSW**; add IVFFlat only past ~20–50K vectors. `owner_sub`/`project_id` are denormalized onto vector tables for join-free filtering. (§11)
- **Personality is three composable layers:** immutable base-persona prose (repo file, code-reviewed) + tunable knobs (config struct) + a **global scheduled mood** (deterministic `moodForClock(now)`, optional `celine:mood` Redis override). Mood is global to Celine, never per-client, and shapes pacing too (§14.4). (§13)
- **Projects** = custom system prompt + a knowledge base. Build **Strategy A** (concatenate all docs into the cached prefix) first; add **Strategy B** (pgvector RAG) only when a KB outgrows the context window. (§11)

## Hard constraints

- **Tiny server: ~300–400 MB RAM for the Go binary + RAG** (Postgres and Redis are already deployed separately). Set `GOMEMLIMIT`, cap the `pgx` pool, keep each tenant's vector table small. Default embedder is **`voyage-3-lite`** (512-dim) to keep vectors cheap. (§ resource discussion)
- Embeddings need a **third-party** provider (Anthropic has no embeddings API) — Voyage AI.
- Secrets (`ANTHROPIC_API_KEY`, OAuth creds, Voyage key) live in env / `.env`, never committed.

## Conventions

- **Commit messages:** skip the technical description. Write **one evocative, incantation-style line** (like a tarot card title) — no conventional-commit prefixes, no file lists, no body. Keep the required `Co-Authored-By` trailer.
