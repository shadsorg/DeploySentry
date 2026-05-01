import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// The @dr-sentry/react package ships dual ESM/CJS via its `exports` map
// (see sdk/react/package.json). Vite picks the ESM build automatically;
// no alias or commonjsOptions override needed.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: { port: 4310, strictPort: true },
  preview: { port: 4310, strictPort: true },
});
