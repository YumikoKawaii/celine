import { useEffect, useRef, useState } from "react";
import { useChatStream } from "../hooks/useChatStream";
import { Starfield } from "./Starfield";
import { MagicCircle } from "./MagicCircle";

export function Chat() {
  const { bubbles, typing, busy, send } = useChatStream();
  const [draft, setDraft] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" });
  }, [bubbles, typing]);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    void send(draft);
    setDraft("");
  };

  const onKey = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void send(draft);
      setDraft("");
    }
  };

  return (
    <div className="chat">
      <Starfield />

      <header className="chat__header">
        <span className="chat__header-glyph">✦</span>
        Celine
        <span className="chat__header-glyph">✦</span>
      </header>

      <div className="chat__body">
        <div className="chat__log" ref={scrollRef}>
          {bubbles.length === 0 && !typing && (
            <div className="chat__empty">
              <MagicCircle />
              <p>What's on your mind?</p>
            </div>
          )}
          {bubbles.map((b) => (
            <div key={b.id} className={`bubble bubble--${b.from}`}>
              {b.from === "celine" && <MagicCircle />}
              <span className="bubble__text">{b.text}</span>
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
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={onKey}
            placeholder="speak your incantation…"
            rows={1}
            autoFocus
          />
          <button type="submit" disabled={busy || !draft.trim()}>
            ✦
          </button>
        </form>
      </div>
    </div>
  );
}
