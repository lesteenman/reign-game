package daily_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
	"github.com/eriksteenman/reign-game/backend/internal/service/daily"
)

// anticheatPuzzle returns the standard 2×2 submit puzzle with an explicit
// difficulty so the per-difficulty floor table applies.
func anticheatPuzzle(difficulty int) *repository.PuzzleRecord {
	p := submitTestPuzzle()
	p.Difficulty = difficulty
	return p
}

// playAssignedAt returns a started PLAY row whose assignedAt is `elapsed`
// before testNow, so SubmitDaily computes serverElapsedMs == elapsed.
func playAssignedAt(playerID string, elapsed time.Duration) *repository.PlayRecord {
	play := submitTestPlay(playerID)
	play.AssignedAt = testNow.Add(-elapsed).Format(time.RFC3339)
	return play
}

// captureSubmit runs SubmitDaily against a store that records the
// transaction legs and returns them alongside the result/error.
func captureSubmit(t *testing.T, puzzle *repository.PuzzleRecord, play *repository.PlayRecord, in daily.SubmitInput) ([]types.TransactWriteItem, *daily.SubmitResult, error) {
	t.Helper()
	var captured []types.TransactWriteItem
	store := buildSubmitStore(&submitDailyFakeStore{
		getPuzzleFunc: func(_ context.Context, _ int, _, _ string) (*repository.PuzzleRecord, error) {
			return puzzle, nil
		},
		getPlayFunc: func(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
			return play, nil
		},
		writeTransactionFunc: func(_ context.Context, items []types.TransactWriteItem) error {
			captured = items
			return nil
		},
	})
	svc := daily.New(store, "test-table", fixedClock(testNow), nil)
	result, err := svc.SubmitDaily(context.Background(), in)
	return captured, result, err
}

// playRowUpdate returns leg 0's PLAY Update (the conditional outcome=solved leg).
func playRowUpdate(t *testing.T, items []types.TransactWriteItem) *types.Update {
	t.Helper()
	if len(items) == 0 || items[0].Update == nil {
		t.Fatalf("expected leg 0 to be a PLAY Update; got %#v", items)
	}
	return items[0].Update
}

// hasLeaderboardLeg reports whether any captured leg is a leaderboard Put.
func hasLeaderboardLeg(items []types.TransactWriteItem) bool {
	for _, it := range items {
		if it.Put == nil {
			continue
		}
		if pk, ok := it.Put.Item["PK"].(*types.AttributeValueMemberS); ok {
			if len(pk.Value) >= len("DAILY-LEADERBOARD#") && pk.Value[:len("DAILY-LEADERBOARD#")] == "DAILY-LEADERBOARD#" {
				return true
			}
		}
	}
	return false
}

// cheatFlagValue extracts the cheatFlag attribute from the PLAY Update's
// expression values, or "" when absent.
func cheatFlagValue(u *types.Update) string {
	// The PLAY update sets cheatFlag via :cheatFlag in ExpressionAttributeValues.
	if v, ok := u.ExpressionAttributeValues[":cheatFlag"].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func TestSubmitDaily_BelowFloor_FlaggedExcludedStillSolved(t *testing.T) {
	// Arrange — Medium puzzle (floor 8000ms), solved in 5000ms.
	puzzle := anticheatPuzzle(2)
	play := playAssignedAt("player-1", 5*time.Second)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  5000,
		Solution:    validSolution,
	}

	// Act
	items, result, err := captureSubmit(t, puzzle, play, in)

	// Assert — solve succeeds, response is the normal shape.
	if err != nil {
		t.Fatalf("SubmitDaily(below-floor) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result (silent exclusion)")
	}
	// No leaderboard leg.
	if hasLeaderboardLeg(items) {
		t.Error("below-floor submit must NOT write a leaderboard leg")
	}
	// PLAY row still solved + carries the flag.
	u := playRowUpdate(t, items)
	if got := cheatFlagValue(u); got != "belowFloor" {
		t.Errorf("PLAY cheatFlag = %q, want belowFloor", got)
	}
	if got := u.ExpressionAttributeValues[":solved"].(*types.AttributeValueMemberS).Value; got != repository.PlayOutcomeSolved {
		t.Errorf("PLAY outcome value = %q, want %q", got, repository.PlayOutcomeSolved)
	}
}

