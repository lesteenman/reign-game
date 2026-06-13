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
// the single canonical 9x9 Standard daily. Constants — change when
// combo rotation lands.
const (
	CandidatePoolSize = 9
	CandidatePoolMode = "standard"
)

// ErrCandidatePoolEmpty is returned by EnsureCandidate when the
// approved pool has no eligible puzzles. Caller logs and exits;
// T=0 will recycle yesterday. Distinct from ErrPoolExhausted in
// sync.go because at T-6h we have the option of deferring to T=0
// instead of failing loudly.
var ErrCandidatePoolEmpty = errors.New("approved candidate pool is empty")

// EnsureCandidate runs the T-6h cron algorithm (design §4 T-6h cron).
// See sync.go's package doc comment for the broader context.
//
// Algorithm:
//  1. GetCandidate. Fresh (queuedAt within 24h relative to `now`) -> nil.
//  2. List approved (verdicted) pool with excludeRecentlyDailied=true.
//  3. Verdicted empty -> fall back to the no-downvote pool. Still
//     empty -> ErrCandidatePoolEmpty. Verdicted is strictly preferred.
//  4. Pick lexicographically smallest puzzleID — deterministic so
//     duplicate firings select the same row and the conditional
//     PutCandidateIfAbsent collapses cleanly.
//  5. PutCandidateIfAbsent. ErrCandidateAlreadyExists -> nil (race-loser).
//
// s.replenishHook, if non-nil, is invoked synchronously with the
// partition's (size, mode) in exactly one of two cases: the verdicted
// pool read returned ≥1 row (it just drained by 1), OR the verdicted
// pool was empty (the strongest low-pool signal, whether or not the
// fallback yields a candidate). The two cases are mutually exclusive, so
// the hook fires at most once. It is not invoked on the fresh-candidate
// (no read) or pool-read-error paths.
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
		// No verdicted puzzle available — an empty verdicted pool is the
		// strongest low-pool signal, so fire the auto-replenish hook (if
		// wired) regardless of whether the fallback produces a candidate.
		if s.replenishHook != nil {
			s.replenishHook(CandidatePoolSize, CandidatePoolMode)
		}

		// Fallback: serve a ready, not-down-voted puzzle instead of
		// recycling/erroring. Verdicted stays strictly preferred — this
		// branch runs only when the verdicted pool is empty.
		pool, err = s.store.ListReadyPoolNoDownvotes(ctx, CandidatePoolSize, CandidatePoolMode, true, now)
		if err != nil {
			return fmt.Errorf("ensure daily candidate: list no-downvote pool: %w", err)
		}
		if len(pool) == 0 {
			return ErrCandidatePoolEmpty
		}
	} else if s.replenishHook != nil {
		// Approved-pool read succeeded with at least one row — partition
		// just drained by 1. Fire the auto-replenish hook (if wired) before
		// the Put: the Put outcome (winner/race-loser/error) doesn't
		// affect whether replenish should run.
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
