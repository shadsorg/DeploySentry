import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// The @dr-sentry/react package.json advertises a `module` field pointing at
// dist/index.mjs, but the SDK is built as CommonJS (dist/index.js). Point Vite
// directly at the real entry so resolution succeeds.
const reactSdkEntry = path.resolve(__dirname, '../../../../sdk/react/dist/index.js');

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@dr-sentry/react': reactSdkEntry,
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    commonjsOptions: {
      // The React SDK is shipped as CommonJS. Rollup's CJS plugin needs to
      // transform it so named imports (useFlag, DeploySentryProvider, ...)
      // resolve correctly.
      include: [/sdk\/react\/dist/, /node_modules/],
      transformMixedEsModules: true,
    },
  },
  server: { port: 4310, strictPort: true },
  preview: { port: 4310, strictPort: true },
});
