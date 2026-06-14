// Session token + OAuth redirect mechanics. The login flow:
//   1. Eisodos  -> { url, state }: Google's authorize URL (sans redirect_uri).
//   2. We append OUR redirect_uri + the state, stash state, send the user to Google.
//   3. Google redirects back to redirect_uri with ?code&state.
//   4. Metabole({ code, redirect_uri }) -> { token, user }: we store the token.
// The same redirect_uri must be used in steps 2 and 4 (Google checks it matches).

const TOKEN_KEY = "celine.token";
const STATE_KEY = "celine.oauth_state";

// Where Google sends the user back. Registered as an Authorized redirect URI in
// the Google Cloud console. Path-less origin keeps registration simple.
export const redirectUri = window.location.origin + "/";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

export function stashState(state: string): void {
  sessionStorage.setItem(STATE_KEY, state);
}

// Validates the state echoed back by Google against what we stashed (CSRF guard),
// consuming it either way so it can't be replayed.
export function consumeState(): string | null {
  const s = sessionStorage.getItem(STATE_KEY);
  sessionStorage.removeItem(STATE_KEY);
  return s;
}

// Appends our redirect_uri + state to the bare authorize URL from Eisodos.
export function buildAuthUrl(base: string, state: string): string {
  const u = new URL(base);
  u.searchParams.set("redirect_uri", redirectUri);
  u.searchParams.set("state", state);
  return u.toString();
}

// Reads ?code&state from the current URL after Google redirects back, then strips
// them so a refresh doesn't re-trigger the exchange.
export function readCallback(): { code: string; state: string } | null {
  const params = new URLSearchParams(window.location.search);
  const code = params.get("code");
  const state = params.get("state");
  if (!code || !state) return null;
  window.history.replaceState({}, "", window.location.pathname);
  return { code, state };
}
