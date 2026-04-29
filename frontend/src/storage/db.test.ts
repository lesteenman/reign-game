import 'fake-indexeddb/auto';
import { describe, it, expect, beforeEach } from 'vitest';
import { openDB, resetDBCache, idFor } from './db';

const DB_NAME = 'reign-game';

/**
 * Wipe IDB between tests. The cached connection (if any) must be closed
 * first — open connections block deleteDatabase indefinitely.
 */
async function wipeDB(): Promise<void> {
  try {
    const db = await openDB();
    db.close();
  } catch {
    // No prior connection — nothing to close.
  }
  resetDBCache();
  await new Promise<void>((resolve, reject) => {
    const req = indexedDB.deleteDatabase(DB_NAME);
    req.onsuccess = () => resolve();
    req.onerror = () => reject(req.error);
    req.onblocked = () => resolve();
  });
}

beforeEach(async () => {
  await wipeDB();
});

describe('idFor', () => {
  it('joins flowType and flowId with a colon', () => {
    // Arrange + Act + Assert
    expect(idFor('curation', '5x5-standard')).toBe('curation:5x5-standard');
    expect(idFor('daily', '2026-04-29')).toBe('daily:2026-04-29');
    expect(idFor('pack', 'beginner-001')).toBe('pack:beginner-001');
  });
});

describe('openDB v2 schema', () => {
  it('creates gameState and completions stores on first open', async () => {
    // Arrange + Act
    const db = await openDB();

    // Assert
    expect(Array.from(db.objectStoreNames)).toEqual(
      expect.arrayContaining(['gameState', 'completions']),
    );
    expect(db.version).toBe(2);
  });

  it('round-trips two distinct Flow Slots without overwriting each other', async () => {
    // Arrange
    const db = await openDB();
    const slotA = {
      id: idFor('curation', '5x5-standard'),
      flowType: 'curation',
      flowId: '5x5-standard',
      payload: 'A',
    };
    const slotB = {
      id: idFor('curation', '7x7-standard'),
      flowType: 'curation',
      flowId: '7x7-standard',
      payload: 'B',
    };

    // Act
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readwrite');
      tx.objectStore('gameState').put(slotA);
      tx.objectStore('gameState').put(slotB);
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
    });

    // Assert — both rows present, neither overwrote the other
    const all = await new Promise<unknown[]>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readonly');
      const req = tx.objectStore('gameState').getAll();
      req.onsuccess = () => resolve(req.result as unknown[]);
      req.onerror = () => reject(req.error);
    });
    expect(all).toHaveLength(2);
  });

  it('upserts on the same slot — three writes, one row, last wins', async () => {
    // Arrange
    const db = await openDB();
    const id = idFor('curation', '5x5-standard');
    const writes = [
      { id, flowType: 'curation', flowId: '5x5-standard', payload: 'first' },
      { id, flowType: 'curation', flowId: '5x5-standard', payload: 'second' },
      { id, flowType: 'curation', flowId: '5x5-standard', payload: 'third' },
    ];

    // Act
    for (const w of writes) {
      await new Promise<void>((resolve, reject) => {
        const tx = db.transaction('gameState', 'readwrite');
        tx.objectStore('gameState').put(w);
        tx.oncomplete = () => resolve();
        tx.onerror = () => reject(tx.error);
      });
    }

    // Assert
    const row = await new Promise<{ payload: string } | undefined>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readonly');
      const req = tx.objectStore('gameState').get(id);
      req.onsuccess = () => resolve(req.result as { payload: string } | undefined);
      req.onerror = () => reject(req.error);
    });
    expect(row?.payload).toBe('third');

    const all = await new Promise<unknown[]>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readonly');
      const req = tx.objectStore('gameState').getAll();
      req.onsuccess = () => resolve(req.result as unknown[]);
      req.onerror = () => reject(req.error);
    });
    expect(all).toHaveLength(1);
  });
});

describe('openDB v1 → v2 upgrade', () => {
  it('clears the gameState store on upgrade and preserves completions', async () => {
    // Arrange — open v1 by hand, seed legacy row + a completion
    await new Promise<void>((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, 1);
      req.onupgradeneeded = () => {
        const db = req.result;
        db.createObjectStore('gameState', { keyPath: 'id' });
        db.createObjectStore('completions', { autoIncrement: true });
      };
      req.onsuccess = () => {
        const db = req.result;
        const tx = db.transaction(['gameState', 'completions'], 'readwrite');
        tx.objectStore('gameState').put({ id: 'current', legacy: true });
        tx.objectStore('completions').add({ puzzleId: 'old-1', time: 42, completedAt: 1 });
        tx.oncomplete = () => {
          db.close();
          resolve();
        };
        tx.onerror = () => reject(tx.error);
      };
      req.onerror = () => reject(req.error);
    });

    // Act — open at v2 via the production helper, which fires onupgradeneeded
    resetDBCache();
    const db = await openDB();

    // Assert — gameState is empty
    const gameRows = await new Promise<unknown[]>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readonly');
      const req = tx.objectStore('gameState').getAll();
      req.onsuccess = () => resolve(req.result as unknown[]);
      req.onerror = () => reject(req.error);
    });
    expect(gameRows).toEqual([]);

    // Assert — completions are preserved
    const completionRows = await new Promise<unknown[]>((resolve, reject) => {
      const tx = db.transaction('completions', 'readonly');
      const req = tx.objectStore('completions').getAll();
      req.onsuccess = () => resolve(req.result as unknown[]);
      req.onerror = () => reject(req.error);
    });
    expect(completionRows).toHaveLength(1);
  });
});
