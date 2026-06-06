import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { CelineService } from "./gen/celine/v1/celine_pb";

// In dev, Vite proxies /celine.v1.* to the Go backend, so same-origin works.
// In production, the Go binary serves both the SPA and the RPC handlers on one port.
// VITE_CELINE_URL can override (e.g. staging URL) but is rarely needed.
const baseUrl = import.meta.env.VITE_CELINE_URL ?? window.location.origin;

const transport = createConnectTransport({ baseUrl });

export const celine = createClient(CelineService, transport);
