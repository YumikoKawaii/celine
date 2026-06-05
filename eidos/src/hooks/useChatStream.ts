import { useCallback, useRef, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { ChatRequestSchema } from "../gen/celine/v1/celine_pb";
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

  const send = useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      if (!trimmed || busy) return;

      setBusy(true);
      setBubbles((b) => [...b, { id: nextId++, from: "user", text: trimmed }]);

      const req = create(ChatRequestSchema, {
        conversationId: conversationId.current,
        text: trimmed,
      });

      try {
        for await (const ev of celine.chat(req)) {
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
              break;
            case "error":
              setBubbles((b) => [
                ...b,
                { id: nextId++, from: "celine", text: `⚠ ${e.value}` },
              ]);
              break;
          }
        }
      } catch (err) {
        setBubbles((b) => [
          ...b,
          { id: nextId++, from: "celine", text: `⚠ ${String(err)}` },
        ]);
      } finally {
        setTyping(false);
        setBusy(false);
      }
    },
    [busy],
  );

  return { bubbles, typing, busy, send };
}
