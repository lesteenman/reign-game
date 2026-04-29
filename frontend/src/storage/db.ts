import type { FlowType } from './types';

const DB_NAME = 'reign-game';
const DB_VERSION = 2;

let dbPromise: Promise<IDBDatabase> | null = null;

/**
 * Composite Flow Slot key — the only place the `:` separator appears in
 * production code.
 */
export function idFor(flowType: FlowType, flowId: string): string {
  return `${flowType}:${flowId}`;
}

/**
 * Open (or return cached) the IndexedDB database. The connection is shared
 * across all callers via a module-level promise.
 *
 * Upgrade behavior (DB_VERSION 1 → 2): the `gameState` store is cleared on
 * upgrade. Pre-Phase-7 rows used `id: 'current'`; the per-flow shape
 * (`id: '{flowType}:{flowId}'`) is incompatible, so the slice ships a
 * graceful drop rather than a row-level migration. `completions` survives.
 */
export function openDB(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise;

  dbPromise = new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION);

    request.onupgradeneeded = (event) => {
      const db = request.result;
      const oldVersion = event.oldVersion;

      if (!db.objectStoreNames.contains('gameState')) {
        db.createObjectStore('gameState', { keyPath: 'id' });
      } else if (oldVersion < 2) {
        const tx = request.transaction;
        if (tx) {
          tx.objectStore('gameState').clear();
        }
      }

      if (!db.objectStoreNames.contains('completions')) {
        db.createObjectStore('completions', { autoIncrement: true });
      }
    };

    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });

  return dbPromise;
}

/**
 * Reset the cached db promise. Used in tests to ensure a fresh connection
 * after deleting the database.
 */
export function resetDBCache(): void {
  dbPromise = null;
}
