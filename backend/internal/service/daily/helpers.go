package daily

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// dailyDateLayout is the canonical YYYY-MM-DD format expected by the
// daily endpoints. Dates outside this layout are rejected with
// ErrInvalidDate.
const dailyDateLayout = "2006-01-02"

// fetchScheduleAndPlay runs GetSchedule and GetPlay concurrently. They
// have no dependency — both are keyed off (date, playerID) alone — so
// fanning them out saves one DDB RTT per request on the daily hot path.
// Returns (schedule, play, scheduleErr, playErr) so callers can surface
// different status codes for each path.
func fetchScheduleAndPlay(
	ctx context.Context,
	store Store,
	date, playerID string,
) (*repository.ScheduleRecord, *repository.PlayRecord, error, error) {
	var (
		schedule *repository.ScheduleRecord
		play     *repository.PlayRecord
		sErr     error
		pErr     error
		wg       sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		schedule, sErr = store.GetSchedule(ctx, date)
	}()
	go func() {
		defer wg.Done()
		play, pErr = store.GetPlay(ctx, playerID, date)
	}()
	wg.Wait()
	return schedule, play, sErr, pErr
}

// materializePlayRow returns the PLAY row for (playerID, date),
// creating it on first GET. On the race-loser branch it re-reads the
// row so the caller surfaces the winner's assignedAt — the
// "assignedAt is set once, never overwritten" invariant lives here.
// existingPlay is the result of an upstream GetPlay (typically from
// fetchScheduleAndPlay's parallel fan-out); when non-nil the function
// short-circuits and returns it directly, avoiding a second DDB read.
// playMs is the wall-clock cost of any PLAY-related DDB calls this
// function issues itself (zero on the cache-hit path).
func materializePlayRow(
	ctx context.Context,
	store Store,
	existingPlay *repository.PlayRecord,
	playerID, date, puzzleID string,
	clock func() time.Time,
) (*repository.PlayRecord, int64, error) {
	if existingPlay != nil {
		return existingPlay, 0, nil
	}

	playStart := time.Now()
	now := clock().UTC()
	putErr := store.PutPlayStartedIfAbsent(ctx, playerID, date, puzzleID, now)
	if putErr == nil {
		return &repository.PlayRecord{
			PlayerID:   playerID,
			Date:       date,
			Outcome:    repository.PlayOutcomeStarted,
			AssignedAt: now.Format(time.RFC3339),
			PuzzleID:   puzzleID,
		}, time.Since(playStart).Milliseconds(), nil
	}
	if !errors.Is(putErr, repository.ErrPlayAlreadyExists) {
		return nil, time.Since(playStart).Milliseconds(), putErr
	}

	// Race-loser: another request created the row between our GetPlay
	// and our PutPlayStartedIfAbsent. Re-read so the caller surfaces
	// the winner's assignedAt.
	winner, err := store.GetPlay(ctx, playerID, date)
	if err != nil {
		return nil, time.Since(playStart).Milliseconds(), err
	}
	if winner == nil {
		// Should not happen — the conditional put failed because the
		// row exists, so the follow-up read should hit it.
		return nil, time.Since(playStart).Milliseconds(), errors.New("play row vanished after race-loser conditional fail")
	}
	return winner, time.Since(playStart).Milliseconds(), nil
}

// parseSourcePartition splits "{size}#{mode}" into its components.
// Returns an error on shapes that can't be parsed; callers map to 500
// because the value is server-controlled (written by the cron) and a
// malformed value is a system-invariant violation, not user input.
func parseSourcePartition(sp string) (size int, mode string, err error) {
	parts := strings.SplitN(sp, "#", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, "", errors.New("sourcePartition must be {size}#{mode}")
	}
	size, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", err
	}
	return size, parts[1], nil
}

// solutionShapeValid checks that solution is a non-empty rectangular
// grid whose cells are 0 or 1. Returns false for empty grids,
// jagged rows, or out-of-range cells. Mismatch against the puzzle's
// expected solution is checked separately by solutionMatches once we
// know the puzzle's actual size.
func solutionShapeValid(solution [][]int) bool {
	if len(solution) == 0 {
		return false
	}
	width := len(solution[0])
	if width == 0 {
		return false
	}
	for _, row := range solution {
		if len(row) != width {
			return false
		}
		for _, cell := range row {
			if cell != 0 && cell != 1 {
				return false
			}
		}
	}
	return true
}

// solutionMatches returns true iff the player's submitted grid equals
// the puzzle's expected solution: same dimensions, and each submitted
// 1 corresponds to expected true (0 ↔ false).
func solutionMatches(submitted [][]int, expected [][]bool) bool {
	if len(submitted) != len(expected) {
		return false
	}
	for i, row := range submitted {
		if len(row) != len(expected[i]) {
			return false
		}
		for j, cell := range row {
			want := expected[i][j]
			if (cell == 1) != want {
				return false
			}
		}
	}
	return true
}

// truncatePlayer keeps the first 8 chars of playerID for log lines.
// The full ID can be a deviceId (UUID-shaped) and we don't want those
// in plaintext logs by accident.
func truncatePlayer(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
