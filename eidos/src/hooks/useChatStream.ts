import { useCallback, useEffect, useRef, useState } from "react";
import { create } from "@bufbuild/protobuf";
import {
  PempoRequestSchema,
  SigaoRequestSchema,
} from "../gen/celine/v1/celine_pb";
import { celine } from "../client";

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

  useEffect(() => {
    const controller = new AbortController();

    (async () => {
      try {
        for await (const ev of celine.parousia(
          {},
          { signal: controller.signal },
        )) {
          const e = ev.event;
          switch (e.case) {
            case "typing":
              // Flush started — everything sent so far is now being processed.
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
      } catch (err) {
        if (!controller.signal.aborted) {
          setBusy(false);
          setTyping(false);
        }
      }
    })();

    return () => controller.abort();
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
