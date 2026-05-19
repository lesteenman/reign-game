package daily_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/daily"
)

// fakeStore is a configurable Store implementation for FinalizeDaily
// tests. Only WriteTransaction is exercised by FinalizeDaily; the
// other methods are no-ops. Function fields let each test case inject
// its own behaviour without defining a full stub per test.
type fakeStore struct {
	writeTransactionFunc func(ctx context.Context, items []types.TransactWriteItem) error
}

func (f *fakeStore) GetSchedule(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
	return nil, nil
}
func (f *fakeStore) GetPuzzle(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
	return nil, nil
}
func (f *fakeStore) GetPlay(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
	return nil, nil
}
func (f *fakeStore) GetCandidate(_ context.Context) (*repository.CandidateRecord, error) {
	return nil, nil
}
func (f *fakeStore) FinalizeDailyTransaction(_ context.Context, _, _, _ string, _ repository.FinalizeMode) error {
	return nil
}
func (f *fakeStore) WriteTransaction(ctx context.Context, items []types.TransactWriteItem) error {
	if f.writeTransactionFunc != nil {
		return f.writeTransactionFunc(ctx, items)
	}
	return nil
}
func (f *fakeStore) ListApprovedPool(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
	return nil, nil
}
func (f *fakeStore) PutCandidateIfAbsent(_ context.Context, _, _ string) error { return nil }
func (f *fakeStore) SubmitPlayTransactionally(_ context.Context, _, _ string, _ *repository.SubmitInput) error {
	return nil
}
func (f *fakeStore) LeaderboardRank(_ context.Context, _ string, _ int64, _ string) (int, error) {
	return 0, nil
}

// makeCancelErr builds a *types.TransactionCanceledException whose
// CancellationReasons have the given codes at each index. Pass
// "ConditionalCheckFailed" at the leg you want to trigger; pass "None"
// for legs that succeed. Matches the shape that
// repository.IsConditionalCheckFailureOnLeg inspects.
func makeCancelErr(codes ...string) error {
	reasons := make([]types.CancellationReason, len(codes))
	for i, code := range codes {
		c := code
		reasons[i] = types.CancellationReason{Code: &c}
	}
	return &types.TransactionCanceledException{CancellationReasons: reasons}
}

