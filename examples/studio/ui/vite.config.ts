import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import react from "@vitejs/plugin-react";
import tailwind from "@tailwindcss/vite";

export default defineConfig({
  plugins: [tailwind(), tanstackStart({ spa: { enabled: true, prerender: { outputPath: "/index.html" } } }), react()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  server: {
    host: "127.0.0.1", port: 5173, strictPort: true,
    proxy: { "/omniweb/": { target: "http://127.0.0.1:7331", changeOrigin: true } },
  },
});
