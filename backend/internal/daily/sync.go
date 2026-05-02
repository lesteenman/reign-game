// Package daily — stub. Real implementation follows in the next commit.
package daily

import (
	"context"
	"errors"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

var ErrPoolExhausted = errors.New("daily pool exhausted: no candidate and no yesterday schedule")

type Repo interface {
	GetSchedule(ctx context.Context, date string) (*repository.ScheduleRecord, error)
	GetCandidate(ctx context.Context) (*repository.CandidateRecord, error)
	FinalizeDailyTransaction(ctx context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error
}

func SyncFinalizeForToday(_ context.Context, _ Repo, _, _ string, _ time.Time) (*repository.ScheduleRecord, error) {
	return nil, errors.New("not implemented")
}
