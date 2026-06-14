import { Starfield } from "./Starfield";
import { MagicCircle } from "./MagicCircle";

export function Login({
  onSignIn,
  error,
}: {
  onSignIn: () => void;
  error: string | null;
}) {
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
      </header>

      <div className="login">
        <p className="login__prompt">Speak your name to enter.</p>
        <button className="login__button" type="button" onClick={onSignIn}>
          Sign in with Google
        </button>
        {error && <p className="login__error">{error}</p>}
      </div>
    </div>
  );
}
