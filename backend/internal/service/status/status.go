// Package status is the application service for player-driven puzzle
// status changes ("solved" / "skipped") on the practice-flow endpoint.
package status

import (
	"context"
	"errors"
	"fmt"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ErrPuzzleNotFound is returned by Service.SetStatus when the addressed
// puzzle row does not exist. Handlers translate it to 404 without ever
// importing repository.ErrPuzzleNotFound directly.
var ErrPuzzleNotFound = errors.New("puzzle not found")

// PuzzleStore is the persistence surface used by Service.
type PuzzleStore interface {
	UpdateStatus(ctx context.Context, pk, sk, status string) error
}

// Service applies a status transition to a puzzle.
type Service struct {
	store PuzzleStore
}

// New constructs a Service.
func New(store PuzzleStore) *Service {
	return &Service{store: store}
}

// SetStatus updates the addressed puzzle's status. The (size, mode)
// pair locates the partition; puzzleID is the sort key.
//
// Returns ErrPuzzleNotFound when the puzzle row does not exist (the
// repository surfaces ConditionalCheckFailed via its own sentinel,
// which this method translates to the service-level sentinel).
func (s *Service) SetStatus(ctx context.Context, size int, mode, puzzleID, status string) error {
	pk := fmt.Sprintf("%d#%s", size, mode)
	if err := s.store.UpdateStatus(ctx, pk, puzzleID, status); err != nil {
		if errors.Is(err, repository.ErrPuzzleNotFound) {
			return ErrPuzzleNotFound
		}
		return fmt.Errorf("updating puzzle status: %w", err)
	}
	return nil
}
