import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import { VitePWA } from 'vite-plugin-pwa';
import path from 'path';

export default defineConfig(({ mode }) => {
  const env = { ...process.env, ...loadEnv(mode, process.cwd(), '') };
  const apiTarget = env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080';
  const devPort = Number(env.VITE_DEV_PORT ?? 3002);

  return {
    base: '/m/',
    plugins: [
      react(),
      VitePWA({
        registerType: 'autoUpdate',
        injectRegister: null,
        includeAssets: ['icon-192.png', 'icon-512.png', 'icon-maskable-512.png'],
        manifest: {
          id: '/m/',
          name: 'Deploy Sentry',
          short_name: 'DS',
          description: 'Monitor deployments and manage feature flags.',
          start_url: '/m/',
          scope: '/m/',
          display: 'standalone',
          orientation: 'portrait',
          background_color: '#0f1419',
          theme_color: '#0f1419',
          icons: [
            { src: 'icon-192.png', sizes: '192x192', type: 'image/png' },
            { src: 'icon-512.png', sizes: '512x512', type: 'image/png' },
            {
              src: 'icon-maskable-512.png',
              sizes: '512x512',
              type: 'image/png',
              purpose: 'maskable',
            },
          ],
        },
        workbox: {
          globPatterns: ['**/*.{js,css,html,png,svg,ico,webmanifest}'],
          navigateFallback: '/m/index.html',
          navigateFallbackDenylist: [/^\/api\//],
        },
        devOptions: {
          enabled: false,
        },
      }),
    ],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      port: devPort,
      strictPort: true,
      proxy: {
        '/api': { target: apiTarget, changeOrigin: true },
      },
    },
    build: {
      outDir: 'dist',
      sourcemap: true,
    },
  };
});
