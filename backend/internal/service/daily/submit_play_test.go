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

// submitFakeStore is a configurable Store for SubmitPlay tests. Only
// WriteTransaction is exercised by SubmitPlay; all other methods are
// no-ops. A function field lets each test inject its own behaviour.
type submitFakeStore struct {
	writeTransactionFunc func(ctx context.Context, items []types.TransactWriteItem) error
}

func (f *submitFakeStore) GetSchedule(_ context.Context, _ string) (*repository.ScheduleRecord, error) {
	return nil, nil
}
func (f *submitFakeStore) GetPuzzle(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
	return nil, nil
}
func (f *submitFakeStore) GetPlay(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
	return nil, nil
}
func (f *submitFakeStore) GetCandidate(_ context.Context) (*repository.CandidateRecord, error) {
	return nil, nil
}
func (f *submitFakeStore) FinalizeDailyTransaction(_ context.Context, _, _, _ string, _ repository.FinalizeMode) error {
	return nil
}
func (f *submitFakeStore) WriteTransaction(ctx context.Context, items []types.TransactWriteItem) error {
	if f.writeTransactionFunc != nil {
		return f.writeTransactionFunc(ctx, items)
	}
	return nil
}
func (f *submitFakeStore) ListApprovedPool(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
	return nil, nil
}
func (f *submitFakeStore) ListReadyPoolNoDownvotes(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
	return nil, nil
}
func (f *submitFakeStore) PutCandidateIfAbsent(_ context.Context, _, _ string) error { return nil }
func (f *submitFakeStore) SubmitPlayTransactionally(_ context.Context, _, _ string, _ *repository.SubmitInput) error {
	return nil
}
func (f *submitFakeStore) LeaderboardRank(_ context.Context, _ string, _ int64, _ string) (int, error) {
	return 0, nil
}

// makeSubmitCancelErr builds a *types.TransactionCanceledException whose
// CancellationReasons have the given codes at each index. Pass
// "ConditionalCheckFailed" at the leg you want to trigger; pass "None"
// for legs that succeed. Matches the shape that
// repository.IsConditionalCheckFailureOnLeg inspects.
func makeSubmitCancelErr(codes ...string) error {
	reasons := make([]types.CancellationReason, len(codes))
	for i, code := range codes {
		c := code
		reasons[i] = types.CancellationReason{Code: &c}
	}
	return &types.TransactionCanceledException{CancellationReasons: reasons}
}

