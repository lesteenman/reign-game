package serve_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/serve"
)

type fakeStore struct {
	nextReady  func(ctx context.Context, size int, mode string) (*repository.PuzzleRecord, error)
	markServed func(ctx context.Context, pk, sk string) error
}

func (f *fakeStore) NextReady(ctx context.Context, size int, mode string) (*repository.PuzzleRecord, error) {
	return f.nextReady(ctx, size, mode)
}

func (f *fakeStore) MarkServed(ctx context.Context, pk, sk string) error {
	return f.markServed(ctx, pk, sk)
}

func TestNextPuzzle_ReturnsPuzzleAndFiresReplenishOnHappyPath(t *testing.T) {
	// Arrange
	want := &repository.PuzzleRecord{ID: "p1", GridSize: 9, Mode: "standard"}
	store := &fakeStore{
		nextReady:  func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) { return want, nil },
		markServed: func(_ context.Context, _, _ string) error { return nil },
	}
	var (
		mu    sync.Mutex
		calls [][2]any
	)
	hook := func(size int, mode string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, [2]any{size, mode})
	}
	svc := serve.New(store, hook)

	// Act
	got, err := svc.NextPuzzle(context.Background(), 9, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got puzzle = %+v, want %+v", got, want)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 || calls[0][0] != 9 || calls[0][1] != "standard" {
		t.Errorf("replenish hook calls = %+v, want [{9 standard}]", calls)
	}
}

func TestNextPuzzle_ReturnsNilWhenPoolIsEmpty(t *testing.T) {
	// Arrange
	store := &fakeStore{
		nextReady:  func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) { return nil, nil },
		markServed: func(_ context.Context, _, _ string) error { return nil },
	}
	hookCalled := false
	svc := serve.New(store, func(int, string) { hookCalled = true })

	// Act
	got, err := svc.NextPuzzle(context.Background(), 9, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got puzzle = %+v, want nil", got)
	}
	if hookCalled {
		t.Error("replenish hook must not fire when pool is empty")
	}
}

func TestNextPuzzle_ReturnsNilOnMarkServedRace(t *testing.T) {
	// Arrange — NextReady finds a row but MarkServed loses the race
	// with another replica that claimed it first.
	puzzle := &repository.PuzzleRecord{ID: "p1", GridSize: 9, Mode: "standard"}
	store := &fakeStore{
		nextReady:  func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) { return puzzle, nil },
		markServed: func(_ context.Context, _, _ string) error { return repository.ErrPuzzleNotFound },
	}
	hookCalled := false
	svc := serve.New(store, func(int, string) { hookCalled = true })

	// Act
	got, err := svc.NextPuzzle(context.Background(), 9, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got puzzle = %+v, want nil on race", got)
	}
	if hookCalled {
		t.Error("replenish hook must not fire when MarkServed loses the race")
	}
}

func TestNextPuzzle_WrapsNextReadyErrors(t *testing.T) {
	// Arrange
	sentinel := errors.New("ddb query exploded")
	store := &fakeStore{
		nextReady:  func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) { return nil, sentinel },
		markServed: func(_ context.Context, _, _ string) error { return nil },
	}
	svc := serve.New(store, nil)

	// Act
	got, err := svc.NextPuzzle(context.Background(), 9, "standard")

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil on error", got)
	}
}

func TestNextPuzzle_WrapsNonRaceMarkServedErrors(t *testing.T) {
	// Arrange
	sentinel := errors.New("ddb update exploded")
	puzzle := &repository.PuzzleRecord{ID: "p1", GridSize: 9, Mode: "standard"}
	store := &fakeStore{
		nextReady:  func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) { return puzzle, nil },
		markServed: func(_ context.Context, _, _ string) error { return sentinel },
	}
	hookCalled := false
	svc := serve.New(store, func(int, string) { hookCalled = true })

	// Act
	got, err := svc.NextPuzzle(context.Background(), 9, "standard")

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil on error", got)
	}
	if hookCalled {
		t.Error("replenish hook must not fire when MarkServed errors")
	}
}

func TestNextPuzzle_NilReplenishIsHandledGracefully(t *testing.T) {
	// Arrange
	puzzle := &repository.PuzzleRecord{ID: "p1", GridSize: 9, Mode: "standard"}
	store := &fakeStore{
		nextReady:  func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) { return puzzle, nil },
		markServed: func(_ context.Context, _, _ string) error { return nil },
	}
	svc := serve.New(store, nil)

	// Act
	got, err := svc.NextPuzzle(context.Background(), 9, "standard")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != puzzle {
		t.Errorf("got = %+v, want puzzle", got)
	}
}
