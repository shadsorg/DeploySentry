import { useSyncExternalStore } from 'react';
import {
  subscribeServiceWorker,
  applyServiceWorkerUpdate,
  type ServiceWorkerState,
} from '../registerSW';

const initialState: ServiceWorkerState = { needRefresh: false, offlineReady: false };
let cachedState: ServiceWorkerState = initialState;

function subscribe(callback: () => void): () => void {
  return subscribeServiceWorker((next) => {
    cachedState = next;
    callback();
  });
}

function getSnapshot(): ServiceWorkerState {
  return cachedState;
}

function getServerSnapshot(): ServiceWorkerState {
  return initialState;
}

export function SwUpdateBanner() {
  const state = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  if (!state.needRefresh) return null;

  return (
    <div className="m-sw-banner" role="status" aria-live="polite">
      <span className="m-sw-banner-text">Update available</span>
      <button
        type="button"
        className="m-button m-button-primary m-sw-banner-action"
        onClick={() => {
          void applyServiceWorkerUpdate();
        }}
      >
        Reload
      </button>
    </div>
  );
}
