## 10. Security notes

- Keep `ANTHROPIC_API_KEY` in env, never in the repo.
- `run_command` / file tools must be **sandboxed** (allowlist, working-dir jail, confirmation step) before they touch anything real.
- If ever exposed beyond localhost: add auth + rate limiting.


