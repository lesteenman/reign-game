import { renderHook, waitFor } from '@shared/test-utils';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { usePackCandidates } from './usePackCandidates';
import type { PackCandidate } from '@shared/types/pack';

const mockFetchPackCandidates = vi.fn();
vi.mock('@features/admin/services/adminPackService', () => ({
  fetchPackCandidates: (...args: unknown[]) => mockFetchPackCandidates(...args),
}));

const MOCK_CANDIDATES: PackCandidate[] = [
  {
    puzzleId: 'p1',
    regionMap: [
      [0, 1],
      [0, 1],
    ],
    metadata: {
      difficulty: 3,
      maxTier: 2,
      tierCounts: [1, 2],
      traceLen: 5,
      generationDurationMs: 120,
      createdAt: '2026-06-14T00:00:00Z',
    },
  },
];

beforeEach(() => {
  mockFetchPackCandidates.mockReset();
});

describe('usePackCandidates', () => {
  it('fetches candidates when size and mode are set', async () => {
    // Arrange
    mockFetchPackCandidates.mockResolvedValue(MOCK_CANDIDATES);

    // Act
    const { result } = renderHook(() => usePackCandidates(7, 'standard'));

    // Assert
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(MOCK_CANDIDATES);
    expect(mockFetchPackCandidates).toHaveBeenCalledWith(
      7,
      'standard',
      expect.any(AbortSignal),
    );
  });

  it('is disabled (no fetch) when size is null', async () => {
    // Arrange
    mockFetchPackCandidates.mockResolvedValue(MOCK_CANDIDATES);

    // Act
    const { result } = renderHook(() => usePackCandidates(null, 'standard'));

    // Assert
    await waitFor(() => expect(result.current.fetchStatus).toBe('idle'));
    expect(mockFetchPackCandidates).not.toHaveBeenCalled();
  });
});
