import { useCallback, useEffect, useState } from "react";
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

      setBubbles((b) => [...b, { id: nextId++, from: "user", text: trimmed }]);

      try {
        await celine.pempo(create(PempoRequestSchema, { text: trimmed }));
      } catch (err) {
        setBubbles((b) => [
          ...b,
          { id: nextId++, from: "celine", text: `⚠ ${String(err)}` },
        ]);
      }
    },
    [],
  );

  return { bubbles, typing, busy, send };
}
