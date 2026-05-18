import { useSyncExternalStore } from 'react';

/**
 * Subscribe to browser online/offline events. Returns the latest
 * `navigator.onLine` value and re-renders consumers when it changes.
 *
 * Uses useSyncExternalStore so the snapshot stays consistent under
 * React concurrent rendering — naive useState + useEffect risks the
 * "tearing" pattern where two simultaneous renders disagree on the
 * current value.
 */

function subscribe(callback: () => void): () => void {
  window.addEventListener('online', callback);
  window.addEventListener('offline', callback);
  return () => {
    window.removeEventListener('online', callback);
    window.removeEventListener('offline', callback);
  };
}

function getSnapshot(): boolean {
  return navigator.onLine;
}

function getServerSnapshot(): boolean {
  // SSR / pre-hydration: assume online. The first effect run after
  // hydration will correct it if navigator.onLine is false.
  return true;
}

export function useOnlineStatus(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
