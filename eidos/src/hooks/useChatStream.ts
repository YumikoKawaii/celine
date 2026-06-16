import { useCallback, useEffect, useRef, useState } from "react";
import { create } from "@bufbuild/protobuf";
import {
  AnamnesisRequestSchema,
  PempoRequestSchema,
  SigaoRequestSchema,
} from "../gen/celine/v1/celine_pb";
import { celine } from "../client";

// Celine's own prosopon id (seeded in 001_init.sql); any other id is the user.
const CELINE_PROSOPON_ID = 1n;

export interface Bubble {
  id: number;
  from: "user" | "celine";
  text: string;
}

let nextId = 1;

// How long the keyboard must stay quiet before we tell the server the user
// is done talking (Sigao). The server keeps a much longer debounce (~45s)
// purely as a fallback for clients that never signal.
const IDLE_MS = 5000;

export function useChatStream() {
  const [bubbles, setBubbles] = useState<Bubble[]>([]);
  const [typing, setTyping] = useState(false);
  const [busy, setBusy] = useState(false);

  // Messages sent since the server last started a flush (its typing event).
  // While > 0 the server holds an undrained inbox, so going quiet must Sigao.
  const queued = useRef(0);
  const idleTimer = useRef<number | null>(null);
  // Guards against double-fired submits (IME Enter, key repeat) that race
  // ahead of React clearing the draft.
  const lastSent = useRef<{ text: string; at: number } | null>(null);

  const armIdle = useCallback(() => {
    if (idleTimer.current !== null) clearTimeout(idleTimer.current);
    idleTimer.current = window.setTimeout(() => {
      idleTimer.current = null;
      if (queued.current > 0) {
        celine.sigao(create(SigaoRequestSchema, {})).catch(() => {});
      }
    }, IDLE_MS);
  }, []);

  // Call on every keystroke — postpones Sigao while the user is still typing.
  const noteTyping = useCallback(() => {
    if (queued.current > 0) armIdle();
  }, [armIdle]);

  // Load the conversation transcript once on mount so a returning user (or a
  // page reload) sees their history, not an empty pane. Runs independently of
  // the Parousia stream — the stream only carries *new* turns, so a reconnect
  // never re-fetches or duplicates what's already on screen.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await celine.anamnesis(create(AnamnesisRequestSchema, {}));
        if (cancelled) return;
        // Stored messages join all bubbles with \n\n (see agent.go). Split them
        // back out so history renders as individual chat bubbles, not one wall of text.
        setBubbles(
          res.messages.flatMap((m) => {
            const from: Bubble["from"] = m.prosoponId === CELINE_PROSOPON_ID ? "celine" : "user";
            return m.text
              .split("\n\n")
              .map((chunk) => ({ id: nextId++, from, text: chunk.trim() }))
              .filter((b) => b.text !== "");
          }),
        );
      } catch {
        // Empty/failed history is non-fatal — start with a blank pane.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // The Parousia stream carries the whole server-side session. If it ever ends
  // — StrictMode's effect cleanup, a hot reload, a dropped connection — the
  // server unregisters the session and subsequent Pempo/Sigao calls fail with
  // "no active session". So we reopen it automatically until the component
  // unmounts, with a small backoff to avoid a tight loop on a hard failure.
  useEffect(() => {
    const controller = new AbortController();
    let stopped = false;

    const run = async () => {
      let backoff = 250;
      while (!stopped) {
        try {
          for await (const ev of celine.parousia(
            {},
            { signal: controller.signal },
          )) {
            backoff = 250; // a delivered event means the stream is healthy
            const e = ev.event;
            switch (e.case) {
              case "typing":
                // Flush started — everything sent so far is being processed.
                queued.current = 0;
                setTyping(true);
                break;
              case "message":
                // Keep the typing dots on — bubbles stream in while the turn
                // is still generating; done/error clears them.
                setBubbles((b) => [
                  ...b,
                  { id: nextId++, from: "celine", text: e.value.text },
                ]);
                break;
              case "done":
                setTyping(false);
                setBusy(false);
                break;
              case "error":
                setTyping(false);
                setBusy(false);
                setBubbles((b) => [
                  ...b,
                  { id: nextId++, from: "celine", text: `⚠ ${e.value}` },
                ]);
                break;
            }
          }
          // Stream ended cleanly (server returned). Reopen unless unmounting.
        } catch {
          if (stopped || controller.signal.aborted) return;
          setBusy(false);
          setTyping(false);
        }

        if (stopped) return;
        // The session was just lost server-side; anything we thought was
        // queued there is gone, so reset the counter before reconnecting.
        queued.current = 0;
        await new Promise((r) => setTimeout(r, backoff));
        backoff = Math.min(backoff * 2, 5000);
      }
    };

    void run();

    return () => {
      stopped = true;
      controller.abort();
    };
  }, []);

  const send = useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      if (!trimmed) return;
      const now = Date.now();
      if (
        lastSent.current &&
        lastSent.current.text === trimmed &&
        now - lastSent.current.at < 400
      ) {
        return;
      }
      lastSent.current = { text: trimmed, at: now };

      setBubbles((b) => [...b, { id: nextId++, from: "user", text: trimmed }]);

      try {
        await celine.pempo(create(PempoRequestSchema, { text: trimmed }));
        queued.current += 1;
        armIdle();
      } catch (err) {
        setBubbles((b) => [
          ...b,
          { id: nextId++, from: "celine", text: `⚠ ${String(err)}` },
        ]);
      }
    },
    [armIdle],
  );

  return { bubbles, typing, busy, send, noteTyping };
}
