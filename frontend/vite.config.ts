import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('@codemirror/lang-') || id.includes('@codemirror/legacy-modes')) {
              return 'vendor-codemirror-langs';
            }
            if (id.includes('codemirror') || id.includes('@codemirror') || id.includes('@lezer')) {
              return 'vendor-codemirror-core';
            }
            if (id.includes('xterm')) {
              return 'vendor-xterm';
            }
            if (id.includes('@tabler/icons-react') || id.includes('lucide-react')) {
              return 'vendor-icons';
            }
            if (id.includes('react') || id.includes('react-dom') || id.includes('@radix-ui')) {
              return 'vendor-react';
            }
            return 'vendor-common';
          }
        },
      },
    },
  },
});
