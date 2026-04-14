import { useCallback, useMemo } from 'react';
import { openDB } from '../storage/db';
import type { GameState, CompletionRecord } from '../storage/types';

/** Hook providing IndexedDB persistence for game state and completions. */
export function useGameStorage() {
  const saveState = useCallback(async (state: GameState): Promise<void> => {
    const db = await openDB();
    return new Promise<void>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readwrite');
      const store = tx.objectStore('gameState');
      const req = store.put(state);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }, []);

  const loadState = useCallback(async (): Promise<GameState | null> => {
    const db = await openDB();
    return new Promise<GameState | null>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readonly');
      const store = tx.objectStore('gameState');
      const req = store.get('current');
      req.onsuccess = () => resolve((req.result as GameState) ?? null);
      req.onerror = () => reject(req.error);
    });
  }, []);

  const clearState = useCallback(async (): Promise<void> => {
    const db = await openDB();
    return new Promise<void>((resolve, reject) => {
      const tx = db.transaction('gameState', 'readwrite');
      const store = tx.objectStore('gameState');
      const req = store.delete('current');
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }, []);

  const addCompletion = useCallback(
    async (record: CompletionRecord): Promise<void> => {
      const db = await openDB();
      return new Promise<void>((resolve, reject) => {
        const tx = db.transaction('completions', 'readwrite');
        const store = tx.objectStore('completions');
        const req = store.add(record);
        req.onsuccess = () => resolve();
        req.onerror = () => reject(req.error);
      });
    },
    [],
  );

  return useMemo(
    () => ({ saveState, loadState, clearState, addCompletion }),
    [saveState, loadState, clearState, addCompletion],
  );
}