func TestSubmitPlay_SignedIn_HappyPath(t *testing.T) {
	// Arrange
	assignedAt := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	submittedAt := assignedAt.Add(10 * time.Second)

	var capturedItems []types.TransactWriteItem
	store := &submitFakeStore{
		writeTransactionFunc: func(_ context.Context, items []types.TransactWriteItem) error {
			capturedItems = items
			return nil
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	submission := &repository.SubmitInput{
		PuzzleID:    "puzzle-uuid-1",
		AssignedAt:  assignedAt,
		SubmittedAt: submittedAt,
		ClientMs:    9500,
		IsAnonymous: false,
		UserID:      "user_abc",
	}

	// Act
	err := svc.SubmitPlay(context.Background(), "player-1", "2026-05-16", submission)

	// Assert
	if err != nil {
		t.Fatalf("SubmitPlay(signed-in) unexpected error: %v", err)
	}
	if len(capturedItems) != 3 {
		t.Fatalf("signed-in submit: len(items) = %d, want 3", len(capturedItems))
	}

	// Leg 0 — Update PLAY row conditional on outcome=started.
	if capturedItems[0].Update == nil {
		t.Fatal("leg 0 missing Update")
	}
	leg0 := capturedItems[0].Update
	if leg0.ConditionExpression == nil || !strings.Contains(*leg0.ConditionExpression, ":started") {
		t.Errorf("leg 0 ConditionExpression must reference :started; got %v", leg0.ConditionExpression)
	}
	playPK := leg0.Key["PK"].(*types.AttributeValueMemberS).Value
	if playPK != "PLAY#player-1" {
		t.Errorf("leg 0 PK = %q, want PLAY#player-1", playPK)
	}
	playSK := leg0.Key["SK"].(*types.AttributeValueMemberS).Value
	if playSK != "DAILY#2026-05-16" {
		t.Errorf("leg 0 SK = %q, want DAILY#2026-05-16", playSK)
	}

	// Leg 1 — ADD counters.solved on the schedule row. Date derives from
	// assignedAt (play-origin date), not the submission date.
	if capturedItems[1].Update == nil {
		t.Fatal("leg 1 missing Update")
	}
	leg1 := capturedItems[1].Update
	schedPK := leg1.Key["PK"].(*types.AttributeValueMemberS).Value
	if schedPK != "DAILY#2026-05-16" {
		t.Errorf("leg 1 PK = %q, want DAILY#2026-05-16", schedPK)
	}
	schedSK := leg1.Key["SK"].(*types.AttributeValueMemberS).Value
	if schedSK != repository.DailySingletonSK {
		t.Errorf("leg 1 SK = %q, want %q", schedSK, repository.DailySingletonSK)
	}
	if leg1.UpdateExpression == nil || !strings.Contains(*leg1.UpdateExpression, "ADD") {
		t.Errorf("leg 1 UpdateExpression should be ADD; got %v", leg1.UpdateExpression)
	}

	// Leg 2 — Leaderboard Put (signed-in only).
	if capturedItems[2].Put == nil {
		t.Fatal("leg 2 missing Put")
	}
	leg2 := capturedItems[2].Put
	lbPK := leg2.Item["PK"].(*types.AttributeValueMemberS).Value
	if lbPK != "DAILY-LEADERBOARD#2026-05-16" {
		t.Errorf("leg 2 PK = %q, want DAILY-LEADERBOARD#2026-05-16", lbPK)
	}
	lbSK := leg2.Item["SK"].(*types.AttributeValueMemberS).Value
	wantSK := repository.BuildLeaderboardSK(10000, "user_abc")
	if lbSK != wantSK {
		t.Errorf("leg 2 SK = %q, want %q", lbSK, wantSK)
	}
	lbUserID := leg2.Item["userId"].(*types.AttributeValueMemberS).Value
	if lbUserID != "user_abc" {
		t.Errorf("leg 2 userId = %q, want user_abc", lbUserID)
	}
}

func TestSubmitPlay_Anonymous_HappyPath(t *testing.T) {
	// Arrange
	assignedAt := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	submittedAt := assignedAt.Add(10 * time.Second)

	var capturedItems []types.TransactWriteItem
	store := &submitFakeStore{
		writeTransactionFunc: func(_ context.Context, items []types.TransactWriteItem) error {
			capturedItems = items
			return nil
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	submission := &repository.SubmitInput{
		PuzzleID:    "puzzle-uuid-1",
		AssignedAt:  assignedAt,
		SubmittedAt: submittedAt,
		ClientMs:    9500,
		IsAnonymous: true,
		UserID:      "",
	}

	// Act
	err := svc.SubmitPlay(context.Background(), "device-xyz", "2026-05-16", submission)

	// Assert
	if err != nil {
		t.Fatalf("SubmitPlay(anonymous) unexpected error: %v", err)
	}
	if len(capturedItems) != 2 {
		t.Fatalf("anonymous submit: len(items) = %d, want 2 (no leaderboard leg)", len(capturedItems))
	}
	if capturedItems[0].Update == nil {
		t.Error("leg 0 should be Update (PLAY row)")
	}
	if capturedItems[1].Update == nil {
		t.Error("leg 1 should be Update (schedule counter)")
	}
}

func TestSubmitPlay_NegativeElapsedMs_RejectsBeforeTransaction(t *testing.T) {
	// Arrange — submittedAt is before assignedAt (clock skew / bad input).
	assignedAt := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	submittedAt := assignedAt.Add(-1 * time.Second)

	calls := 0
	store := &submitFakeStore{
		writeTransactionFunc: func(_ context.Context, _ []types.TransactWriteItem) error {
			calls++
			return nil
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	submission := &repository.SubmitInput{
		AssignedAt:  assignedAt,
		SubmittedAt: submittedAt,
		IsAnonymous: true,
	}

	// Act
	err := svc.SubmitPlay(context.Background(), "player-1", "2026-05-16", submission)

	// Assert
	if err == nil {
		t.Fatal("expected error for negative elapsed, got nil")
	}
	if calls != 0 {
		t.Errorf("WriteTransaction should not be called for negative elapsed; got %d calls", calls)
	}
}

func TestSubmitPlay_RaceLosser_ReturnsErrPlayNotInStartedState(t *testing.T) {
	// Arrange — leg 0 (PLAY Update) fails its condition, meaning the play
	// row is already solved (or missing). This is the idempotency guard.
	assignedAt := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	submittedAt := assignedAt.Add(10 * time.Second)

	store := &submitFakeStore{
		writeTransactionFunc: func(_ context.Context, _ []types.TransactWriteItem) error {
			// 2 legs for anonymous; leg 0 (PLAY Update) fails.
			return makeSubmitCancelErr("ConditionalCheckFailed", "None")
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	submission := &repository.SubmitInput{
		AssignedAt:  assignedAt,
		SubmittedAt: submittedAt,
		IsAnonymous: true,
	}

	// Act
	err := svc.SubmitPlay(context.Background(), "player-1", "2026-05-16", submission)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, repository.ErrPlayNotInStartedState) {
		t.Fatalf("error = %v, want repository.ErrPlayNotInStartedState", err)
	}
}

func TestSubmitPlay_GenericWriteError_WrapsError(t *testing.T) {
	// Arrange
	assignedAt := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	submittedAt := assignedAt.Add(10 * time.Second)

	ddbErr := errors.New("dynamodb throttled")
	store := &submitFakeStore{
		writeTransactionFunc: func(_ context.Context, _ []types.TransactWriteItem) error {
			return ddbErr
		},
	}
	svc := daily.New(store, "test-table", time.Now, nil)

	submission := &repository.SubmitInput{
		AssignedAt:  assignedAt,
		SubmittedAt: submittedAt,
		IsAnonymous: true,
	}

	// Act
	err := svc.SubmitPlay(context.Background(), "player-1", "2026-05-16", submission)

	// Assert
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, repository.ErrPlayNotInStartedState) {
		t.Fatalf("generic error must NOT map to ErrPlayNotInStartedState; got %v", err)
	}
	if !strings.Contains(err.Error(), "dynamodb throttled") {
		t.Errorf("error should wrap underlying cause; got %v", err)
	}
}
