import { useCallback, useMemo } from 'react';
import { idFor, openDB } from '../storage/db';
import type { CompletionRecord, FlowType, GameState } from '../storage/types';

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

  const loadState = useCallback(
    async (flowType: FlowType, flowId: string): Promise<GameState | null> => {
      const db = await openDB();
      return new Promise<GameState | null>((resolve, reject) => {
        const tx = db.transaction('gameState', 'readonly');
        const store = tx.objectStore('gameState');
        const req = store.get(idFor(flowType, flowId));
        req.onsuccess = () => resolve((req.result as GameState) ?? null);
        req.onerror = () => reject(req.error);
      });
    },
    [],
  );

  const clearState = useCallback(
    async (flowType: FlowType, flowId: string): Promise<void> => {
      const db = await openDB();
      return new Promise<void>((resolve, reject) => {
        const tx = db.transaction('gameState', 'readwrite');
        const store = tx.objectStore('gameState');
        const req = store.delete(idFor(flowType, flowId));
        req.onsuccess = () => resolve();
        req.onerror = () => reject(req.error);
      });
    },
    [],
  );

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
