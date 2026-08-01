import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
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
