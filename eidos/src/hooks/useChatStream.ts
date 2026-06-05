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

// useChatStream owns the conversation state and consumes the server-streaming
// Chat RPC. The backend paces the bubbles (§14): a `typing` event flips the
// indicator on, the following `message` event delivers the whole bubble.
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
          // Hoist the oneof into a const local so discriminant narrowing
          // survives the setState calls below (property-access narrowing on
          // ev.event would be reset by an intervening function call).
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
