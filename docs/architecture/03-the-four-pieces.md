## 3. The four pieces that matter

### 3.1 The brain — Claude via the Anthropic API
- Use **`anthropic-sdk-go`** (official SDK) with the Messages API.
- **Tool use (function calling):** Claude decides *when* to reach for a tool; the Go code executes it and feeds the result back.
- **Prompt caching** on the system prompt + tool definitions, so we don't pay full price every turn.
- **Streaming** responses so the UI feels alive.
- Default model: latest Claude (e.g. `claude-opus-4-8` for quality, `claude-sonnet-4-6` for cheaper/faster turns). Make it configurable.

### 3.2 The agent loop — our code (the heart we own)
```
1. Receive user message via the Chat server-streaming RPC (client identified by Google sub)
2. Recall context (tiered, §12.5): curated profile (always) + a thresholded memory hint
   (top-K vector search, injected only above a similarity cutoff) + recent transcript;
   enqueue an index job for the incoming user message. Deeper memory = the `recall` tool.
3. Call Claude with: system prompt + recalled memory + tools + history (prompt-cached)
4. Stream ChatEvent messages back to the browser as they arrive
5. If Claude emits tool_use blocks:
      run each tool → append tool_result → go back to step 3
   else:
      final answer — persist to Postgres, enqueue an index job for the assistant message,
      close the stream
```

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


