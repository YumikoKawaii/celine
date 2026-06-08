import { useCallback, useEffect, useRef, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { PempoRequestSchema } from "../gen/celine/v1/celine_pb";
import { celine } from "../client";

export interface Bubble {
  id: number;
  from: "user" | "celine";
  text: string;
}

let nextId = 1;

export function useChatStream() {
  const [bubbles, setBubbles] = useState<Bubble[]>([]);
  const [typing, setTyping] = useState(false);
  const [busy, setBusy] = useState(false);
  const conversationId = useRef("");

  // Open the Parousia stream once on mount and keep it alive for the session.
  // All agent events (typing indicators, bubbles, tool activity, done) arrive here.
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
              setTyping(true);
              break;
            case "message":
              setTyping(false);
              setBubbles((b) => [
                ...b,
                { id: nextId++, from: "celine", text: e.value.text },
              ]);
              break;
            case "done":
              conversationId.current = e.value.conversationId;
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

  // Pempo is a fire-and-forget unary call. The response is just an ack —
  // the actual reply flows back through the persistent Parousia stream above.
  // busy stays true until the Done event arrives through that stream.
  const send = useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      if (!trimmed || busy) return;

      setBusy(true);
      setBubbles((b) => [...b, { id: nextId++, from: "user", text: trimmed }]);

      try {
        await celine.pempo(
          create(PempoRequestSchema, {
            conversationId: conversationId.current,
            text: trimmed,
          }),
        );
      } catch (err) {
        setBubbles((b) => [
          ...b,
          { id: nextId++, from: "celine", text: `⚠ ${String(err)}` },
        ]);
        setBusy(false);
      }
    },
    [busy],
  );

  return { bubbles, typing, busy, send };
}
