## 13. Personality (Celine's characteristic)

Celine's character is **the system prompt**, but structured as composable layers, never one prose blob. This keeps the "soul" stable and code-reviewed while letting behaviour flex per client, per project, and **per mood on a schedule** — without rewriting prose. Configurability runs along two axes: **depth** (what you can change — bounded dials → guarded free text → the code-only soul) and **scope** (who it's for — global → project → client → mood).

```
   invariant boundaries (§13.1)         ✗ nothing below can override these — the floor
        ▲ under everything
base persona (immutable prose)          ← the soul, version-controlled, code-reviewed
        ▲ placeholders filled by
   knobs (enums) + archetype macro      ← Depth 1: bounded dials; global → project → client
        ▲ optionally extended by
   free-text personalization            ← Depth 2: per-client / per-project prose, guarded (capped, below base)
        ▲ overlaid by
   mood (scheduled overlay)             ← transient: nudges knobs + adds a flavor line
        ↓
   final system-prompt prefix  (cached — see §3.2 / §11.3 assembly)
```

### 13.1 Base persona — the character sheet

A version-controlled prose file (e.g. `internal/agent/prompts/celine.md`, embedded via Go `embed`). Edited rarely, code-reviewed like code. It contains `{{placeholders}}` the knobs fill in. Sections:

- **Identity** — who she is, her name, her role as a personal assistant.
- **Voice & tone** — how she speaks at rest (the default the knobs modulate).
- **Relationship to the user** — JARVIS-style: loyal, proactive, anticipates needs, addresses the client by name (from `clients.display_name`).
- **Principles & boundaries** — *honesty: never fabricate; say "I don't know"*; report tool failures plainly; safety / refusal stance. These are **invariant** — knobs and moods can't override them.
- **Signature quirks** — the small consistent touches that make her *her*.

### 13.2 Knobs — tunable dials

A small struct of **enumerated** dials rendered *into* the prose, so behaviour tunes without prompt surgery. Enums (not free text) are the point: a client can set any of these and still can't break coherence or jailbreak the persona — a dial only has valid positions.

```go
type Persona struct {
    // Voice
    Verbosity   string // "concise" | "balanced" | "detailed"
    Formality   string // "casual" | "neutral" | "formal"
    Warmth      string // "reserved" | "neutral" | "warm"
    Humor       string // "none" | "dry" | "playful"
    Emoji       bool
    Language    string // "" = match the user
    // Manner
    Proactivity string // "passive" | "responsive" | "anticipatory"  — how JARVIS-like
    Candor      string // "diplomatic" | "balanced" | "blunt"
    Curiosity   string // "none" | "light" | "inquisitive"           — asks follow-ups?
    Opinionated string // "neutral" | "leans" | "takes-a-stance"
    Address     string // "name" | "nickname" | "honorific"          — how she calls the client
    // Action
    Initiative  string // "confirm-first" | "act-then-report" | "autonomous" — tool boldness, still under §10 safety
    ToolNarrate bool   // announce tool use, or stay silent
}
```

**Archetype macro — a preset of presets.** Rather than set a dozen dials, a client or project picks one named archetype that expands to a full `Persona`; explicit knobs then override individual fields on top.

```go
// Archetypes expand to a Persona, used as the *starting point* before the override chain.
var Archetypes = map[string]Persona{
    "butler":        {Formality: "formal", Warmth: "warm", Proactivity: "anticipatory", Candor: "diplomatic", Initiative: "confirm-first"},
    "hacker-friend": {Formality: "casual", Humor: "dry",  Candor: "blunt",  Emoji: true, Initiative: "act-then-report"},
    "coach":         {Warmth: "warm", Curiosity: "inquisitive", Proactivity: "anticipatory", Candor: "balanced", Opinionated: "takes-a-stance"},
}
```

**Resolution order (later wins):** `archetype` expands first → **global defaults → per-project → per-client (`preferences`) → mood overlay**. Any explicit dial beats the archetype's implied value at the same-or-narrower scope. Pair with config-level `temperature` (lower = steadier persona) and `model`.

### 13.2.1 Free-text personalization (the expressive layer, Depth 2)

When dials aren't enough, two **free-text** fields let a human (or Celine herself) speak in prose:

- **Per-client** `clients.persona_note` — *"how should Celine treat you?"* (like claude.ai custom instructions). Set by the client, or learned and written by Celine via the `remember` tool over time.
- **Per-project** `projects.system_prompt` — the project's persona/instructions (already in the data model, §11.2).

Both inject into the **cached prefix** (stable per client/project → cheap, §13.4). Because free text is unbounded, it is **guarded**:

- it sits **below** the immutable base persona, so on any conflict the base persona and the **invariant boundaries (§13.1) win** — free text can never loosen honesty/safety;
- a **length cap** keeps it from blowing the prefix / RAM budget;
- it *adds colour*, it doesn't *replace* the soul.

This is the deepest **runtime-configurable** layer. Deeper still is the base persona itself (§13.1) — but that's code-reviewed prose, changed by PR, never runtime config.

### 13.3 Mood — scheduled personality overlay

A **mood** is a named preset that (a) overrides some knobs and (b) adds one short *flavor line* to the prompt. Mood is global to Celine — *she* has a mood — not per-client.

```go
var Moods = map[string]Persona{ /* knob overrides per mood */ }
var MoodFlavor = map[string]string{
    "focused":  "Crisp and to the point right now; skip the small talk.",
    "cheerful": "Bright and encouraging this morning.",
    "mellow":   "Calm, unhurried, low-key — it's late.",
    "playful":  "Feeling playful; a little extra wit is welcome.",
}
```

**Resolution at assembly time:**

```
mood := redis.Get("celine:mood")        // optional override (scheduled or manual)
if mood == "" { mood = moodForClock(now) }   // deterministic default by time-of-day
p := defaults.merge(projectPrefs).merge(clientPrefs).merge(Moods[mood])
prefix := render(basePersona, p) + "\n" + MoodFlavor[mood]
```

- **Default = a pure function of the clock** (`moodForClock`) — morning→cheerful, work hours→focused, late→mellow, weekend→playful. **No background job needed**; it's computed per request.
- **Override = an optional `celine:mood` key in Redis**, set by a tiny cron or by hand (e.g. a celebratory mood on a birthday, with a TTL). This is the *only* place the "on a schedule" mechanism lives — and it's optional sugar on top of the deterministic default.

### 13.4 Caching

The rendered persona sits in the **cached prefix** (layers above the §12 / §11.3 breakpoint). Knobs and mood change slowly — at most a handful of times a day — so a mood flip just invalidates the prefix **once** (one full-price turn, then re-cached for the next window). Cheap. Never put anything per-turn-dynamic (RAG recall) into this block.

### 13.5 Where each piece lives

| Piece | Lives in | Cadence |
|---|---|---|
| Base persona prose | repo file (embedded) | edited rarely, code-reviewed |
| Invariant boundaries | base persona (§13.1) | never overridable |
| Global knob defaults | config | deploy-time |
| Archetype presets | `Archetypes` map (code) | edited rarely |
| Per-project knobs + persona | `projects.preferences` + `projects.system_prompt` | per project |
| Per-client knobs | `clients.preferences` | per client |
| Per-client free-text | `clients.persona_note` (set by client or learned) | per client / opportunistic |
| Current mood | `moodForClock(now)`, override in `celine:mood` | minutes–hours |


