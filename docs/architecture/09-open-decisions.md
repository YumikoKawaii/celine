## 9. Open decisions (defaults chosen, easy to revisit)

- **Connect server-streaming RPC** for chat (typed, schema-first). Bidi streaming later if we add voice / interrupts.
- **Postgres** (durable) + **Redis** (hot session state) — both already on the target server.
- **Auth: Google OIDC only, client-side redirect.** The React app runs the Google sign-in flow (Google Identity Services) and obtains the ID token — there is **no server-side redirect and no `/callback` endpoint.** The client sends the ID token as an `Authorization: Bearer` header on every RPC; a server interceptor verifies it (Google JWKS), extracts the `sub` claim, and puts the caller on the context. `GetCurrentUser` is the one auth-flow RPC (verify + upsert client + return profile), called once after the redirect. Sign-out is client-side; **no server session store** (consistent with §12's no-session-TTL). Multi-tenant from day one — all data keyed on `sub`.
- **Per-client memory:** RAG over indexed messages (pgvector, filtered by `owner_sub`), indexed per-turn via a bounded queue. Optional curated `memory.md` profile, decoupled from sessions.
- **Recall is tiered, not an always-on prepend (§12.5):** curated profile (cached) + thresholded auto-hint + an **agentic `recall` tool** for on-demand "fetch more." `recall` is an **in-process custom tool, not MCP.**
- **No idle-timeout sessions:** indexing runs continuously per request, so the previously-deferred end-of-session scheduling problem is gone (see §12).
- **API key + OAuth creds in env / `.env`**, never committed.
- **Model:** configurable; default to latest Claude.


