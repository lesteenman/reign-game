import { useEffect, useState } from 'react';
import { apiKeyHeader } from '@shared/api/client';
import { useOnlineStatus } from './useOnlineStatus';

/**
 * Authoritative connectivity hook. Combines navigator.onLine (cheap,
 * event-driven) with an active probe against /api/health.
 *
 * Why the probe is needed: when the service worker serves the entire
 * cached app shell during a reload while offline, the browser never
 * attempts a real network request, so the `offline` event doesn't
 * fire and navigator.onLine stays `true` — a stale truth. Probing
 * /api/health (which is excluded from the SW's navigateFallbackDenylist
 * and not in any runtime cache) goes straight to the network and
 * gives us ground truth.
 *
 * Strategy: probe once on mount. If the probe fails, treat as
 * offline. Re-probe whenever the browser fires an `online` event
 * (the user *thinks* they're back; verify).
 */
export function useConnectivity(): boolean {
  const navigatorOnline = useOnlineStatus();
  const [probeOffline, setProbeOffline] = useState(false);

  useEffect(() => {
    let cancelled = false;

    function probe() {
      fetch('/api/health', {
        method: 'HEAD',
        cache: 'no-store',
        headers: apiKeyHeader(),
      })
        .then((r) => {
          if (!cancelled) setProbeOffline(!r.ok);
        })
        .catch(() => {
          if (!cancelled) setProbeOffline(true);
        });
    }

    probe();

    function onOnline() {
      probe();
    }
    window.addEventListener('online', onOnline);

    return () => {
      cancelled = true;
      window.removeEventListener('online', onOnline);
    };
  }, []);

  return navigatorOnline && !probeOffline;
}
