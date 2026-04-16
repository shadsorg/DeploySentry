import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig(({ mode }) => {
  const env = { ...process.env, ...loadEnv(mode, process.cwd(), '') };
  const apiTarget = env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080';
  const devPort = Number(env.VITE_DEV_PORT ?? 3001);

  return {
    plugins: [react()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      port: devPort,
      strictPort: true,
      allowedHosts: ['dr-sentry.com', 'www.dr-sentry.com'],
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
        },
        '/install.sh': {
          target: apiTarget,
          changeOrigin: true,
        },
        '/health': {
          target: apiTarget,
          changeOrigin: true,
        },
        '/ready': {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },
    build: {
      outDir: 'dist',
      sourcemap: true,
    },
  };
});
