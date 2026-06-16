import { useEffect, useRef, useState } from "react";
import { useChatStream } from "../hooks/useChatStream";
import { Starfield } from "./Starfield";
import { MagicCircle } from "./MagicCircle";
import type { User } from "../gen/celine/v1/hermes_pb";

export function Chat({
  user,
  onSignOut,
}: {
  user: User | null;
  onSignOut: () => void;
}) {
  const { bubbles, typing, busy, send, noteTyping } = useChatStream();
  const [draft, setDraft] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // rAF defers until after the browser has painted the new bubble, so
    // scrollHeight is fully settled and we don't land a few pixels short.
    const id = requestAnimationFrame(() => {
      scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" });
    });
    return () => cancelAnimationFrame(id);
  }, [bubbles, typing]);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    void send(draft);
    setDraft("");
  };

  const onKey = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    // Ignore Enter during IME composition and key repeat — both can fire a
    // second submit before React clears the draft.
    if (e.repeat || e.nativeEvent.isComposing) return;
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void send(draft);
      setDraft("");
    }
  };

  return (
    <div className="chat">
      <Starfield />
      <div className="magic-circle-bg">
        <MagicCircle />
      </div>

      <header className="chat__header">
        <span className="chat__header-glyph">✦</span>
        Celine
        <span className="chat__header-glyph">✦</span>
        <button
          className="chat__signout"
          type="button"
          onClick={onSignOut}
          title={user?.email ? `Sign out ${user.email}` : "Sign out"}
        >
          {user?.avatarUrl ? (
            <img className="chat__avatar" src={user.avatarUrl} alt="" />
          ) : (
            "⏻"
          )}
        </button>
      </header>

      <div className="chat__body">
        <div className="chat__log" ref={scrollRef}>
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
            onChange={(e) => {
              setDraft(e.target.value);
              noteTyping();
            }}
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
