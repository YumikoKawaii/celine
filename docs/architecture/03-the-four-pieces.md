## 3. The four pieces that matter

### 3.1 The brain — Claude via the Anthropic API
- Use **`anthropic-sdk-go`** (official SDK) with the Messages API.
- **Tool use (function calling):** Claude decides *when* to reach for a tool; the Go code executes it and feeds the result back.
- **Prompt caching** on the system prompt + tool definitions, so we don't pay full price every turn.
- **Streaming** responses so the UI feels alive.
- Default model: latest Claude (e.g. `claude-opus-4-8` for quality, `claude-sonnet-4-6` for cheaper/faster turns). Make it configurable.

### 3.2 The agent loop — our code (the heart we own)

```
  LaleoRequest {text}                   sub ← AuthInterceptor (Hermes JWT)
       │                                      │
       └──────────────┬───────────────────────┘
                      ▼
         ┌────────────────────────────────────────────────┐
         │               agent.Chat()                     │
         │                                                │
         │  ① convs.GetOrCreate(prosopon)                 │
         │  ② msgs.List → hist[]                          │
         │  ③ tier-1 recall: clients.memory_md            │
         │     always in cached system prefix             │
         │  ④ tier-2 recall: embed query → vector search  │
         │     inject hint only if score > threshold      │
         │  ⑤ msgs.Save(user) + Enqueue index job         │
         └────────────────────┬───────────────────────────┘
                              │  system + tools + hist[]
                              ▼
                      ┌───────────────┐
                      │  Claude API   │◄────────────────────┐
                      └───────┬───────┘                     │
                              │                             │
             ┌────────────────┴──────────────┐             │
             │                               │             │
       end_turn                          tool_use          │
             │                               │             │
             │                               ▼             │
             │                    ergon/registry            │
             │                    Execute(name, input)      │
             │                               │             │
             │                    ┌──────────┴──────────┐  │
             │                    │ tool_result          │  │
             │                    │ (or error)           │  │
             │                    └──────────────────────┘  │
             │                    append to hist[], loop ───┘
             │
             ▼
  Turn.Bubbles → LaleoEvent{Message} (each sent whole, §14)
  msgs.Create(assistant) + Enqueue index job
  send LaleoEvent{Done}
```

Steps ③–④ are the read path (§12.5); step ⑤ and the final enqueue are the write path (§12.2–12.3). The tool loop (step tool_use → ergon → loop) is Step 3 of the build order (§8).

### 3.3 Tools — what makes it an *assistant*, not just chat
Each tool = a function + a JSON schema description. Start tiny, grow later.

| Tool                | Purpose                                  | Milestone |
|---------------------|------------------------------------------|-----------|
| `web_search`        | Current info from the internet           | early     |
| `remember` / `recall` | Persistent long-term memory — **agentic, on-demand; in-process tool, not MCP** (§12.5) | early |
| `manage_reminders`  | Todos / alarms                           | mid       |
| `read_files`        | Read local files (sandboxed)             | mid       |
| `run_command`       | Touch the machine (sandbox carefully!)   | later     |
| calendar / email / Spotify / smart home | Quality-of-life          | later     |

### 3.4 Memory — so she's *yours* (per-client)
Celine is **one bot serving many clients**; memory is keyed per client (Google `sub`). Memory is **RAG-based**: every message is embedded and indexed *as it happens*, then recalled by filtered vector search. There is no session-buffer-then-distill step (see §12).
- **Indexed memory (RAG):** every user and assistant message is embedded and stored in pgvector, scoped by `owner_sub`. Recall = top-K filtered vector search over *this client's* rows. Indexing is per-turn and queue-based (see §12).
- **Conversation transcripts:** per-conversation history in Postgres — *what we said.* Replayed as message history for the active thread.
- **Curated profile (`memory.md`, optional):** a small always-loaded summary of *who this person is*, injected into the cached prefix so core facts are present even without a query match. Decoupled from sessions — updated opportunistically, not at any session end. Optional; RAG recall covers most needs.


