package status_test

import (
	"context"
	"errors"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/status"
)

type fakeStore struct {
	calls []call
	err   error
}

type call struct {
	pk, sk, status string
}

func (f *fakeStore) UpdateStatus(_ context.Context, pk, sk, st string) error {
	f.calls = append(f.calls, call{pk: pk, sk: sk, status: st})
	return f.err
}

func TestSetStatus_BuildsPKAndForwardsToStore(t *testing.T) {
	// Arrange
	store := &fakeStore{}
	svc := status.New(store)

	// Act
	err := svc.SetStatus(context.Background(), 7, "standard", "puzzle-123", "solved")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(store.calls))
	}
	got := store.calls[0]
	if got.pk != "7#standard" || got.sk != "puzzle-123" || got.status != "solved" {
		t.Errorf("call = %+v, want {7#standard puzzle-123 solved}", got)
	}
}

func TestSetStatus_TranslatesRepoNotFound(t *testing.T) {
	// Arrange
	store := &fakeStore{err: repository.ErrPuzzleNotFound}
	svc := status.New(store)

	// Act
	err := svc.SetStatus(context.Background(), 7, "standard", "puzzle-123", "solved")

	// Assert
	if !errors.Is(err, status.ErrPuzzleNotFound) {
		t.Errorf("err = %v, want status.ErrPuzzleNotFound", err)
	}
}

func TestSetStatus_WrapsOtherErrors(t *testing.T) {
	// Arrange
	sentinel := errors.New("ddb exploded")
	store := &fakeStore{err: sentinel}
	svc := status.New(store)

	// Act
	err := svc.SetStatus(context.Background(), 7, "standard", "puzzle-123", "solved")

	// Assert
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want wrap of sentinel", err)
	}
	if errors.Is(err, status.ErrPuzzleNotFound) {
		t.Error("err must not be ErrPuzzleNotFound when the underlying cause is different")
	}
}
