// Package serve is the application service for the player-facing
// "next puzzle" flow. It orchestrates the read-then-claim-then-replenish
// sequence so the HTTP handler stays a thin parser/responder.
package serve

import (
	"context"
	"errors"
	"fmt"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// PuzzleStore is the persistence surface used by Service.
type PuzzleStore interface {
	NextReady(ctx context.Context, size int, mode string) (*repository.PuzzleRecord, error)
	MarkServed(ctx context.Context, pk, sk string) error
}

// ReplenishNotifier is invoked synchronously after a successful drain
// so the reactive top-up path can fire. The hook is responsible for its
// own async dispatch and timeout — the service stays synchronous.
type ReplenishNotifier func(size int, mode string)

// Service orchestrates serving a ready puzzle to a player.
type Service struct {
	store     PuzzleStore
	replenish ReplenishNotifier
}

// New constructs a Service. A nil replenish notifier is supported —
// drained pools simply won't fire the reactive top-up path. Useful for
// local dev without SQS and for tests that don't exercise the hook.
func New(store PuzzleStore, replenish ReplenishNotifier) *Service {
	return &Service{store: store, replenish: replenish}
}

// NextPuzzle returns the next ready puzzle for (size, mode), atomically
// claims it (status → "served"), and fires the reactive replenish hook.
//
// Returns (nil, nil) when the pool is empty OR when another replica
// won the NextReady→MarkServed race. The handler maps both to 404
// "no puzzles available" — the player retries either way, so collapsing
// them at the service boundary keeps the HTTP layer simple.
func (s *Service) NextPuzzle(ctx context.Context, size int, mode string) (*repository.PuzzleRecord, error) {
	puzzle, err := s.store.NextReady(ctx, size, mode)
	if err != nil {
		return nil, fmt.Errorf("fetching next ready puzzle: %w", err)
	}
	if puzzle == nil {
		return nil, nil
	}

	pk := fmt.Sprintf("%d#%s", size, mode)
	if err := s.store.MarkServed(ctx, pk, puzzle.ID); err != nil {
		if errors.Is(err, repository.ErrPuzzleNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("marking puzzle served: %w", err)
	}

	if s.replenish != nil {
		s.replenish(size, mode)
	}

	return puzzle, nil
}
