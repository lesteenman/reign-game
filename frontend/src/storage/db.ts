import { idFor } from '@reign/core/storage';
import type { CompletionRecord, FlowType, GameState, GameStorage } from '@reign/core/storage';

const DB_NAME = 'reign-game';
const DB_VERSION = 3;

let dbPromise: Promise<IDBDatabase> | null = null;

/**
 * Open (or return cached) the IndexedDB database. The connection is shared
 * across all callers via a module-level promise.
 *
 * Upgrade behavior (DB_VERSION 1 → 2): the `gameState` store is cleared on
 * upgrade. Pre-Phase-7 rows used `id: 'current'`; the per-flow shape
 * (`id: '{flowType}:{flowId}'`) is incompatible, so the slice ships a
 * graceful drop rather than a row-level migration. `completions` survives.
 *
 * DB_VERSION 2 → 3 adds the `packProgress` store (local pack-play
 * progress, keyed `pack:{slug}`). Additive — existing stores survive.
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

      if (!db.objectStoreNames.contains('packProgress')) {
        db.createObjectStore('packProgress', { keyPath: 'id' });
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

/**
 * IndexedDB implementation of `@reign/core`'s `GameStorage` contract.
 * Each method opens (or reuses) the shared connection and wraps a single
 * IDB transaction in a promise.
 */
export const indexedDbGameStorage: GameStorage = {
  async saveState(state: GameState): Promise<void> {
    const db = await openDB();
    return new Promise<void>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readwrite');
      const store = tx.objectStore('gameState');
      const req = store.put(state);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  },

  async loadState(flowType: FlowType, flowId: string): Promise<GameState | null> {
    const db = await openDB();
    return new Promise<GameState | null>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readonly');
      const store = tx.objectStore('gameState');
      const req = store.get(idFor(flowType, flowId));
      req.onsuccess = () => resolve((req.result as GameState) ?? null);
      req.onerror = () => reject(req.error);
    });
  },

  async clearState(flowType: FlowType, flowId: string): Promise<void> {
    const db = await openDB();
    return new Promise<void>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readwrite');
      const store = tx.objectStore('gameState');
      const req = store.delete(idFor(flowType, flowId));
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  },

  async addCompletion(record: CompletionRecord): Promise<void> {
    const db = await openDB();
    return new Promise<void>((resolve, reject) => {
      const tx = db.transaction('completions', 'readwrite');
      const store = tx.objectStore('completions');
      const req = store.add(record);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  },
};
