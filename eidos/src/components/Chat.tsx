import { useEffect, useRef, useState } from "react";
import { useChatStream } from "../hooks/useChatStream";

export function Chat() {
  const { bubbles, typing, busy, send } = useChatStream();
  const [draft, setDraft] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [bubbles, typing]);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    void send(draft);
    setDraft("");
  };

  return (
    <div className="chat">
      <header className="chat__header">Celine</header>

      <div className="chat__log" ref={scrollRef}>
        {bubbles.map((b) => (
          <div key={b.id} className={`bubble bubble--${b.from}`}>
            {b.text}
          </div>
        ))}
        {typing && (
          <div className="bubble bubble--celine bubble--typing">
            <span />
            <span />
            <span />
          </div>
        )}
      </div>

      <form className="chat__input" onSubmit={submit}>
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="say something to Celine…"
          autoFocus
        />
        <button type="submit" disabled={busy || !draft.trim()}>
          Send
        </button>
      </form>
    </div>
  );
}
