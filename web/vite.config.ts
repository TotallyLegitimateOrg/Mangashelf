import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

const webPort = Number(process.env.MANGASHELF_DEV_WEB_PORT || "5173");
const apiPort = Number(process.env.MANGASHELF_DEV_HTTP_PORT || "3001");

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  server: {
    host: "0.0.0.0",
    port: webPort,
    strictPort: true,
    proxy: {
      "/api": `http://127.0.0.1:${apiPort}`,
      "/extensions": `http://127.0.0.1:${apiPort}`,
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
