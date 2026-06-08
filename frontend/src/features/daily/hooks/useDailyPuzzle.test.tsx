import { renderHook, waitFor, act } from '@shared/test-utils';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { focusManager } from '@tanstack/react-query';
import { useDailyPuzzle } from './useDailyPuzzle';
import type { DailyPuzzlePayload } from '@shared/types/daily';

// Mock the daily service so each window-focus refetch is observable.
const mockGetDaily = vi.fn();
vi.mock('@features/daily/services/dailyService', async () => {
  const actual = await vi.importActual<
    typeof import('@features/daily/services/dailyService')
  >('@features/daily/services/dailyService');
  return { ...actual, getDaily: (date?: string) => mockGetDaily(date) };
});

// Mock storage so the IDB short-circuit is a no-op (null) and the
// queryFn always reaches the network path under test.
import type { GameState } from '@storage/types';
const mockLoadState = vi.fn<
  (...args: [unknown, unknown]) => Promise<GameState | null>
>(async () => null);
vi.mock('@shared/game/hooks/useGameStorage', () => ({
  useGameStorage: () => ({
    saveState: vi.fn(async () => {}),
    loadState: mockLoadState,
    clearState: vi.fn(async () => {}),
    addCompletion: vi.fn(async () => {}),
  }),
}));

const PLAYING_PAYLOAD: DailyPuzzlePayload = {
  puzzleId: 'daily-2026-06-06',
  grid: 5,
  regions: [
    [0, 0, 1, 1, 2],
    [0, 0, 1, 1, 2],
    [3, 3, 4, 4, 2],
    [3, 3, 4, 4, 2],
    [3, 3, 4, 4, 2],
  ],
  assignedAt: '2026-06-06T00:00:00Z',
  outcome: 'started',
  isRecycle: false,
};

const SOLVED_PAYLOAD: DailyPuzzlePayload = {
  ...PLAYING_PAYLOAD,
  outcome: 'solved',
  serverElapsedMs: 9_876,
  submittedAt: '2026-06-06T10:00:00Z',
};

describe('useDailyPuzzle refetchOnWindowFocus', () => {
  beforeEach(() => {
    mockGetDaily.mockReset();
    mockLoadState.mockClear();
    focusManager.setFocused(true);
  });

  it('refetches on window focus and surfaces a newly-solved server state', async () => {
    // Arrange: first load returns a playing payload (e.g. tab B mid-play).
    mockGetDaily.mockResolvedValueOnce(PLAYING_PAYLOAD);
    const { result } = renderHook(() => useDailyPuzzle());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({
      kind: 'playing',
      payload: PLAYING_PAYLOAD,
    });

    // Act: the server now records the daily as solved (tab A solved it);
    // simulate the browser regaining focus on tab B.
    mockGetDaily.mockResolvedValueOnce(SOLVED_PAYLOAD);
    act(() => {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
    });

    // Assert: the focus refetch re-ran the queryFn and the solved state
    // propagated without a manual reload.
    await waitFor(() =>
      expect(result.current.data).toEqual({
        kind: 'solved',
        payload: SOLVED_PAYLOAD,
        result: { serverElapsedMs: 9_876, leaderboardRank: undefined },
        submittedAt: '2026-06-06T10:00:00Z',
      }),
    );
    expect(mockGetDaily).toHaveBeenCalledTimes(2);
  });
});
