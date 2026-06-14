import { createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { Celine } from "./gen/celine/v1/celine_pb";
import { Hermes } from "./gen/celine/v1/hermes_pb";
import { getToken } from "./auth";

// In dev, Vite proxies /celine.v1.* to the Go backend, so same-origin works.
// In production, the Go binary serves both the SPA and the RPC handlers on one port.
// VITE_CELINE_URL can override (e.g. staging URL) but is rarely needed.
const baseUrl = import.meta.env.VITE_CELINE_URL ?? window.location.origin;

// Attaches the stored session JWT as a Bearer token on every request. The auth
// interceptor on the server rejects calls without it (except the Hermes routes,
// which are how a token is obtained in the first place).
const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  return next(req);
};

const transport = createConnectTransport({
  baseUrl,
  interceptors: [authInterceptor],
});

export const celine = createClient(Celine, transport);
export const hermes = createClient(Hermes, transport);
