import { defineConfig, Plugin } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { spawn, ChildProcess } from "child_process";
import path from "path";
import fs from "fs";
import os from "os";
import pkg from "./package.json" with { type: "json" };

function forgeDaemonPlugin(): Plugin {
  let daemonProc: ChildProcess | null = null;
  return {
    name: "forge-daemon-starter",
    configureServer() {
      const home = os.homedir();
      const currentDir = import.meta.dirname || process.cwd();
      const daemonPath = path.resolve(currentDir, "../src/server/daemon.ts");
      if (fs.existsSync(daemonPath)) {
        try {
          const extraPaths = [
            "/opt/homebrew/bin",
            "/Users/hy4-mac-002/homebrew/bin",
            "/usr/local/bin",
            path.join(home, ".bun", "bin"),
            path.join(home, ".cargo", "bin"),
            path.join(home, "go", "bin"),
            path.join(home, ".local", "bin"),
          ];
          const envPath = `${extraPaths.join(":")}:${process.env.PATH || ""}`;

          daemonProc = spawn("bun", ["run", daemonPath], {
            cwd: path.resolve(currentDir, ".."),
            stdio: "inherit",
            env: {
              ...process.env,
              PATH: envPath,
            },
          });

          process.on("exit", () => {
            try { daemonProc?.kill(); } catch {}
          });
        } catch (err) {
          console.error("Failed to auto-spawn Forge daemon:", err);
        }
      }
    },
  };
}

// https://vite.dev/config/
export default defineConfig({
  base: "./",
  plugins: [react(), tailwindcss(), forgeDaemonPlugin()],
  // Single source of truth: version lives in package.json (bumped by Makefile)
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:45123",
        changeOrigin: true,
      },
      "/ws": {
        target: "ws://127.0.0.1:45123",
        ws: true,
      },
    },
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
