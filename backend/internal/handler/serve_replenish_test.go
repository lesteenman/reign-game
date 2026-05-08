package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// hookCall records one invocation of the replenish hook so tests can
// assert it was called with the right (size, mode) on the right paths.
type hookCall struct {
	size int
	mode string
}

// captureReplenishHook returns a hook function that appends each call
// to `calls`. Mutex-guarded for the goroutine-dispatched callsite.
func captureReplenishHook(calls *[]hookCall, mu *sync.Mutex) func(int, string) {
	return func(size int, mode string) {
		mu.Lock()
		*calls = append(*calls, hookCall{size: size, mode: mode})
		mu.Unlock()
	}
}

func TestServeHandler_FiresReplenishHookAfterSuccessfulServe(t *testing.T) {
	// Arrange
	puzzle := &repository.PuzzleRecord{
		GridSize: 9,
		Mode:     "standard",
		ID:       "puzzle-uuid-9std",
	}
	fetcher := &mockPuzzleFetcher{
		nextReadyFunc: func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) {
			return puzzle, nil
		},
		markServedFunc: func(_ context.Context, _ string, _ string) error { return nil },
	}
	var calls []hookCall
	var mu sync.Mutex
	h := handler.ServeHandler(fetcher, captureReplenishHook(&calls, &mu))

	req := httptest.NewRequest(http.MethodGet, "/puzzles/next?size=9&mode=standard", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	h.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 {
		t.Fatalf("hook calls = %d, want 1", len(calls))
	}
	if calls[0].size != 9 || calls[0].mode != "standard" {
		t.Errorf("hook call = %+v, want {9 standard}", calls[0])
	}
}

func TestServeHandler_DoesNotFireReplenishHookOnNoPuzzle(t *testing.T) {
	// Arrange — NextReady returns no puzzle; no MarkServed; no hook.
	fetcher := &mockPuzzleFetcher{
		nextReadyFunc:  func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) { return nil, nil },
		markServedFunc: func(_ context.Context, _ string, _ string) error { return nil },
	}
	var calls []hookCall
	var mu sync.Mutex
	h := handler.ServeHandler(fetcher, captureReplenishHook(&calls, &mu))

	req := httptest.NewRequest(http.MethodGet, "/puzzles/next?size=9&mode=standard", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	h.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 0 {
		t.Errorf("hook must not fire on no-puzzle path; got %d calls", len(calls))
	}
}

func TestServeHandler_DoesNotFireReplenishHookOnMarkServedFailure(t *testing.T) {
	// Arrange — NextReady returns a puzzle but MarkServed errors.
	puzzle := &repository.PuzzleRecord{GridSize: 7, Mode: "standard", ID: "p"}
	fetcher := &mockPuzzleFetcher{
		nextReadyFunc:  func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) { return puzzle, nil },
		markServedFunc: func(_ context.Context, _ string, _ string) error { return errors.New("ddb broken") },
	}
	var calls []hookCall
	var mu sync.Mutex
	h := handler.ServeHandler(fetcher, captureReplenishHook(&calls, &mu))

	req := httptest.NewRequest(http.MethodGet, "/puzzles/next?size=7&mode=standard", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	h.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 0 {
		t.Errorf("hook must not fire when MarkServed fails; got %d calls", len(calls))
	}
}

func TestServeHandler_NilReplenishHookIsHandledGracefully(t *testing.T) {
	// Arrange — passing nil hook is the existing-callers compatibility
	// path; the handler must not panic.
	puzzle := &repository.PuzzleRecord{GridSize: 9, Mode: "standard", ID: "p"}
	fetcher := &mockPuzzleFetcher{
		nextReadyFunc:  func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) { return puzzle, nil },
		markServedFunc: func(_ context.Context, _ string, _ string) error { return nil },
	}
	h := handler.ServeHandler(fetcher, nil)

	req := httptest.NewRequest(http.MethodGet, "/puzzles/next?size=9&mode=standard", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	h.ServeHTTP(rec, req)

	// Assert — nil hook must not panic; success still 200.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}
