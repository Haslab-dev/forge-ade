import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import pkg from "./package.json" with { type: "json" };

// NOTE: The old forgeDaemonPlugin (auto-spawning `bun run src/server/daemon.ts`)
// and the /api + /ws proxies to 127.0.0.1:45123 are GONE. The backend is now
// the in-shell services layer (src/services.zig) reached over the native
// bridge (window.zero.invoke("services.<Method>")). The frontend never talks
// to a loopback daemon anymore.

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
        codeSplitting: {
          groups: [
            { name: "vendor", test: /node_modules\/(react|react-dom|scheduler)\// },
            // Core CodeMirror runtime. The @codemirror/lang-* grammars are
            // deliberately excluded: they are loaded on demand via dynamic
            // import (src/lib/languages.ts) and must stay in their own
            // per-language chunks.
            { name: "codemirror", test: /node_modules\/(codemirror|@codemirror\/(state|view|language|commands|search|autocomplete|lint))\// },
            // One lazy chunk per editor grammar, loaded on file open.
            ...[
              "lang-javascript", "lang-go", "lang-python", "lang-rust",
              "lang-json", "lang-html", "lang-markdown", "lang-cpp",
              "lang-sql", "lang-php", "lang-css", "lang-less",
              "lang-sass", "lang-java", "lang-xml", "lang-vue",
            ].map((pkg) => ({ name: pkg.replace("lang-", ""), test: new RegExp(`node_modules/@codemirror/${pkg}/`) })),
            { name: "xterm", test: /node_modules\/(xterm|xterm-addon-fit)\// },
            { name: "icons", test: /node_modules\/(@tabler\/icons-react|lucide-react)\// },
            { name: "marked", test: /node_modules\/marked\// },
          ],
        },
      },
    },
  },
});
