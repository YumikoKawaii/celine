import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { CelineService } from "./gen/celine/v1/celine_pb";

const baseUrl = import.meta.env.VITE_CELINE_URL ?? "http://127.0.0.1:8787";

const transport = createConnectTransport({ baseUrl });

export const celine = createClient(CelineService, transport);
