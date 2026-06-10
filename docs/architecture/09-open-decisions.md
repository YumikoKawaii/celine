## 9. Open decisions (defaults chosen, easy to revisit)

- **Connect server-streaming RPC** for chat (typed, schema-first). Bidi streaming later if we add voice / interrupts.
- **Postgres** (durable) + **Redis** (hot session state) — both already on the target server.
- **Auth: Google OIDC, server-side code exchange, server-issued JWT (§7.1).** The `Hermes` service runs the flow: `Eisodos` returns the Google sign-in URL + `state`; the browser redirects and hands the `code` back via `Metabole`, where the **server** (holding the client secret) swaps it with Google, upserts the `prosopons` row, and returns its **own** HS256 JWT (claims embed `sub`, `prosopon_id`, `conversation_id`). The client sends that JWT as `Authorization: Bearer` on every RPC; a server interceptor verifies it and puts the claims on the context. `Gnorizo` resolves the current caller; `Exodos` (sign-out) is client-side — **no server session store** (consistent with §12's no-session-TTL). Unset `CELINE_JWT_SECRET` = dev mode (anon). Multi-tenant from day one — all data keyed on `sub`.
- **Per-client memory:** RAG over indexed messages (pgvector, filtered by `owner_sub`), indexed per-turn via a bounded queue. Optional curated `memory.md` profile, decoupled from sessions.
- **Recall is tiered, not an always-on prepend (§12.5):** curated profile (cached) + thresholded auto-hint + an **agentic `recall` tool** for on-demand "fetch more." `recall` is an **in-process custom tool, not MCP.**
- **No idle-timeout sessions:** indexing runs continuously per request, so the previously-deferred end-of-session scheduling problem is gone (see §12).
- **API key + OAuth creds in env / `.env`**, never committed.
- **Model:** configurable; default to latest Claude.