func TestSubmitDaily_Divergence_FlaggedExcluded(t *testing.T) {
	// Arrange — Medium puzzle solved in 20000ms (above floor). Client claims
	// 9000ms → divergence 11000 > tolerance max(2000, 10000) = 10000.
	puzzle := anticheatPuzzle(2)
	play := playAssignedAt("player-1", 20*time.Second)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  9000,
		Solution:    validSolution,
	}

	// Act
	items, result, err := captureSubmit(t, puzzle, play, in)

	// Assert
	if err != nil {
		t.Fatalf("SubmitDaily(divergence) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if hasLeaderboardLeg(items) {
		t.Error("divergence submit must NOT write a leaderboard leg")
	}
	u := playRowUpdate(t, items)
	if got := cheatFlagValue(u); got != "timeDivergence" {
		t.Errorf("PLAY cheatFlag = %q, want timeDivergence", got)
	}
}

func TestSubmitDaily_NormalSolve_RanksUnflagged(t *testing.T) {
	// Arrange — Medium puzzle solved in 20000ms, client claims 20000ms.
	puzzle := anticheatPuzzle(2)
	play := playAssignedAt("player-1", 20*time.Second)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  20000,
		Solution:    validSolution,
	}

	// Act
	items, result, err := captureSubmit(t, puzzle, play, in)

	// Assert — ranks as today: leaderboard leg present, no flag.
	if err != nil {
		t.Fatalf("SubmitDaily(normal) unexpected error: %v", err)
	}
	if result == nil || result.LeaderboardRank == nil {
		t.Fatalf("normal signed-in solve should return a leaderboard rank; got %#v", result)
	}
	if !hasLeaderboardLeg(items) {
		t.Error("normal solve must write a leaderboard leg")
	}
	u := playRowUpdate(t, items)
	if got := cheatFlagValue(u); got != "" {
		t.Errorf("normal solve PLAY cheatFlag = %q, want empty", got)
	}
}

func TestSubmitDaily_BoundaryAtFloor_NotFlagged(t *testing.T) {
	// Arrange — Hard puzzle (floor 12000ms) solved in exactly 12000ms,
	// client matches. At the floor is NOT below it.
	puzzle := anticheatPuzzle(3)
	play := playAssignedAt("player-1", 12*time.Second)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  12000,
		Solution:    validSolution,
	}

	// Act
	items, _, err := captureSubmit(t, puzzle, play, in)

	// Assert
	if err != nil {
		t.Fatalf("SubmitDaily(at-floor) unexpected error: %v", err)
	}
	if !hasLeaderboardLeg(items) {
		t.Error("exactly-at-floor solve must rank (leaderboard leg present)")
	}
	u := playRowUpdate(t, items)
	if got := cheatFlagValue(u); got != "" {
		t.Errorf("at-floor PLAY cheatFlag = %q, want empty", got)
	}
}

func TestSubmitDaily_ClientClaimedZero_NoDivergenceFlag(t *testing.T) {
	// Arrange — unknown difficulty (no floor), 20000ms solve, client claims 0
	// (older client). Divergence check must be skipped → unflagged + ranks.
	puzzle := anticheatPuzzle(0)
	play := playAssignedAt("player-1", 20*time.Second)
	in := daily.SubmitInput{
		PlayerID:    "player-1",
		IsAnonymous: false,
		Date:        testDate,
		PlayTimeMs:  0,
		Solution:    validSolution,
	}

	// Act
	items, _, err := captureSubmit(t, puzzle, play, in)

	// Assert
	if err != nil {
		t.Fatalf("SubmitDaily(client-zero) unexpected error: %v", err)
	}
	if !hasLeaderboardLeg(items) {
		t.Error("client-claimed-zero solve must rank (no divergence flag)")
	}
	u := playRowUpdate(t, items)
	if got := cheatFlagValue(u); got != "" {
		t.Errorf("client-zero PLAY cheatFlag = %q, want empty", got)
	}
}

func TestSubmitDaily_AnonymousBelowFloor_FlagRecordedNoLeaderboard(t *testing.T) {
	// Arrange — anonymous below-floor solve. Anonymous already skips the
	// leaderboard; the flag must still record on the PLAY row.
	puzzle := anticheatPuzzle(2)
	play := playAssignedAt("device-xyz", 5*time.Second)
	in := daily.SubmitInput{
		PlayerID:    "device-xyz",
		IsAnonymous: true,
		Date:        testDate,
		PlayTimeMs:  5000,
		Solution:    validSolution,
	}

	// Act
	items, result, err := captureSubmit(t, puzzle, play, in)

	// Assert
	if err != nil {
		t.Fatalf("SubmitDaily(anon below-floor) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if hasLeaderboardLeg(items) {
		t.Error("anonymous submit must never write a leaderboard leg")
	}
	u := playRowUpdate(t, items)
	if got := cheatFlagValue(u); got != "belowFloor" {
		t.Errorf("anonymous PLAY cheatFlag = %q, want belowFloor", got)
	}
}
