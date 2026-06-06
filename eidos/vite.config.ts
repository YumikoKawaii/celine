import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // Forward Connect RPC calls (all paths under the proto package root) to
      // the Go backend. The browser sees only :5173, so no CORS preflight fires.
      "/celine.v1.": "http://localhost:8080",
    },
  },
});
