import { defineConfig } from "vite";

// https://vitejs.dev/config/
export default defineConfig(async () => ({
  build: {
    // build dist into the python package dir so it ships with the wheel
    // and is picked up by context_factory() / PyInstaller datas
    outDir: "../python/src/tauri_app_wheel/frontend",
  },
  // we set fixed port in python code, so it's better to fail if that port is not available
  server: {
    port: 1420,
    strictPort: true,
    watch: {
      ignored: ["**/.venv/**"],
    },
  },
}));
