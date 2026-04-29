import { registerSW } from 'virtual:pwa-register';

export interface ServiceWorkerState {
  needRefresh: boolean;
  offlineReady: boolean;
}

type Listener = (state: ServiceWorkerState) => void;

const listeners = new Set<Listener>();
let state: ServiceWorkerState = { needRefresh: false, offlineReady: false };
let updateFn: ((reload?: boolean) => Promise<void>) | null = null;

function emit() {
  for (const l of listeners) l(state);
}

export function subscribeServiceWorker(l: Listener): () => void {
  listeners.add(l);
  l(state);
  return () => {
    listeners.delete(l);
  };
}

export function applyServiceWorkerUpdate(): Promise<void> {
  return updateFn ? updateFn(true) : Promise.resolve();
}

export function initServiceWorker() {
  updateFn = registerSW({
    immediate: true,
    onNeedRefresh() {
      state = { ...state, needRefresh: true };
      emit();
    },
    onOfflineReady() {
      state = { ...state, offlineReady: true };
      emit();
    },
  });
}
