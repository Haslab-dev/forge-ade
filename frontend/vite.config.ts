import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import pkg from "./package.json" with { type: "json" };

// https://vite.dev/config/
export default defineConfig({
  base: "./",
  plugins: [react(), tailwindcss()],
  // Single source of truth: version lives in package.json (bumped by Makefile)
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
  },
  build: {
    rolldownOptions: {
      output: {
        // Split large vendor groups into separate chunks (rolldown API).
        advancedChunks: {
          groups: [
            { name: "vendor", test: /node_modules\/(react|react-dom|scheduler)\// },
            { name: "codemirror", test: /node_modules\/(codemirror|@codemirror)\// },
            { name: "xterm", test: /node_modules\/(xterm|xterm-addon-fit)\// },
            { name: "icons", test: /node_modules\/(@tabler\/icons-react|lucide-react)\// },
            { name: "marked", test: /node_modules\/marked\// },
          ],
        },
      },
    },
  },
});
