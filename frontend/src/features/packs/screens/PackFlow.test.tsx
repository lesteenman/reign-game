import 'fake-indexeddb/auto';
import { render, screen, cleanup } from '@shared/test-utils';
import { fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { openDB, resetDBCache } from '@storage/db';
import {
  savePackProgress,
  emptyPackProgress,
  markSolved,
  loadPackProgress,
} from '@storage/packProgress';
import type { PackServeView } from '@features/packs/types/pack';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

// Replace the heavy shared GameBoard (Clerk + grid + timers) with a stub
// that surfaces the current puzzleId and a button that fires the
// onSolveDetected delegate — that's the contract PackFlow drives.
vi.mock('@shared/game/components/GameBoard', () => ({
  GameBoard: (props: {
    puzzle: { puzzleId: string };
    onSolveDetected?: (s: number[][], ms: number) => void;
  }) => (
    <div data-testid="gameboard-stub">
      <span data-testid="current-puzzle">{props.puzzle.puzzleId}</span>
      <button
        type="button"
        data-testid="fake-solve"
        onClick={() => props.onSolveDetected?.([[1]], 1000)}
      >
        solve
      </button>
    </div>
  ),
}));

import { PackFlow } from './PackFlow';

const DB_NAME = 'reign-game';
let originalFetch: typeof globalThis.fetch;

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function mockPackFetch(status: number, view: PackServeView | unknown) {
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString();
    if (url.includes('/api/packs/')) return jsonResponse(status, view);
    return jsonResponse(200, {});
  }) as typeof globalThis.fetch;
}

async function wipeDB(): Promise<void> {
  try {
    const db = await openDB();
    db.close();
  } catch {
    /* none */
  }
  resetDBCache();
  await new Promise<void>((resolve) => {
    const req = indexedDB.deleteDatabase(DB_NAME);
    req.onsuccess = () => resolve();
    req.onerror = () => resolve();
    req.onblocked = () => resolve();
  });
}

const meta = {
  difficulty: 1,
  maxTier: 1,
  tierCounts: [1],
  traceLen: 1,
  generationDurationMs: 10,
  createdAt: '2026-01-01T00:00:00Z',
};

const VIEW: PackServeView = {
  pack: { slug: 'starter', name: 'Starter Pack', size: 5, mode: 'standard', puzzleCount: 2 },
  puzzles: [
    { puzzleId: 'p1', regionMap: [[0]], metadata: meta },
    { puzzleId: 'p2', regionMap: [[0]], metadata: meta },
  ],
};

beforeEach(async () => {
  mockNavigate.mockClear();
  originalFetch = globalThis.fetch;
  await wipeDB();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  cleanup();
});

function renderFlow(slug: string | null) {
  return render(
    <MemoryRouter>
      <PackFlow slug={slug} />
    </MemoryRouter>,
  );
}

describe('PackFlow', () => {
  it('renders a not-found state for a missing slug (no pack fetch)', async () => {
    // Arrange — record calls; resolve everything so the connectivity
    // probe doesn't crash on an undefined return.
    const calls: string[] = [];
    globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
      calls.push(typeof input === 'string' ? input : input.toString());
      return jsonResponse(200, {});
    }) as typeof globalThis.fetch;

    // Act
    renderFlow(null);

    // Assert
    expect(await screen.findByTestId('pack-not-found')).toBeInTheDocument();
    expect(calls.some((u) => u.includes('/api/packs'))).toBe(false);
  });

  it('starts at the first puzzle for a fresh pack', async () => {
    // Arrange
    mockPackFetch(200, VIEW);

    // Act
    renderFlow('starter');

    // Assert
    expect(await screen.findByTestId('current-puzzle')).toHaveTextContent('p1');
  });

  it('resumes at the first unsolved puzzle from persisted progress', async () => {
    // Arrange — p1 already solved
    await savePackProgress(
      markSolved(emptyPackProgress('starter'), 'p1', ['p1', 'p2']),
    );
    mockPackFetch(200, VIEW);

    // Act
    renderFlow('starter');

    // Assert
    expect(await screen.findByTestId('current-puzzle')).toHaveTextContent('p2');
  });

  it('advances to the next puzzle on solve and persists progress', async () => {
    // Arrange
    mockPackFetch(200, VIEW);
    renderFlow('starter');
    expect(await screen.findByTestId('current-puzzle')).toHaveTextContent('p1');

    // Act
    fireEvent.click(screen.getByTestId('fake-solve'));

    // Assert — UI advances to p2
    await waitFor(() =>
      expect(screen.getByTestId('current-puzzle')).toHaveTextContent('p2'),
    );
    // ...and p1 is persisted as solved
    await waitFor(async () => {
      const stored = await loadPackProgress('starter');
      expect(stored?.solvedPuzzleIds).toEqual(['p1']);
    });
  });

  it('shows the pack-complete state once every puzzle is solved', async () => {
    // Arrange — both already solved
    let p = markSolved(emptyPackProgress('starter'), 'p1', ['p1', 'p2']);
    p = markSolved(p, 'p2', ['p1', 'p2']);
    await savePackProgress(p);
    mockPackFetch(200, VIEW);

    // Act
    renderFlow('starter');

    // Assert
    expect(await screen.findByTestId('pack-complete')).toBeInTheDocument();
  });

  it('transitions to complete after solving the final puzzle in-session', async () => {
    // Arrange — p1 solved, resume at p2 (the last one)
    await savePackProgress(
      markSolved(emptyPackProgress('starter'), 'p1', ['p1', 'p2']),
    );
    mockPackFetch(200, VIEW);
    renderFlow('starter');
    expect(await screen.findByTestId('current-puzzle')).toHaveTextContent('p2');

    // Act
    fireEvent.click(screen.getByTestId('fake-solve'));

    // Assert
    expect(await screen.findByTestId('pack-complete')).toBeInTheDocument();
  });

  it('shows a graceful error (not a crash) when the pack 404s', async () => {
    // Arrange
    mockPackFetch(404, { message: 'pack not found' });

    // Act
    renderFlow('starter');

    // Assert
    expect(await screen.findByTestId('pack-error')).toBeInTheDocument();
    expect(screen.getByTestId('pack-browse')).toBeInTheDocument();
  });
});
