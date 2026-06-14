import 'fake-indexeddb/auto';
import { describe, it, expect, beforeEach } from 'vitest';
import { openDB, resetDBCache } from './db';
import {
  emptyPackProgress,
  packProgressId,
  resumeIndex,
  markSolved,
  loadPackProgress,
  savePackProgress,
  loadAllPackProgress,
  type PackProgress,
} from './packProgress';

const DB_NAME = 'reign-game';

/** Wipe IDB between tests (close any cached connection first). */
async function wipeDB(): Promise<void> {
  try {
    const db = await openDB();
    db.close();
  } catch {
    // No prior connection.
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

describe('packProgressId / emptyPackProgress', () => {
  it('builds the pack Flow Slot key', () => {
    // Arrange + Act + Assert
    expect(packProgressId('starter')).toBe('pack:starter');
  });

  it('emptyPackProgress starts with no solved ids at index 0', () => {
    // Arrange + Act
    const progress = emptyPackProgress('starter');

    // Assert
    expect(progress).toEqual({
      id: 'pack:starter',
      slug: 'starter',
      solvedPuzzleIds: [],
      currentIndex: 0,
    });
  });
});

describe('resumeIndex', () => {
  const ids = ['p1', 'p2', 'p3'];

  it('returns 0 when nothing is solved', () => {
    // Arrange + Act + Assert
    expect(resumeIndex(ids, [])).toBe(0);
  });

  it('returns the first unsolved index regardless of solve order', () => {
    // Arrange + Act + Assert — p1 + p3 solved → resume at p2 (index 1)
    expect(resumeIndex(ids, ['p3', 'p1'])).toBe(1);
  });

  it('skips contiguous solved prefix', () => {
    // Arrange + Act + Assert
    expect(resumeIndex(ids, ['p1', 'p2'])).toBe(2);
  });

  it('returns puzzleIds.length when all are solved (complete signal)', () => {
    // Arrange + Act + Assert
    expect(resumeIndex(ids, ['p1', 'p2', 'p3'])).toBe(3);
  });

  it('ignores a solved id no longer present in the pack (re-curated)', () => {
    // Arrange — `gone` was solved but is no longer in the pack
    // Act + Assert — first unsolved present id is p1
    expect(resumeIndex(ids, ['gone'])).toBe(0);
  });

  it('returns 0 for an empty pack', () => {
    // Arrange + Act + Assert
    expect(resumeIndex([], [])).toBe(0);
  });
});

describe('markSolved', () => {
  const ids = ['p1', 'p2', 'p3'];

  it('appends the solved id and advances the cursor', () => {
    // Arrange
    const start = emptyPackProgress('starter');

    // Act
    const next = markSolved(start, 'p1', ids);

    // Assert
    expect(next.solvedPuzzleIds).toEqual(['p1']);
    expect(next.currentIndex).toBe(1);
  });

  it('is idempotent on a re-solved id', () => {
    // Arrange
    const start: PackProgress = {
      ...emptyPackProgress('starter'),
      solvedPuzzleIds: ['p1'],
      currentIndex: 1,
    };

    // Act
    const next = markSolved(start, 'p1', ids);

    // Assert
    expect(next.solvedPuzzleIds).toEqual(['p1']);
    expect(next.currentIndex).toBe(1);
  });

  it('drives the cursor past the end when the last puzzle is solved', () => {
    // Arrange
    const start: PackProgress = {
      ...emptyPackProgress('starter'),
      solvedPuzzleIds: ['p1', 'p2'],
      currentIndex: 2,
    };

    // Act
    const next = markSolved(start, 'p3', ids);

    // Assert
    expect(next.solvedPuzzleIds).toEqual(['p1', 'p2', 'p3']);
    expect(next.currentIndex).toBe(3);
  });
});

describe('loadPackProgress / savePackProgress (IndexedDB)', () => {
  it('returns null for a pack with no persisted progress', async () => {
    // Arrange + Act
    const loaded = await loadPackProgress('starter');

    // Assert
    expect(loaded).toBeNull();
  });

  it('round-trips a progress record across a fresh connection', async () => {
    // Arrange
    const progress = markSolved(emptyPackProgress('starter'), 'p1', [
      'p1',
      'p2',
    ]);
    await savePackProgress(progress);

    // Act — simulate a reload by dropping the cached connection
    const db = await openDB();
    db.close();
    resetDBCache();
    const loaded = await loadPackProgress('starter');

    // Assert
    expect(loaded).toEqual(progress);
  });

  it('keeps distinct packs in distinct slots', async () => {
    // Arrange
    await savePackProgress(
      markSolved(emptyPackProgress('alpha'), 'a1', ['a1', 'a2']),
    );
    await savePackProgress(emptyPackProgress('beta'));

    // Act
    const all = await loadAllPackProgress();

    // Assert
    expect(Object.keys(all).sort()).toEqual(['alpha', 'beta']);
    expect(all.alpha?.solvedPuzzleIds).toEqual(['a1']);
    expect(all.beta?.solvedPuzzleIds).toEqual([]);
  });
});
