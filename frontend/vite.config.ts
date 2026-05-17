import path from "path";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";
import { resolveApiTarget } from "./src/config/env";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const apiTarget = resolveApiTarget(env.VITE_API_TARGET);

  return {
    plugins: [react()],
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
    build: {
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes("react-pdf") || id.includes("pdfjs-dist")) {
              return "pdf-viewer";
            }

            if (id.includes("node_modules")) {
              if (
                id.includes("react-router-dom") ||
                id.includes("react-dom") ||
                id.includes("/react/")
              ) {
                return "react-core";
              }

              if (
                id.includes("@reduxjs/toolkit") ||
                id.includes("react-redux") ||
                id.includes("@tanstack/react-query")
              ) {
                return "state-management";
              }

              if (
                id.includes("framer-motion") ||
                id.includes("lucide-react") ||
                id.includes("@radix-ui")
              ) {
                return "ui-vendor";
              }
            }
          },
        },
      },
    },
    server: {
      proxy: {
        "/api": {
          target: apiTarget,
          changeOrigin: true,
          ws: true,
        },
      },
    },
  };
});
