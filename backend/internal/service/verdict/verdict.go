// Package verdict is the application service for recording admin verdicts
// on puzzles and recomputing the cached summary projection. It keeps the
// HTTP handler a thin parser/responder.
package verdict

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ErrPuzzleNotFound is returned by Service.Submit when the addressed
// puzzle row does not exist. Handlers translate it to 404 without ever
// importing repository internals.
var ErrPuzzleNotFound = errors.New("puzzle not found")

// Store is the persistence surface used by Service.
type Store interface {
	GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error)
	PutVerdict(ctx context.Context, v *repository.VerdictRecord) error
	RecomputeVerdictSummary(ctx context.Context, size int, mode, puzzleID string) (repository.VerdictSummary, error)
}

// SubmitInput is the plain-field input to Service.Submit. Defined here so
// the HTTP handler can call Submit without importing repository internals.
// The service constructs the repository.VerdictRecord from these fields.
type SubmitInput struct {
	GridSize      int
	Mode          string
	PuzzleID      string
	RaterRole     string
	RaterID       string
	Value         string
	PlayTimeMs    int64
	Outcome       string
	ClientVersion string
}

// Summary is the service-level verdict summary returned by Submit.
// The handler constructs its response DTO from these plain fields,
// keeping the handler free of any repository import.
type Summary struct {
	Up            int
	Down          int
	LastUpdatedAt string
}

// Service records verdicts and recomputes the summary projection.
type Service struct {
	store Store
}

// New constructs a Service.
func New(store Store) *Service {
	return &Service{store: store}
}

// Submit records a verdict and returns the recomputed summary.
//
// Returns ErrPuzzleNotFound when the addressed puzzle does not exist
// (GetPuzzle returns nil). Other GetPuzzle errors are wrapped.
// PutVerdict errors are wrapped.
//
// RecomputeVerdictSummary is best-effort: its failure does NOT cause
// Submit to fail. repository.ErrPuzzleNotFound during recompute is a
// benign race (puzzle deleted between PutVerdict and recompute) and the
// empty Summary{} is returned. Other recompute failures are logged
// WARN and the empty summary is returned.
func (s *Service) Submit(ctx context.Context, in *SubmitInput) (Summary, error) {
	puzzle, err := s.store.GetPuzzle(ctx, in.GridSize, in.Mode, in.PuzzleID)
	if err != nil {
		return Summary{}, fmt.Errorf("looking up puzzle: %w", err)
	}
	if puzzle == nil {
		return Summary{}, ErrPuzzleNotFound
	}

	v := &repository.VerdictRecord{
		GridSize:      in.GridSize,
		Mode:          in.Mode,
		PuzzleID:      in.PuzzleID,
		RaterRole:     in.RaterRole,
		RaterID:       in.RaterID,
		Value:         in.Value,
		PlayTimeMs:    in.PlayTimeMs,
		Outcome:       in.Outcome,
		ClientVersion: in.ClientVersion,
	}
	if err := s.store.PutVerdict(ctx, v); err != nil {
		return Summary{}, fmt.Errorf("recording verdict: %w", err)
	}

	recomputed, recomputeErr := s.store.RecomputeVerdictSummary(ctx, in.GridSize, in.Mode, in.PuzzleID)
	if recomputeErr != nil {
		if errors.Is(recomputeErr, repository.ErrPuzzleNotFound) {
			log.Printf("WARN: verdict: summary recompute hit ErrPuzzleNotFound (race) puzzle=%s rater=%s", in.PuzzleID, in.RaterID)
		} else {
			log.Printf("WARN: verdict: summary recompute failed puzzle=%s rater=%s err=%v", in.PuzzleID, in.RaterID, recomputeErr)
		}
		return Summary{}, nil
	}

	return Summary{
		Up:            recomputed.Up,
		Down:          recomputed.Down,
		LastUpdatedAt: recomputed.LastUpdatedAt,
	}, nil
}
