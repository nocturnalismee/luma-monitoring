import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev server proxies /api to the local Go backend (AGENT.md section 6).
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://127.0.0.1:8080",
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
  },
});
