// Stub for `virtual:pwa-register` used during unit tests. The real module is
// provided at build-time by vite-plugin-pwa and isn't resolvable in vitest.
export function registerSW(): (reload?: boolean) => Promise<void> {
  return async () => {};
}
