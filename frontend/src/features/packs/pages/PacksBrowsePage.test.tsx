import 'fake-indexeddb/auto';
import { render, screen, cleanup } from '@shared/test-utils';
import { fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { openDB, resetDBCache } from '@storage/db';
import { savePackProgress, emptyPackProgress, markSolved } from '@storage/packProgress';
import type { PackSummary } from '@features/packs/types/pack';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

import { PacksBrowsePage } from './PacksBrowsePage';

const DB_NAME = 'reign-game';
let originalFetch: typeof globalThis.fetch;

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

/** Mock fetch: /api/packs → packs; anything else → empty 200. */
function mockPacksFetch(status: number, packs: PackSummary[] | unknown) {
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString();
    if (url.includes('/api/packs')) return jsonResponse(status, packs);
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

const SAMPLE: PackSummary[] = [
  { slug: 'starter', name: 'Starter Pack', size: 5, mode: 'standard', puzzleCount: 8 },
  { slug: 'tricky', name: 'Tricky Pack', size: 9, mode: 'double', puzzleCount: 4 },
];

beforeEach(async () => {
  mockNavigate.mockClear();
  originalFetch = globalThis.fetch;
  await wipeDB();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  cleanup();
});

function renderBrowse() {
  return render(
    <MemoryRouter>
      <PacksBrowsePage />
    </MemoryRouter>,
  );
}

describe('PacksBrowsePage', () => {
  it('lists published packs with name + size/mode + progress', async () => {
    // Arrange
    mockPacksFetch(200, SAMPLE);

    // Act
    renderBrowse();

    // Assert
    const starter = await screen.findByTestId('pack-tile-starter');
    expect(starter).toHaveTextContent('Starter Pack');
    expect(starter).toHaveTextContent('5×5 Standard · 0/8');
    const tricky = screen.getByTestId('pack-tile-tricky');
    expect(tricky).toHaveTextContent('9×9 Double · 0/4');
  });

  it('shows local progress count from persisted records', async () => {
    // Arrange — 2 of starter's 8 solved
    let progress = markSolved(emptyPackProgress('starter'), 'p1', ['p1', 'p2']);
    progress = markSolved(progress, 'p2', ['p1', 'p2']);
    await savePackProgress(progress);
    mockPacksFetch(200, SAMPLE);

    // Act
    renderBrowse();

    // Assert
    const starter = await screen.findByTestId('pack-tile-starter');
    await waitFor(() =>
      expect(starter).toHaveTextContent('5×5 Standard · 2/8'),
    );
  });

  it('navigates to the pack play route on tile click', async () => {
    // Arrange
    mockPacksFetch(200, SAMPLE);
    renderBrowse();
    const starter = await screen.findByTestId('pack-tile-starter');

    // Act
    fireEvent.click(starter);

    // Assert
    expect(mockNavigate).toHaveBeenCalledWith('/play?flow=pack&pack=starter');
  });

  it('renders the empty state for no published packs', async () => {
    // Arrange
    mockPacksFetch(200, []);

    // Act
    renderBrowse();

    // Assert
    expect(await screen.findByTestId('packs-empty')).toBeInTheDocument();
  });

  it('renders an error state with retry on a failed list fetch', async () => {
    // Arrange
    mockPacksFetch(500, { message: 'boom' });

    // Act
    renderBrowse();

    // Assert
    expect(await screen.findByTestId('packs-error')).toBeInTheDocument();
    expect(screen.getByTestId('packs-retry')).toBeInTheDocument();
  });
});
