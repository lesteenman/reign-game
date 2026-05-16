package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/daily"
)

type fakeService struct {
	ensureErr      error
	ensureCalled   bool
	finalizeReturn *repository.ScheduleRecord
	finalizeErr    error
	finalizeCalled bool
	finalizeArgs   struct{ today, yesterday string }
}

func (f *fakeService) EnsureCandidate(_ context.Context, _ time.Time) error {
	f.ensureCalled = true
	return f.ensureErr
}

func (f *fakeService) SyncFinalizeForToday(_ context.Context, today, yesterday string, _ time.Time) (*repository.ScheduleRecord, error) {
	f.finalizeCalled = true
	f.finalizeArgs.today = today
	f.finalizeArgs.yesterday = yesterday
	return f.finalizeReturn, f.finalizeErr
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
}

func TestHandle_EnsureSuccess(t *testing.T) {
	// Arrange
	svc := &fakeService{}
	// Act
	err := handle(context.Background(), EventBridgeScheduledEvent{DetailType: DetailTypeEnsure}, svc, fixedClock)
	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.ensureCalled {
		t.Fatalf("EnsureCandidate not called")
	}
	if svc.finalizeCalled {
		t.Fatalf("Finalize must not be called for ensure event")
	}
}

func TestHandle_EnsurePoolEmpty_NilReturn(t *testing.T) {
	// Arrange — sentinel returns nil at the dispatch boundary.
	svc := &fakeService{ensureErr: daily.ErrCandidatePoolEmpty}
	// Act
	err := handle(context.Background(), EventBridgeScheduledEvent{DetailType: DetailTypeEnsure}, svc, fixedClock)
	// Assert
	if err != nil {
		t.Fatalf("ErrCandidatePoolEmpty must surface as nil; got %v", err)
	}
}

func TestHandle_EnsureGenericError_Surfaces(t *testing.T) {
	// Arrange
	plain := errors.New("ddb broken")
	svc := &fakeService{ensureErr: plain}
	// Act
	err := handle(context.Background(), EventBridgeScheduledEvent{DetailType: DetailTypeEnsure}, svc, fixedClock)
	// Assert — non-sentinel errors propagate so EventBridge retries.
	if !errors.Is(err, plain) {
		t.Fatalf("expected wrapped %v, got %v", plain, err)
	}
}

func TestHandle_FinalizeSuccess_DateArguments(t *testing.T) {
	// Arrange
	svc := &fakeService{finalizeReturn: &repository.ScheduleRecord{PuzzleID: "p1", SourcePartition: "9#standard"}}
	// Act
	err := handle(context.Background(), EventBridgeScheduledEvent{DetailType: DetailTypeFinalize}, svc, fixedClock)
	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.finalizeArgs.today != "2026-05-02" {
		t.Fatalf("today=%s", svc.finalizeArgs.today)
	}
	if svc.finalizeArgs.yesterday != "2026-05-01" {
		t.Fatalf("yesterday=%s", svc.finalizeArgs.yesterday)
	}
}

func TestHandle_FinalizePoolExhausted_NilReturn(t *testing.T) {
	// Arrange
	svc := &fakeService{finalizeErr: daily.ErrPoolExhausted}
	// Act
	err := handle(context.Background(), EventBridgeScheduledEvent{DetailType: DetailTypeFinalize}, svc, fixedClock)
	// Assert
	if err != nil {
		t.Fatalf("pool-exhausted must surface as nil; got %v", err)
	}
}

func TestHandle_UnknownDetailType_NilReturn(t *testing.T) {
	// Arrange
	svc := &fakeService{}
	// Act
	err := handle(context.Background(), EventBridgeScheduledEvent{DetailType: "unknown-thing"}, svc, fixedClock)
	// Assert — log + return nil, no retry.
	if err != nil {
		t.Fatalf("unknown detail-type must return nil; got %v", err)
	}
	if svc.ensureCalled || svc.finalizeCalled {
		t.Fatalf("no service method should be called for unknown event")
	}
}
