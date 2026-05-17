// Package puzzlestore is the application service for persisting newly
// generated puzzles. It exists so SQS workers and HTTP handlers can
// write puzzles via a plain-field input without importing
// internal/repository.
package puzzlestore

import (
	"context"
	"fmt"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// PuzzleInput is the plain-field payload for storing a freshly generated
// puzzle. Repository internals don't leak through this struct.
type PuzzleInput struct {
	ID                   string
	GridSize             int
	Mode                 string
	RegionMap            [][]int
	Solution             [][]bool
	Status               string
	Difficulty           int
	MaxTier              int
	TierCounts           []int
	TraceLen             int
	GenerationDurationMs int64
	CreatedAt            string
	Seed                 int64
}

// Store is the persistence surface used by Service. The production wiring
// passes *repository.PuzzleRepository; tests use a fake.
type Store interface {
	PutPuzzle(ctx context.Context, puzzle *repository.PuzzleRecord) error
}

// Service handles puzzle persistence.
type Service struct {
	store Store
}

// New constructs a Service.
func New(store Store) *Service {
	return &Service{store: store}
}

// StorePuzzle persists a freshly generated puzzle. Wraps any underlying
// store error.
func (s *Service) StorePuzzle(ctx context.Context, in *PuzzleInput) error {
	rec := &repository.PuzzleRecord{
		ID:                   in.ID,
		GridSize:             in.GridSize,
		Mode:                 in.Mode,
		RegionMap:            in.RegionMap,
		Solution:             in.Solution,
		Status:               in.Status,
		Difficulty:           in.Difficulty,
		MaxTier:              in.MaxTier,
		TierCounts:           in.TierCounts,
		TraceLen:             in.TraceLen,
		GenerationDurationMs: in.GenerationDurationMs,
		CreatedAt:            in.CreatedAt,
		Seed:                 in.Seed,
	}
	if err := s.store.PutPuzzle(ctx, rec); err != nil {
		return fmt.Errorf("storing puzzle %s: %w", in.ID, err)
	}
	return nil
}
