import 'fake-indexeddb/auto';
import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook } from '@shared/test-utils';
import { useGameStorage } from './useGameStorage';
import { resetDBCache } from '@storage/db';
import type { GameState, CompletionRecord } from '@storage/types';
import type { PuzzleData } from '@engine/types';

const TEST_PUZZLE: PuzzleData = {
  puzzleId: 'test-001',
  gridSize: 3,
  mode: 'standard',
  regionMap: [
    [0, 0, 1],
    [0, 2, 1],
    [2, 2, 1],
  ],
};

function makeGameState(): GameState {
  return {
    id: 'curation:3x3-standard',
    flowType: 'curation',
    flowId: '3x3-standard',
    puzzle: TEST_PUZZLE,
    cells: [
      ['empty', 'excluded', 'empty'],
      ['marked', 'empty', 'empty'],
      ['empty', 'empty', 'marked'],
    ],
    timer: {
      elapsedAtLastPause: 42,
      lastResumedAt: null,
    },
    status: 'in-progress',
    startedAt: 1700000000000,
  };
}

// Reset the fake-indexeddb between tests so each test starts with a clean DB.
beforeEach(async () => {
  // Close any existing connection before deleting.
  const { openDB: getDB } = await import('@storage/db');
  try {
    const db = await getDB();
    db.close();
  } catch {
    // DB may not be open yet — that's fine.
  }
  // Reset the cached db promise so the next openDB() creates a fresh connection.
  resetDBCache();
  // Delete the database between tests to ensure isolation.
  await new Promise<void>((resolve, reject) => {
    const req = indexedDB.deleteDatabase('reign-game');
    req.onsuccess = () => resolve();
    req.onerror = () => reject(req.error);
  });
});

describe('useGameStorage', () => {
  it('loadState returns null when no state saved', async () => {
    const { result } = renderHook(() => useGameStorage());
    const loaded = await result.current.loadState('curation', '3x3-standard');
    expect(loaded).toBeNull();
  });

  it('saveState then loadState returns the same data', async () => {
    const { result } = renderHook(() => useGameStorage());
    const state = makeGameState();
    await result.current.saveState(state);
    const loaded = await result.current.loadState('curation', '3x3-standard');
    expect(loaded).toEqual(state);
  });

  it('clearState removes saved state', async () => {
    const { result } = renderHook(() => useGameStorage());
    await result.current.saveState(makeGameState());
    await result.current.clearState('curation', '3x3-standard');
    const loaded = await result.current.loadState('curation', '3x3-standard');
    expect(loaded).toBeNull();
  });

  it('keeps Flow Slots independent — writing one does not affect another', async () => {
    const { result } = renderHook(() => useGameStorage());
    const slotA = makeGameState();
    const slotB = { ...makeGameState(), id: 'curation:5x5-standard', flowId: '5x5-standard' };
    await result.current.saveState(slotA);
    await result.current.saveState(slotB);

    const loadedA = await result.current.loadState('curation', '3x3-standard');
    const loadedB = await result.current.loadState('curation', '5x5-standard');
    expect(loadedA).toEqual(slotA);
    expect(loadedB).toEqual(slotB);
  });

  it('addCompletion appends a record', async () => {
    const { result } = renderHook(() => useGameStorage());
    const record: CompletionRecord = {
      puzzleId: 'test-001',
      time: 120,
      completedAt: 1700000120000,
    };
    await result.current.addCompletion(record);

    // Read directly from the store to verify
    const { openDB } = await import('@storage/db');
    const db = await openDB();
    const tx = db.transaction('completions', 'readonly');
    const store = tx.objectStore('completions');
    const all = await new Promise<CompletionRecord[]>((resolve, reject) => {
      const req = store.getAll();
      req.onsuccess = () => resolve(req.result as CompletionRecord[]);
      req.onerror = () => reject(req.error);
    });
    expect(all).toHaveLength(1);
    expect(all[0]).toMatchObject(record);
  });
});
