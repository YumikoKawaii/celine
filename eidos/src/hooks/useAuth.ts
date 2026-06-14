import { useCallback, useEffect, useState } from "react";
import { create } from "@bufbuild/protobuf";
import {
  EisodosRequestSchema,
  ExodosRequestSchema,
  GnorizoRequestSchema,
  MetaboleRequestSchema,
  type User,
} from "../gen/celine/v1/hermes_pb";
import { hermes } from "../client";
import {
  buildAuthUrl,
  clearToken,
  consumeState,
  getToken,
  readCallback,
  redirectUri,
  setToken,
  stashState,
} from "../auth";

type Status = "loading" | "authenticated" | "anonymous";

export interface Auth {
  status: Status;
  user: User | null;
  error: string | null;
  signIn: () => Promise<void>;
  signOut: () => void;
}

export function useAuth(): Auth {
  const [status, setStatus] = useState<Status>("loading");
  const [user, setUser] = useState<User | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Resolve auth on first load: finish a pending OAuth callback if one is in the
  // URL, otherwise validate any stored token. Runs exactly once.
  useEffect(() => {
    let cancelled = false;

    const resolve = async () => {
      const callback = readCallback();
      if (callback) {
        const expected = consumeState();
        if (!expected || callback.state !== expected) {
          if (!cancelled) {
            setError("Login failed: state mismatch. Please try again.");
            setStatus("anonymous");
          }
          return;
        }
        try {
          const res = await hermes.metabole(
            create(MetaboleRequestSchema, { code: callback.code, redirectUri }),
          );
          if (cancelled) return;
          setToken(res.token);
          setUser(res.user ?? null);
          setStatus("authenticated");
        } catch (e) {
          if (cancelled) return;
          clearToken();
          setError(e instanceof Error ? e.message : "Login failed.");
          setStatus("anonymous");
        }
        return;
      }

      if (!getToken()) {
        if (!cancelled) setStatus("anonymous");
        return;
      }

      // Have a token — confirm it's still valid (and hydrate the user).
      try {
        const res = await hermes.gnorizo(create(GnorizoRequestSchema, {}));
        if (cancelled) return;
        setUser(res.user ?? null);
        setStatus("authenticated");
      } catch {
        if (cancelled) return;
        clearToken();
        setStatus("anonymous");
      }
    };

    void resolve();
    return () => {
      cancelled = true;
    };
  }, []);

  const signIn = useCallback(async () => {
    setError(null);
    try {
      const res = await hermes.eisodos(create(EisodosRequestSchema, {}));
      stashState(res.state);
      window.location.assign(buildAuthUrl(res.url, res.state));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not start sign-in.");
    }
  }, []);

  const signOut = useCallback(() => {
    void hermes.exodos(create(ExodosRequestSchema, {})).catch(() => {});
    clearToken();
    setUser(null);
    setStatus("anonymous");
  }, []);

  return { status, user, error, signIn, signOut };
}
