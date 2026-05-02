// Package daily orchestrates higher-level Daily Puzzle operations
// that compose multiple repository calls (sync-fallback finalize,
// the cron's T-6h ensure + T=0 finalize, etc.). Single-step DDB
// primitives stay in repository/daily.go; multi-step algorithms
// land here so handler and cron share the same code path.
package daily

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ErrPoolExhausted is returned by SyncFinalizeForToday when the
// system has no usable puzzle: candidate is empty AND yesterday's
// schedule row does not exist. Operators must surface this — the
// daily endpoint will 500 until a candidate is queued or a manual
// recovery happens (DP-16, Finding 9). NEVER silently no-op.
var ErrPoolExhausted = errors.New("daily pool exhausted: no candidate and no yesterday schedule")

// Repo is the narrow interface SyncFinalizeForToday needs. The
// production *repository.PuzzleRepository satisfies it; tests use
// a hand-rolled fake in this same package.
type Repo interface {
	GetSchedule(ctx context.Context, date string) (*repository.ScheduleRecord, error)
	GetCandidate(ctx context.Context) (*repository.CandidateRecord, error)
	FinalizeDailyTransaction(ctx context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error

	ListApprovedPool(ctx context.Context, size int, mode string, excludeRecentlyDailied bool, now time.Time) ([]repository.PuzzleRecord, error)
	PutCandidateIfAbsent(ctx context.Context, puzzleID, sourcePartition string) error
}

// SyncFinalizeForToday runs the T=0 finalize algorithm for `today`
// when the schedule row is missing (DP-05). It is invoked by the GET
// handler (chunk 3c) and is also the algorithm the T=0 cron will use
// (chunk 5 — extracted here for sharing).
//
// Algorithm (design §4 T=0 cron, decision tree at step 4):
//
//  1. GetSchedule(yesterday). Missing -> treat solved=0.
//  2. GetCandidate.
//  3. Decision tree:
//     candidate empty AND yesterday missing -> ErrPoolExhausted
//     candidate empty AND yesterday present -> recycle yesterday
//     candidate present AND yesterday missing -> confirm candidate
//     candidate present AND yesterday.solved == 0 -> recycle yesterday
//     candidate present AND yesterday.solved > 0 -> confirm candidate
//  4. FinalizeDailyTransaction with chosen puzzleID, sourcePartition, mode.
//  5. ErrScheduleAlreadyFinalized -> race-loser, GetSchedule(today),
//     return that row.
//  6. Success -> GetSchedule(today) and return the canonical row
//     (assignedAt was stamped inside the transaction).
//
// `today` and `yesterday` are caller-supplied to keep this package
// time-zone-agnostic and trivially testable. Caller is responsible
// for computing them as YYYY-MM-DD UTC.
func SyncFinalizeForToday(
	ctx context.Context,
	repo Repo,
	today, yesterday string,
	now time.Time,
) (*repository.ScheduleRecord, error) {
	yesterdaySchedule, err := repo.GetSchedule(ctx, yesterday)
	if err != nil {
		return nil, fmt.Errorf("sync finalize for %s: reading yesterday schedule: %w", today, err)
	}

	candidate, err := repo.GetCandidate(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync finalize for %s: reading candidate: %w", today, err)
	}

	puzzleID, sourcePartition, mode, err := chooseFinalizeTarget(candidate, yesterdaySchedule)
	if err != nil {
		return nil, err
	}

	finalizeErr := repo.FinalizeDailyTransaction(ctx, today, puzzleID, sourcePartition, mode)
	if finalizeErr != nil && !errors.Is(finalizeErr, repository.ErrScheduleAlreadyFinalized) {
		return nil, fmt.Errorf("sync finalize for %s: %w", today, finalizeErr)
	}

	canonical, err := repo.GetSchedule(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("sync finalize for %s: re-reading schedule: %w", today, err)
	}
	if canonical == nil {
		return nil, fmt.Errorf("sync finalize for %s: schedule vanished after FinalizeDailyTransaction commit (race=%t)", today, errors.Is(finalizeErr, repository.ErrScheduleAlreadyFinalized))
	}
	return canonical, nil
}

// chooseFinalizeTarget implements the design §4 step-4 decision tree.
// Pulled out for readability and unit-testable in isolation if we ever
// want to.
func chooseFinalizeTarget(
	candidate *repository.CandidateRecord,
	yesterday *repository.ScheduleRecord,
) (puzzleID, sourcePartition string, mode repository.FinalizeMode, err error) {
	yesterdayPresent := yesterday != nil
	candidatePresent := candidate != nil

	if !candidatePresent && !yesterdayPresent {
		return "", "", "", ErrPoolExhausted
	}

	if !candidatePresent && yesterdayPresent {
		return yesterday.PuzzleID, yesterday.SourcePartition, repository.FinalizeModeRecycle, nil
	}

	if candidatePresent && !yesterdayPresent {
		return candidate.PuzzleID, candidate.SourcePartition, repository.FinalizeModeConfirm, nil
	}

	if yesterday.Counters.Solved == 0 {
		return yesterday.PuzzleID, yesterday.SourcePartition, repository.FinalizeModeRecycle, nil
	}
	return candidate.PuzzleID, candidate.SourcePartition, repository.FinalizeModeConfirm, nil
}
