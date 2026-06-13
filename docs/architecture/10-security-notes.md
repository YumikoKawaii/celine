## 10. Security notes

- Keep `ANTHROPIC_API_KEY` in env, never in the repo.
- `run_command` / file tools must be **sandboxed** (allowlist, working-dir jail, confirmation step) before they touch anything real.
- If ever exposed beyond localhost: add auth + rate limiting.
- **Access whitelist.** Only listed emails may obtain a token. The gate lives at
  `Metabole` (the OAuth exchange — first point the verified email is known), so
  an unlisted account never gets a JWT and never reaches the agent loop or spends
  Anthropic usage. Config: `CELINE_WHITELIST` → a YAML file (`emails:` list,
  case-insensitive); see `basis/whitelist.example.yaml`. Unset/empty = **open
  access** (dev only). A configured-but-unreadable file is a hard startup error —
  it never fails open. (`internal/hermes/whitelist.go`)
- **No raw errors to clients.** Errors from external systems (Claude, Ollama,
  Postgres) are logged server-side and replaced with a generic message before
  streaming: a failed turn sends a fixed `error` event, a failed tool surfaces
  to the client as "tool unavailable" (while Claude still gets the real error so
  it can report the failure honestly). A single failed turn no longer drops the
  `Parousia` stream — the session stays open. (`internal/rpc/celine.go`,
  `internal/agent/agent.go`)


