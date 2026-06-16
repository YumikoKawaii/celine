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
  const { bubbles, typing, send, noteTyping } = useChatStream();
  const [draft, setDraft] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // rAF defers until after the browser has painted the new bubble, so
    // scrollHeight is fully settled and we don't land a few pixels short.
    const id = requestAnimationFrame(() => {
      const el = scrollRef.current;
      if (!el) return;
      const target = el.scrollHeight - el.clientHeight;
      const distance = target - el.scrollTop;
      // Bubbles arrive paced apart (§14); a fresh `smooth` animation on each one
      // gets restarted before it finishes, so over a long log it crawls and
      // never catches the bottom — the visible lag. Smooth-scroll only the
      // small final nudge; snap instantly when we're far behind so each bubble
      // lands immediately.
      el.scrollTo({ top: target, behavior: distance > 600 ? "auto" : "smooth" });
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
          <button type="submit" disabled={!draft.trim()}>
            ✦
          </button>
        </form>
      </div>
    </div>
  );
}
