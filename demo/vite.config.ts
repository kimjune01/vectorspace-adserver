import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 6969,
    proxy: {
      "/chat": "http://localhost:8080",
      "/ad-request": "http://localhost:8080",
      "/embed": "http://localhost:8080",
      "/embeddings": "http://localhost:8080",
      "/event": "http://localhost:8080",
      "/stats": "http://localhost:8080",
      "/tee": "http://localhost:8080",
    },
  },
});
