## 11. Projects & Knowledge Base (claude.ai "Projects" equivalent)

**Equation:** a Project = a **custom system prompt** + a **knowledge base** (a pile of docs). When you chat "inside" a project, Celine answers with that persona and that context loaded.

### 11.1 Two retrieval strategies (start simple, grow into the second)

| Strategy | When | How |
|---|---|---|
| **A. Stuff-it-all** (MVP) | KB fits in context (~up to tens of K tokens) | Concatenate every doc into one **prompt-cached** context block, prepended to each request. Dead simple, no embeddings. |
| **B. Retrieval (RAG)** | KB too big to fit | Chunk docs → embed → vector search → inject only the **top-K relevant** chunks per turn. |

Build **A first** — with prompt caching it's cheap and covers most personal projects. Add **B** only when a KB outgrows the context window. The chat code barely changes: both strategies just produce "the context block to prepend."

> ⚠️ **Embeddings note:** Anthropic does **not** provide an embeddings API. For strategy B, use a third-party embedder — Anthropic recommends **Voyage AI**; default to **`voyage-3-lite`** here (512-dim → 2 KB/vector, ~3× cheaper, recall loss irrelevant at personal scale). OpenAI embeddings or a local model also work. Vector store: **Postgres + `pgvector`** (already have Postgres), or plain cosine in Go at small scale.

> 🧭 **Why not a dedicated vector DB (Qdrant, etc.)?** Overkill at this scale, and costly on a tiny box — another process with its own RAM, deployment, and backups. `pgvector` reuses the Postgres we already run, keeps vectors *next to* their rows (filter + JOIN in one SQL query), and stays in sync for free. At personal scale (≪ 1M vectors) use **brute-force exact scan, no HNSW** — the index's resident-RAM appetite is the one thing the box can't spare; add **IVFFlat** only past ~20–50K vectors. Reach for a dedicated store only at millions of vectors / high QPS / sharding. **Per-tenant isolation = a `WHERE owner_sub = $1` filter, not a separate "collection."**

### 11.2 Data model (Postgres) — everything keyed per client

```sql
clients(
  sub TEXT PRIMARY KEY,          -- Google `sub` claim (stable identity)
  email, display_name,
  memory_md TEXT,                -- the per-client durable profile (agent-maintained)
  preferences JSONB,             -- per-client persona knobs + archetype (§13.2)
  persona_note TEXT,             -- free-text "how to treat you" (§13.2.1; capped, guarded)
  created_at, updated_at
)

projects(
  id, owner_sub REFERENCES clients(sub),  -- projects are per-client
  name, system_prompt,                    -- system_prompt = the project's free-text persona (§13.2.1)
  preferences JSONB,                      -- per-project persona knobs + archetype (§13.2)
  created_at
)

documents(
  id, project_id, name, content, token_count, created_at
)

-- strategy B (project KB RAG): tenant key denormalized onto chunks for fast filtered search
chunks(
  id,
  document_id REFERENCES documents(id),
  owner_sub  TEXT NOT NULL,        -- denormalized → filter without a join
  project_id BIGINT,               -- denormalized → project-scoped search
  ordinal, content,
  embedding  vector(512)           -- voyage-3-lite
)
CREATE INDEX ON chunks (owner_sub, project_id);   -- btree, cheap; narrows the scan first

-- per-client memory RAG: every message embedded + indexed (see §12)
memory_index(
  id,
  owner_sub  TEXT NOT NULL REFERENCES clients(sub),  -- tenant isolation
  message_id REFERENCES messages(id),
  role,                            -- user | assistant
  content,                         -- the text that was embedded
  embedding  vector(512),
  created_at
)
CREATE INDEX ON memory_index (owner_sub);

conversations(
  id, owner_sub REFERENCES clients(sub),
  project_id NULLABLE,           -- nullable = no project
  title, created_at
)

messages(
  id, conversation_id, role, content, created_at
)
```

**Per-tenant isolation = a column + a `WHERE`, not a separate store.** Both RAG stores carry a denormalized `owner_sub`; "search only this client's data" is one filtered query:

```sql
-- project KB recall (scoped to a client + project)
SELECT content FROM chunks
WHERE owner_sub = $1 AND project_id = $2
ORDER BY embedding <=> $3 LIMIT 8;

-- per-client memory recall
SELECT content FROM memory_index
WHERE owner_sub = $1
ORDER BY embedding <=> $2 LIMIT 8;
```

With brute-force scan (no ANN index) the `WHERE` is applied *during* the scan — so it's both **exact** (no filtered-ANN recall gotcha) and **faster** (cosine runs only over that client's rows). The btree index narrows to those rows first.

### 11.3 How a project-scoped chat is assembled

```
1. Chat request arrives with a project_id
2. Load project.system_prompt
3. Build the KB context block:
     - Strategy A: concatenate all project documents
     - Strategy B: embed the user's message → top-K chunks
4. Send to Claude:
     system = [ project.system_prompt,
                {kb_block, cache_control: ephemeral} ]   ← cached prefix
     messages = conversation history + new user message
5. Stream reply back (same agent loop as always — tools still work)
```

The KB block goes in the **cached** part of the system prompt, so re-sending it each turn is cheap (you pay full price once, ~90% less on cache hits within the TTL).

### 11.4 RPC additions (`celine.proto`)

```proto
rpc CreateProject(CreateProjectRequest) returns (Project);
rpc ListProjects(ListProjectsRequest)   returns (ListProjectsResponse);
rpc UpdateProject(UpdateProjectRequest) returns (Project);   // edit system prompt
rpc AddDocument(AddDocumentRequest)     returns (Document);   // upload to KB
rpc ListDocuments(ListDocumentsRequest) returns (ListDocumentsResponse);
rpc DeleteDocument(DeleteDocumentRequest) returns (DeleteDocumentResponse);

// ChatRequest gains an optional project_id (already supports conversation_id).
```

### 11.5 Where it lands in the build order

Slots in as a milestone after **Memory (4)** and before/around **Personality (5)** — projects are essentially "scoped personalities + scoped memory," so they reuse both the system-prompt and the Postgres plumbing already built.

1. `projects` + `documents` tables, `CreateProject` / `AddDocument` RPCs, minimal UI (project list + a textarea for the system prompt + file upload).
2. Wire project context into the chat assembly (**Strategy A**, prompt-cached).
3. *(Later)* Add `chunks` + embeddings + top-K retrieval (**Strategy B**) once a KB gets large.


