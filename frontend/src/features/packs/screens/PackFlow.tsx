import { useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { styled, Text, View } from 'tamagui';
import { PageShell } from '@shared/components/PageShell';
import { SecondaryButton } from '@shared/components/Button';
import { Card } from '@shared/components/Card';
import { Spinner } from '@shared/components/Spinner';
import { ApiError } from '@shared/api';
import { usePack } from '@features/packs/hooks/usePack';
import { usePackProgress } from '@features/packs/hooks/usePackProgress';
import { resumeIndex } from '@storage/packProgress';
import { PackGameBoard } from './PackGameBoard';
import { PackCompleteScreen } from './PackCompleteScreen';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styledAny = styled as any;

const LoadingPanel = styledAny(View, {
  name: 'PackFlowLoadingPanel',
  alignItems: 'center',
  gap: 16,
  paddingVertical: 48,
});

const LoadingCaption = styledAny(Text, {
  name: 'PackFlowLoadingCaption',
  fontFamily: '"Nunito Sans", system-ui, sans-serif',
  fontWeight: '600',
  fontSize: '$4',
  color: '$body',
  margin: 0,
});

const ErrorText = styledAny(Text, {
  name: 'PackFlowErrorText',
  fontSize: '$5',
  fontWeight: '700',
  color: '$body',
  textAlign: 'center',
  render: <p />,
  margin: 0,
});

export interface PackFlowProps {
  /** Pack slug from `?pack=`. Null → not-found state (no fetch). */
  slug: string | null;
}

/** Maps a thrown pack-load error to user-facing copy. */
function loadErrorCopy(err: ApiError | Error): string {
  const status = err instanceof ApiError ? err.status : null;
  if (status === 404) return 'This pack is no longer available';
  return 'Could not load this pack';
}

/**
 * Pack-play flow chrome. Mirrors DailyFlow's structure: a read query
 * (`usePack`) + local progress (`usePackProgress`) drive the render.
 *
 * Render branches:
 *   slug === null                  → not-found (missing/unknown slug)
 *   pack or progress isPending     → loading
 *   pack.error (incl. 404)         → graceful error w/ exit to browse
 *   resumeIndex ≥ puzzleCount      → PackCompleteScreen
 *   otherwise                      → PackGameBoard for the current puzzle
 *
 * On solve, `recordSolve` marks the puzzle solved + advances the cursor
 * (persisted to IndexedDB at `pack:{slug}`); the next render shows the
 * next unsolved puzzle, or the complete screen when the cursor passes
 * the end. A pack that 404s mid-session (deleted/unpublished) surfaces
 * via `pack.error` as a graceful exit, not a crash.
 */
export function PackFlow({ slug }: PackFlowProps) {
  const navigate = useNavigate();
  const packQuery = usePack(slug);
  const { query: progressQuery, recordSolve } = usePackProgress(slug);

  const handleBrowse = useCallback(() => {
    navigate('/packs');
  }, [navigate]);

  const retryFetch = useCallback(() => {
    void packQuery.refetch();
  }, [packQuery]);

  const puzzleIds = useMemo(
    () => packQuery.data?.puzzles.map((p) => p.puzzleId) ?? [],
    [packQuery.data],
  );

  const handleSolved = useCallback(
    (_solution: number[][], _elapsedMs: number, puzzleId: string) => {
      void recordSolve(puzzleId, puzzleIds);
    },
    [recordSolve, puzzleIds],
  );

  // Missing / unknown slug → not-found. usePack is disabled in this
  // branch (enabled: slug !== null), so no request fired.
  if (slug === null) {
    return (
      <PageShell onBack={handleBrowse} subtitle="Puzzle Packs">
        <Card size="compact" data-testid="pack-not-found">
          <ErrorText>Pack not found</ErrorText>
          <SecondaryButton onClick={handleBrowse} data-testid="pack-browse">
            Browse packs
          </SecondaryButton>
        </Card>
      </PageShell>
    );
  }

  if (packQuery.isPending || progressQuery.isPending) {
    return (
      <PageShell onBack={handleBrowse} subtitle="Puzzle Packs">
        <LoadingPanel data-testid="pack-loading">
          <Spinner />
          <LoadingCaption>Loading pack…</LoadingCaption>
        </LoadingPanel>
      </PageShell>
    );
  }

  if (packQuery.error) {
    return (
      <PageShell onBack={handleBrowse} subtitle="Puzzle Packs">
        <Card size="compact" data-testid="pack-error">
          <ErrorText>{loadErrorCopy(packQuery.error)}</ErrorText>
          <SecondaryButton onClick={retryFetch} data-testid="pack-retry">
            Try again
          </SecondaryButton>
          <SecondaryButton onClick={handleBrowse} data-testid="pack-browse">
            Browse packs
          </SecondaryButton>
        </Card>
      </PageShell>
    );
  }

  const { pack, puzzles } = packQuery.data;
  const solvedPuzzleIds = progressQuery.data?.solvedPuzzleIds ?? [];
  const index = resumeIndex(puzzleIds, solvedPuzzleIds);
  const currentPuzzle = puzzles[index];

  // Cursor at or past the end (no current puzzle) → every puzzle solved.
  // Also covers an empty pack — puzzleCount 0 → index 0, no puzzle.
  if (index >= puzzles.length || currentPuzzle === undefined) {
    return (
      <PackCompleteScreen
        packName={pack.name}
        puzzleCount={pack.puzzleCount}
      />
    );
  }

  const subtitle = `${pack.name} · ${index + 1}/${puzzles.length}`;

  return (
    <PageShell onBack={handleBrowse} subtitle={subtitle}>
      <PackGameBoard
        key={currentPuzzle.puzzleId}
        slug={pack.slug}
        mode={pack.mode}
        size={pack.size}
        puzzle={currentPuzzle}
        onSolved={(solution, elapsedMs) =>
          handleSolved(solution, elapsedMs, currentPuzzle.puzzleId)
        }
      />
    </PageShell>
  );
}
