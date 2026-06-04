# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

**Greenfield — design-only.** The repo currently contains a single artifact: `architecture.md`. No code is scaffolded, there are no commits, and nothing builds yet. `architecture.md` is the **source of truth** for every decision below; read it first and keep it in sync when decisions change.

## What Celine is

A personal assistant ("JARVIS-style") with **Claude as the brain**, a **Go backend**, and a **React web UI**. One bot serving many clients, multi-tenant from day one. The whole system is one loop: *user talks → Claude thinks → tools act → Celine reports back.* Everything else is plumbing around that loop.

## Intended toolchain (per architecture.md — not yet scaffolded)

| Concern | Choice | Notes |
|---|---|---|
| Backend | Go, single binary | entrypoint `cmd/celine/main.go` |
| Brain SDK | `anthropic-sdk-go` | Messages API, tool use, streaming, prompt caching |
| Transport | **Connect RPC** (`connectrpc.com/connect`) | server-streaming `Chat` |
| Schema/codegen | **Protobuf + Buf** | `buf generate` turns `proto/celine/v1/celine.proto` → Go handlers + TS client |
| Frontend | React + Vite + TypeScript | in `web/`, consumes generated TS client |
| Durable store | **Postgres** (`pgx`) + **pgvector** | clients, conversations, memory, projects, vectors |
| Hot state | **Redis** | indexing job queue, caching, rate limiting (no session TTL) |
| Auth | **Google OAuth / OIDC only** | verify ID token; key all data on the `sub` claim |

**The `.proto` is the contract.** Both Go handlers and the TS client are generated from it via `buf generate` — change the proto, regenerate, never hand-edit generated code.

## Architecture you must understand before editing

These span multiple (future) files and are non-obvious:

- **Agent loop** (`internal/agent/`) — the heart we own. Recall context → call Claude with the cached system prefix + tools + history → if `tool_use` blocks, run tools and loop; else finalize. See architecture.md §3.2.
- **Layered system prompt (cached prefix), most-stable → least-stable:** base persona → response contract → tool defs → project prompt → project KB → curated profile (`memory.md`) → **cache breakpoint** → per-turn RAG recall → history. **Never put per-turn-dynamic content (RAG recall) above the breakpoint** — it would break caching every turn. (§13)
- **Memory is RAG, not sessions.** Every message (user *and* assistant — twice per turn) is embedded and indexed into `memory_index`, scoped by `owner_sub`; recall is a filtered vector search at the next turn's start. There is **no idle-timeout session** and no end-of-session distill job. Indexing is async via a **bounded Redis-list queue** to protect the tiny server. (§12)
- **Per-tenant isolation = a `WHERE owner_sub = $1` filter, not a separate store.** Use **pgvector**, not a dedicated vector DB (Qdrant etc.). At this scale: **brute-force exact scan, no HNSW**; add IVFFlat only past ~20–50K vectors. `owner_sub`/`project_id` are denormalized onto vector tables for join-free filtering. (§11)
- **Personality is three composable layers:** immutable base-persona prose (repo file, code-reviewed) + tunable knobs (config struct) + a **global scheduled mood** (deterministic `moodForClock(now)`, optional `celine:mood` Redis override). Mood is global to Celine, never per-client. (§13)
- **Projects** = custom system prompt + a knowledge base. Build **Strategy A** (concatenate all docs into the cached prefix) first; add **Strategy B** (pgvector RAG) only when a KB outgrows the context window. (§11)

## Hard constraints

- **Tiny server: ~300–400 MB RAM for the Go binary + RAG** (Postgres and Redis are already deployed separately). Set `GOMEMLIMIT`, cap the `pgx` pool, keep each tenant's vector table small. Default embedder is **`voyage-3-lite`** (512-dim) to keep vectors cheap. (§ resource discussion)
- Embeddings need a **third-party** provider (Anthropic has no embeddings API) — Voyage AI.
- Secrets (`ANTHROPIC_API_KEY`, OAuth creds, Voyage key) live in env / `.env`, never committed.

## Open / in-flight decisions

- **Response shape — segmentation (§14):** Celine replies as multiple short messages, not one wall of text. Direction under discussion leans toward **forced structured output** (`messages: string[]`) + a deterministic length-cap backstop — model keeps semantic/mood judgment, schema guarantees segmentation can't drift. Confirm the current state in architecture.md §14 before implementing.

## Conventions

- **Commit messages:** skip the technical description. Write **one evocative, incantation-style line** (like a tarot card title) — no conventional-commit prefixes, no file lists, no body. Keep the required `Co-Authored-By` trailer.
