## 8. Build order (each step is demoable)

1. **Plumbing** — define `celine.proto`, run `buf generate`, stand up the Connect server with a `Chat` RPC that streams hardcoded `Message` bubbles (with `Typing` beats); React chat window consumes the generated client and renders the bubbles. *No Claude yet — prove the typed pipe.*
2. **Brain** — wire `anthropic-sdk-go`, real streaming chat. Celine talks.
3. **Tool loop** — add the registry + one tool (`web_search`), handle the tool-use round-trip, render tool cards in the UI.
4. **Memory** — embed every message into pgvector per client (queue-based, twice per turn), recall via filtered vector search. Optional `clients.memory_md` profile in the cached prefix. She remembers each person.
5. **Personality** — the layered persona that makes her *Celine*: base prose + tunable knobs + scheduled mood (see §13).
6. **Grow** — reminders, files, calendar, then voice / auth / deploy whenever.