func TestFinalizeDaily_ConfirmMode_HappyPath(t *testing.T) {
	// Arrange
	var capturedItems []types.TransactWriteItem
	store := &fakeStore{
		writeTransactionFunc: func(_ context.Context, items []types.TransactWriteItem) error {
			capturedItems = items
			return nil
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	// Act
	err := svc.FinalizeDaily(context.Background(), "2026-05-16", "puzzle-uuid-1", "9#standard", repository.FinalizeModeConfirm)

	// Assert
	if err != nil {
		t.Fatalf("FinalizeDaily(confirm) unexpected error: %v", err)
	}
	if len(capturedItems) != 3 {
		t.Fatalf("confirm mode: len(items) = %d, want 3", len(capturedItems))
	}
	// Leg 0 — Put schedule row.
	if capturedItems[0].Put == nil {
		t.Fatal("leg 0 missing Put")
	}
	pk := capturedItems[0].Put.Item["PK"].(*types.AttributeValueMemberS).Value
	if pk != "DAILY#2026-05-16" {
		t.Errorf("leg 0 PK = %q, want DAILY#2026-05-16", pk)
	}
	sk := capturedItems[0].Put.Item["SK"].(*types.AttributeValueMemberS).Value
	if sk != repository.DailySingletonSK {
		t.Errorf("leg 0 SK = %q, want %q", sk, repository.DailySingletonSK)
	}
	if capturedItems[0].Put.ConditionExpression == nil || *capturedItems[0].Put.ConditionExpression != "attribute_not_exists(PK)" {
		t.Errorf("leg 0 ConditionExpression should be attribute_not_exists(PK)")
	}
	// Leg 1 — Update PuzzleRecord lastDailyDate.
	if capturedItems[1].Update == nil {
		t.Fatal("leg 1 missing Update")
	}
	if capturedItems[1].Update.ConditionExpression != nil {
		t.Errorf("leg 1 must be unconditional; got ConditionExpression=%q", *capturedItems[1].Update.ConditionExpression)
	}
	updatePK := capturedItems[1].Update.Key["PK"].(*types.AttributeValueMemberS).Value
	if updatePK != "9#standard" {
		t.Errorf("leg 1 PK = %q, want 9#standard", updatePK)
	}
	updateSK := capturedItems[1].Update.Key["SK"].(*types.AttributeValueMemberS).Value
	if updateSK != "puzzle-uuid-1" {
		t.Errorf("leg 1 SK = %q, want puzzle-uuid-1", updateSK)
	}
	// Leg 2 — Delete candidate, conditional on puzzleId match.
	if capturedItems[2].Delete == nil {
		t.Fatal("leg 2 missing Delete")
	}
	delPK := capturedItems[2].Delete.Key["PK"].(*types.AttributeValueMemberS).Value
	if delPK != repository.DailyCandidatePK {
		t.Errorf("leg 2 PK = %q, want %q", delPK, repository.DailyCandidatePK)
	}
	delSK := capturedItems[2].Delete.Key["SK"].(*types.AttributeValueMemberS).Value
	if delSK != repository.DailySingletonSK {
		t.Errorf("leg 2 SK = %q, want %q", delSK, repository.DailySingletonSK)
	}
	if capturedItems[2].Delete.ConditionExpression == nil || !strings.Contains(*capturedItems[2].Delete.ConditionExpression, "puzzleId") {
		t.Errorf("leg 2 ConditionExpression must reference puzzleId; got %v", capturedItems[2].Delete.ConditionExpression)
	}
}

func TestFinalizeDaily_RecycleMode_HappyPath(t *testing.T) {
	// Arrange
	var capturedItems []types.TransactWriteItem
	store := &fakeStore{
		writeTransactionFunc: func(_ context.Context, items []types.TransactWriteItem) error {
			capturedItems = items
			return nil
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	// Act
	err := svc.FinalizeDaily(context.Background(), "2026-05-16", "puzzle-uuid-1", "9#standard", repository.FinalizeModeRecycle)

	// Assert
	if err != nil {
		t.Fatalf("FinalizeDaily(recycle) unexpected error: %v", err)
	}
	if len(capturedItems) != 2 {
		t.Fatalf("recycle mode: len(items) = %d, want 2 (no candidate-delete leg)", len(capturedItems))
	}
	if capturedItems[0].Put == nil {
		t.Error("recycle leg 0 should be Put (schedule row)")
	}
	if capturedItems[1].Update == nil {
		t.Error("recycle leg 1 should be Update (PuzzleRecord lastDailyDate)")
	}
}

func TestFinalizeDaily_InvalidMode_ReturnsErrorBeforeDDB(t *testing.T) {
	// Arrange
	calls := 0
	store := &fakeStore{
		writeTransactionFunc: func(_ context.Context, _ []types.TransactWriteItem) error {
			calls++
			return nil
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	// Act
	err := svc.FinalizeDaily(context.Background(), "2026-05-16", "puzzle-uuid-1", "9#standard", repository.FinalizeMode("bogus"))

	// Assert
	if err == nil {
		t.Fatal("expected error for invalid mode, got nil")
	}
	if calls != 0 {
		t.Errorf("WriteTransaction should not be called for invalid mode; got %d calls", calls)
	}
}

func TestFinalizeDaily_RaceLosser_ReturnsErrScheduleAlreadyFinalized(t *testing.T) {
	// Arrange — leg 0 (Put schedule row) fails its condition, meaning
	// another process already wrote the schedule row first.
	store := &fakeStore{
		writeTransactionFunc: func(_ context.Context, _ []types.TransactWriteItem) error {
			// 3 legs for confirm-mode; leg 0 (schedule Put) fails.
			return makeCancelErr("ConditionalCheckFailed", "None", "None")
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	// Act
	err := svc.FinalizeDaily(context.Background(), "2026-05-16", "puzzle-uuid-1", "9#standard", repository.FinalizeModeConfirm)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repository.ErrScheduleAlreadyFinalized) {
		t.Fatalf("error = %v, want repository.ErrScheduleAlreadyFinalized", err)
	}
}

func TestFinalizeDaily_GenericWriteError_WrapsError(t *testing.T) {
	// Arrange
	ddbErr := errors.New("dynamodb throttled")
	store := &fakeStore{
		writeTransactionFunc: func(_ context.Context, _ []types.TransactWriteItem) error {
			return ddbErr
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	// Act
	err := svc.FinalizeDaily(context.Background(), "2026-05-16", "puzzle-uuid-1", "9#standard", repository.FinalizeModeConfirm)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, repository.ErrScheduleAlreadyFinalized) {
		t.Fatalf("generic error must NOT map to ErrScheduleAlreadyFinalized; got %v", err)
	}
	if !strings.Contains(err.Error(), "dynamodb throttled") {
		t.Errorf("error should wrap underlying cause; got %v", err)
	}
}
