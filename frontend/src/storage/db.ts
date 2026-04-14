const DB_NAME = 'reign-game';
const DB_VERSION = 1;

let dbPromise: Promise<IDBDatabase> | null = null;

/**
 * Open (or return cached) the IndexedDB database.
 * The connection is shared across all callers via a module-level promise.
 */
export function openDB(): Promise<IDBDatabase> {
  if (dbPromise) return dbPromise;

  dbPromise = new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION);

    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains('gameState')) {
        db.createObjectStore('gameState', { keyPath: 'id' });
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
