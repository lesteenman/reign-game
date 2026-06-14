import { render, cleanup } from '@shared/test-utils';
import { describe, it, expect, afterEach } from 'vitest';
import { ThemeProvider } from '@theme/ThemeContext';
import { PackCandidatePreview } from './PackCandidatePreview';
import type { PackCandidate } from '@shared/types/pack';

afterEach(cleanup);

const CANDIDATE: PackCandidate = {
  puzzleId: 'p1',
  regionMap: [
    [0, 1, 2],
    [0, 1, 2],
    [0, 1, 2],
  ],
  metadata: {
    difficulty: 4,
    maxTier: 3,
    tierCounts: [1, 1, 1],
    traceLen: 6,
    generationDurationMs: 150,
    createdAt: '2026-06-14T00:00:00Z',
  },
};

describe('PackCandidatePreview', () => {
  it('renders the shared Grid for the candidate regionMap', async () => {
    // Arrange + Act
    const { container, findByTestId } = render(
      <ThemeProvider>
        <PackCandidatePreview candidate={CANDIDATE} mode="standard" />
      </ThemeProvider>,
    );

    // Assert — anchor on the loaded Grid before counting cells (lesson 13).
    await findByTestId('game-grid');
    const cells = container.querySelectorAll('[data-testid^="cell-"]');
    expect(cells.length).toBe(9); // 3x3 regionMap
  });

  it('shows difficulty and tier metadata', async () => {
    // Arrange + Act
    const { findAllByText } = render(
      <ThemeProvider>
        <PackCandidatePreview candidate={CANDIDATE} mode="standard" />
      </ThemeProvider>,
    );

    // Assert (StrictMode double-renders, so match one-or-more)
    const matches = await findAllByText('Difficulty 4 · Tier 3');
    expect(matches.length).toBeGreaterThan(0);
  });
});
