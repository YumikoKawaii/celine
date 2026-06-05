# Celine — Architecture

A personal assistant ("JARVIS-style") with **Claude as the brain**, a **Go backend**, and a **React web UI**.

These files are the **source of truth** for the design — one small file per section.
The `§N` shorthand used throughout the code and `CLAUDE.md` maps to the numbered
files below (e.g. `§3.2` → [`03-the-four-pieces.md`](03-the-four-pieces.md), `§14` →
[`14-response-shape.md`](14-response-shape.md)).

## Sections

| § | Section | File |
|---|---|---|
| 1 | Core idea — the one loop | [01-core-idea.md](01-core-idea.md) |
| 2 | High-level architecture (diagram) | [02-high-level-architecture.md](02-high-level-architecture.md) |
| 3 | The four pieces that matter — brain · agent loop · tools · memory | [03-the-four-pieces.md](03-the-four-pieces.md) |
| 4 | Tech choices for this stack | [04-tech-choices.md](04-tech-choices.md) |
| 5 | Repo layout (+ 5.1 current implementation, as built) | [05-repo-layout.md](05-repo-layout.md) |
| 6 | The Tool interface | [06-tool-interface.md](06-tool-interface.md) |
| 7 | RPC API (Connect, initial) | [07-rpc-api.md](07-rpc-api.md) |
| 8 | Build order (each step is demoable) | [08-build-order.md](08-build-order.md) |
| 9 | Open decisions | [09-open-decisions.md](09-open-decisions.md) |
| 10 | Security notes | [10-security-notes.md](10-security-notes.md) |
| 11 | Projects & Knowledge Base | [11-projects-and-kb.md](11-projects-and-kb.md) |
| 12 | Memory indexing (continuous, queue-based) | [12-memory-indexing.md](12-memory-indexing.md) |
| 13 | Personality (Celine's characteristic) | [13-personality.md](13-personality.md) |
| 14 | Response shape — chat-bubble delivery | [14-response-shape.md](14-response-shape.md) |

## Where the build is

Step 2 ("brain") is done — see [§5.1](05-repo-layout.md) for the as-built backend
diagram and [§8](08-build-order.md) for the full roadmap. `CLAUDE.md` tracks the
per-step status.

> **Editing convention:** keep each section self-contained in its own file. When a
> decision changes, update its section file *and* any cross-referencing sections.
> If you add a new top-level section, add a row here.
