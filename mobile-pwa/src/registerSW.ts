import { registerSW } from 'virtual:pwa-register';

export function initServiceWorker() {
  // autoUpdate: Workbox installs the new SW automatically on next nav.
  // Phase 1 prompts for immediate reload via a console log; a proper banner
  // UI replaces this in phase 6.
  const update = registerSW({
    immediate: true,
    onNeedRefresh() {
      // eslint-disable-next-line no-console
      console.info('[pwa] update available — reloading');
      void update(true);
    },
    onOfflineReady() {
      // eslint-disable-next-line no-console
      console.info('[pwa] offline-ready');
    },
  });
}
