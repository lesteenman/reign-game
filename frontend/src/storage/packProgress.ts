import { openDB } from '@storage/db';

/**
 * Local pack-play progress, persisted to IndexedDB in the dedicated
 * `packProgress` object store. This is the canonical persisted shape —
 * consumers (hooks, screens, tests) import it from here rather than
 * redeclaring it (frontend/CLAUDE.md lesson 4).
 *
 * Local-only until Player Progress Sync (#128). Keyed by the Flow Slot
 * id `pack:{slug}` so DevTools inspection reads naturally alongside the
 * `gameState` Flow Slots.
 */
export interface PackProgress {
  /** Composite key for the IndexedDB record — `pack:{slug}`. */
  id: string;
  /** The pack's slug (duplicated onto the body for readability). */
  slug: string;
  /** Puzzle ids the player has solved, in solve order. */
  solvedPuzzleIds: string[];
  /**
   * Index the player is currently positioned at. Persisted so a reload
   * lands on the same puzzle even before any solve; the authoritative
   * resume target is recomputed from `solvedPuzzleIds` via `resumeIndex`.
   */
  currentIndex: number;
}

const STORE_NAME = 'packProgress';

/** Builds the `pack:{slug}` composite key for the progress store. */
export function packProgressId(slug: string): string {
  return `pack:${slug}`;
}

/** A zeroed progress record for a pack that has none persisted yet. */
export function emptyPackProgress(slug: string): PackProgress {
  return {
    id: packProgressId(slug),
    slug,
    solvedPuzzleIds: [],
    currentIndex: 0,
  };
}

/**
 * Resolve the resume index for a pack: the first index whose puzzleId is
 * not in the solved set. A solved id no longer present in `puzzleIds`
 * (admin re-curated the pack) is ignored — resume is by current order,
 * best-effort. Returns `puzzleIds.length` when every puzzle is solved,
 * which callers read as the pack-complete signal.
 */
export function resumeIndex(
  puzzleIds: string[],
  solvedPuzzleIds: string[],
): number {
  const solved = new Set(solvedPuzzleIds);
  const idx = puzzleIds.findIndex((id) => !solved.has(id));
  return idx === -1 ? puzzleIds.length : idx;
}

/**
 * Return updated progress with `puzzleId` marked solved and the cursor
 * advanced to the next unsolved puzzle. Idempotent — re-solving an
 * already-recorded id doesn't duplicate it. Pure: callers persist the
 * result via `savePackProgress`.
 */
export function markSolved(
  progress: PackProgress,
  puzzleId: string,
  puzzleIds: string[],
): PackProgress {
  const solvedPuzzleIds = progress.solvedPuzzleIds.includes(puzzleId)
    ? progress.solvedPuzzleIds
    : [...progress.solvedPuzzleIds, puzzleId];
  return {
    ...progress,
    solvedPuzzleIds,
    currentIndex: resumeIndex(puzzleIds, solvedPuzzleIds),
  };
}

/**
 * Read a pack's persisted progress. Returns null when none exists yet
 * (a fresh pack); callers default to `emptyPackProgress`.
 */
export async function loadPackProgress(
  slug: string,
): Promise<PackProgress | null> {
  const db = await openDB();
  return new Promise<PackProgress | null>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly');
    const store = tx.objectStore(STORE_NAME);
    const req = store.get(packProgressId(slug));
    req.onsuccess = () => resolve((req.result as PackProgress) ?? null);
    req.onerror = () => reject(req.error);
  });
}

/**
 * Read every persisted pack-progress record, keyed by slug. Used by the
 * browse surface to render per-pack local progress ("3/8") without a
 * per-row IDB read.
 */
export async function loadAllPackProgress(): Promise<
  Record<string, PackProgress>
> {
  const db = await openDB();
  return new Promise<Record<string, PackProgress>>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly');
    const store = tx.objectStore(STORE_NAME);
    const req = store.getAll();
    req.onsuccess = () => {
      const rows = (req.result as PackProgress[]) ?? [];
      const bySlug: Record<string, PackProgress> = {};
      for (const row of rows) {
        bySlug[row.slug] = row;
      }
      resolve(bySlug);
    };
    req.onerror = () => reject(req.error);
  });
}

/** Persist a pack's progress (upsert on the composite key). */
export async function savePackProgress(progress: PackProgress): Promise<void> {
  const db = await openDB();
  return new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite');
    const store = tx.objectStore(STORE_NAME);
    const req = store.put(progress);
    req.onsuccess = () => resolve();
    req.onerror = () => reject(req.error);
  });
}
