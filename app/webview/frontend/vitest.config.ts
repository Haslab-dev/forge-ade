import { defineConfig } from "vitest/config";
import path from "node:path";

// Standalone test config: no react/tailwind plugins needed (vitest's esbuild
// applies the tsconfig jsx setting). Mirrors the app's `define` globals so
// modules importing lib/utils.ts resolve outside a full vite build.
export default defineConfig({
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
  },
  define: {
    __APP_VERSION__: JSON.stringify("0.0.0-test"),
    __APP_BUILD__: JSON.stringify("0"),
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
