package daily

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// CandidateFreshnessWindow is the rolling window inside which an
// existing DAILY-CANDIDATE row is considered fresh enough to skip a
// reselect. 24h matches the T-6h cron's daily cadence (design §4).
const CandidateFreshnessWindow = 24 * time.Hour

// CandidatePoolSize / CandidatePoolMode scope candidate selection to
// the single canonical 9x9 Standard daily (D1). Constants — change
// when D1 rotation lands.
const (
	CandidatePoolSize = 9
	CandidatePoolMode = "standard"
)

// ErrCandidatePoolEmpty is returned by EnsureCandidate when the
// approved pool has no eligible puzzles. Caller logs and exits;
// T=0 will recycle yesterday (Finding 9, DP-16). Distinct from
// ErrPoolExhausted in sync.go because at T-6h we have the option
// of deferring to T=0 instead of failing loudly.
var ErrCandidatePoolEmpty = errors.New("approved candidate pool is empty")

// EnsureCandidate runs the T-6h cron algorithm (design §4 T-6h cron).
// See sync.go's package doc comment for the broader context.
//
// Algorithm:
//  1. GetCandidate. Fresh (queuedAt within 24h relative to `now`) -> nil.
//  2. List approved pool with excludeRecentlyDailied=true.
//  3. Empty -> ErrCandidatePoolEmpty.
//  4. Pick lexicographically smallest puzzleID — deterministic so
//     duplicate firings select the same row and the conditional
//     PutCandidateIfAbsent collapses cleanly.
//  5. PutCandidateIfAbsent. ErrCandidateAlreadyExists -> nil (race-loser).
//
// s.replenishHook, if non-nil, is invoked synchronously after a
// non-empty ListApprovedPool result, with the partition's (size, mode).
// The pool read drained the approved partition; the hook gives the
// caller a chance to publish auto-replenish messages. Wiring decides
// whether to dispatch async/sync. The hook is not invoked on
// fresh-candidate (no read), pool-empty, or pool-read-error paths.
//
// Future-clock-skew: a candidate with QueuedAt > now is treated as
// stale (refresh) — defensive, not strictly required.
func (s *Service) EnsureCandidate(ctx context.Context, now time.Time) error {
	existing, err := s.store.GetCandidate(ctx)
	if err != nil {
		return fmt.Errorf("ensure daily candidate: get candidate: %w", err)
	}
	if existing != nil && isCandidateFresh(existing, now) {
		return nil
	}

	pool, err := s.store.ListApprovedPool(ctx, CandidatePoolSize, CandidatePoolMode, true, now)
	if err != nil {
		return fmt.Errorf("ensure daily candidate: list approved pool: %w", err)
	}
	if len(pool) == 0 {
		return ErrCandidatePoolEmpty
	}

	// Approved-pool read succeeded with at least one row — partition
	// just drained by 1. Fire the auto-replenish hook (if wired) before
	// the Put: the Put outcome (winner/race-loser/error) doesn't
	// affect whether replenish should run.
	if s.replenishHook != nil {
		s.replenishHook(CandidatePoolSize, CandidatePoolMode)
	}

	pick := lowestPuzzleID(pool)
	sourcePartition := fmt.Sprintf("%d#%s", CandidatePoolSize, CandidatePoolMode)

	if err := s.store.PutCandidateIfAbsent(ctx, pick, sourcePartition); err != nil {
		if errors.Is(err, repository.ErrCandidateAlreadyExists) {
			return nil
		}
		return fmt.Errorf("ensure daily candidate: put candidate: %w", err)
	}
	return nil
}

// isCandidateFresh returns true when the candidate's QueuedAt timestamp
// is parseable AND within CandidateFreshnessWindow of `now`. A future
// QueuedAt (clock skew) counts as stale so the cron refreshes.
func isCandidateFresh(c *repository.CandidateRecord, now time.Time) bool {
	queuedAt, err := time.Parse(time.RFC3339, c.QueuedAt)
	if err != nil {
		log.Printf("daily cron: WARN: candidate.queuedAt unparseable, treating as stale: id=%s queuedAt=%q err=%v", c.PuzzleID, c.QueuedAt, err)
		return false
	}
	age := now.Sub(queuedAt)
	if age < 0 {
		return false
	}
	return age < CandidateFreshnessWindow
}

// lowestPuzzleID returns the lexicographically smallest puzzle ID in
// the slice. Stable across duplicate cron firings — required for the
// idempotent conditional PutCandidateIfAbsent.
func lowestPuzzleID(pool []repository.PuzzleRecord) string {
	sort.Slice(pool, func(i, j int) bool { return pool[i].ID < pool[j].ID })
	return pool[0].ID
}
