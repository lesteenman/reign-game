package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	"github.com/eriksteenman/reign-game/backend/internal/daily"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// poolExhaustedMessage is the canonical 500 body phrase emitted when
// sync-fallback hits daily.ErrPoolExhausted (DP-16). Stable phrase: a
// future R-8-02 frontend will key off it for a "we're working on it"
// graceful UI. Keep in sync with daily_test.go's poolExhaustedErrorBody.
const poolExhaustedMessage = "pool exhausted"

// dailyDateLayout is the canonical YYYY-MM-DD format expected by
// GET /api/daily/{date}. Dates outside this layout are rejected with
// 400 per DP-07.
const dailyDateLayout = "2006-01-02"

// dailyDeviceHeader is the header carrying the anonymous device
// identifier when no Clerk session cookie is present (DP-08).
const dailyDeviceHeader = "X-Device-Id"

// DailyRepo is the narrow surface the GET handler needs from the
// repository layer. Decoupling on a small interface lets tests use a
// hand-rolled fake without LocalStack and keeps the production
// repository (*repository.PuzzleRepository) trivially compliant.
type DailyRepo interface {
	GetSchedule(ctx context.Context, date string) (*repository.ScheduleRecord, error)
	GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error)
	GetPlay(ctx context.Context, playerID, date string) (*repository.PlayRecord, error)
	PutPlayStartedIfAbsent(ctx context.Context, playerID, date, puzzleID string, assignedAt time.Time) error

	// GetCandidate + FinalizeDailyTransaction are required by the
	// sync-fallback path (DP-05). DailyRepo is a structural superset of
	// daily.Repo so a *DailyRepo value can be passed straight into
	// daily.SyncFinalizeForToday.
	GetCandidate(ctx context.Context) (*repository.CandidateRecord, error)
	FinalizeDailyTransaction(ctx context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error
}

// dailyResponse is the GET 200 response shape (DP-09). ServerElapsedMs
// and SubmittedAt are pointer-typed so `omitempty` drops them entirely
// when outcome != solved — frontends that pre-render started/solved
// branches can rely on the field's presence as the discriminator.
type dailyResponse struct {
	PuzzleID        string  `json:"puzzleId"`
	Grid            int     `json:"grid"`
	Regions         [][]int `json:"regions"`
	AssignedAt      string  `json:"assignedAt"`
	Outcome         string  `json:"outcome"`
	ServerElapsedMs *int64  `json:"serverElapsedMs,omitempty"`
	SubmittedAt     *string `json:"submittedAt,omitempty"`
}

// DailyGetHandler creates an HTTP handler for GET /api/daily/{date}.
//
// Flow (chunk 2):
//  1. auth + date window (chunk 1, unchanged)
//  2. schedule read — 404 when absent (sync fallback is chunk 3)
//  3. parse sourcePartition → size+mode → GetPuzzle
//  4. PLAY-row materialize: GetPlay → PutPlayStartedIfAbsent on miss,
//     re-GetPlay on race-loser; never overwrite assignedAt (DP-19)
//  5. emit DP-09 JSON
//
// The clock argument lets tests pin "today" for the
// [yesterdayUTC, todayUTC] window check; production callers pass
// time.Now.
func DailyGetHandler(repo DailyRepo, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		start := time.Now()

		playerID, isAnonymous, ok := resolveDailyPlayer(r)
		if !ok {
			writeDailyError(w, http.StatusUnauthorized, "unauthenticated")
			log.Printf("daily get: 401 unauthenticated path=%s", r.URL.Path)
			return
		}

		date := chi.URLParam(r, "date")
		parsed, err := time.ParseInLocation(dailyDateLayout, date, time.UTC)
		if err != nil {
			writeDailyError(w, http.StatusBadRequest, "invalid date")
			log.Printf("daily get: 400 invalid_date date=%q player=%s anon=%t", date, playerID, isAnonymous)
			return
		}

		now := clock().UTC()
		todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		yesterdayUTC := todayUTC.AddDate(0, 0, -1)
		todayStr := todayUTC.Format(dailyDateLayout)
		yesterdayStr := yesterdayUTC.Format(dailyDateLayout)

		requested := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		if requested.Before(yesterdayUTC) || requested.After(todayUTC) {
			writeDailyError(w, http.StatusNotFound, "out of window")
			log.Printf("daily get: 404 out_of_window date=%s player=%s anon=%t", date, playerID, isAnonymous)
			return
		}

		ctx := r.Context()

		scheduleStart := time.Now()
		schedule, err := repo.GetSchedule(ctx, date)
		scheduleMs := time.Since(scheduleStart).Milliseconds()
		if err != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily get: 500 schedule_read_failed date=%s err=%v", date, err)
			return
		}

		var syncMs int64
		if schedule == nil {
			// DP-05: sync-fallback engages ONLY for today. Yesterday's
			// schedule should always exist by the time today is being
			// requested — if it doesn't, the system is in an
			// unrecoverable state and we 404 rather than attempt to
			// retro-finalize a past day.
			if date != todayStr {
				writeDailyError(w, http.StatusNotFound, "schedule not finalized")
				log.Printf("daily get: 404 schedule_absent date=%s player=%s", date, truncatePlayer(playerID))
				return
			}
			syncStart := time.Now()
			finalized, syncErr := daily.SyncFinalizeForToday(ctx, repo, todayStr, yesterdayStr, clock())
			syncMs = time.Since(syncStart).Milliseconds()
			if errors.Is(syncErr, daily.ErrPoolExhausted) {
				writeDailyError(w, http.StatusInternalServerError, poolExhaustedMessage)
				log.Printf("daily get: 500 pool_exhausted date=%s player=%s sync_ms=%d", date, truncatePlayer(playerID), syncMs)
				return
			}
			if syncErr != nil {
				writeDailyError(w, http.StatusInternalServerError, "internal error")
				log.Printf("daily get: 500 sync_finalize_failed date=%s sync_ms=%d err=%v", date, syncMs, syncErr)
				return
			}
			schedule = finalized
		}

		size, mode, err := parseSourcePartition(schedule.SourcePartition)
		if err != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily get: 500 source_partition_malformed date=%s sourcePartition=%q err=%v", date, schedule.SourcePartition, err)
			return
		}

		puzzleStart := time.Now()
		puzzle, err := repo.GetPuzzle(ctx, size, mode, schedule.PuzzleID)
		puzzleMs := time.Since(puzzleStart).Milliseconds()
		if err != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily get: 500 puzzle_read_failed date=%s puzzleId=%s err=%v", date, schedule.PuzzleID, err)
			return
		}
		if puzzle == nil {
			// Schedule pointed at a puzzle that does not exist — broken
			// invariant. Don't 404, because that would let a corrupted
			// schedule masquerade as "no daily today".
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily get: 500 puzzle_missing date=%s puzzleId=%s", date, schedule.PuzzleID)
			return
		}

		play, playMs, err := materializePlayRow(ctx, repo, playerID, date, schedule.PuzzleID, clock)
		if err != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily get: 500 play_materialize_failed date=%s player=%s err=%v", date, truncatePlayer(playerID), err)
			return
		}

		resp := dailyResponse{
			PuzzleID:   schedule.PuzzleID,
			Grid:       puzzle.GridSize,
			Regions:    puzzle.RegionMap,
			AssignedAt: play.AssignedAt,
			Outcome:    play.Outcome,
		}
		if play.Outcome == repository.PlayOutcomeSolved {
			elapsed := play.ServerElapsedMs
			submittedAt := play.SubmittedAt
			resp.ServerElapsedMs = &elapsed
			resp.SubmittedAt = &submittedAt
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("daily get: response write failed: %v", err)
			return
		}

		totalMs := time.Since(start).Milliseconds()
		log.Printf("daily get: total_ms=%d schedule_ms=%d sync_ms=%d puzzle_ms=%d play_ms=%d path=%s player=%s",
			totalMs, scheduleMs, syncMs, puzzleMs, playMs, r.URL.Path, truncatePlayer(playerID))
	})
}

// materializePlayRow returns the PLAY row for (playerID, date),
// creating it on first GET. On the race-loser branch it re-reads the
// row so the caller surfaces the winner's assignedAt — DP-19's
// "assignedAt is set once, never overwritten" invariant lives here.
// playMs is the wall-clock cost of every PLAY-related DDB call so the
// per-step timing log can be a single bucket.
func materializePlayRow(
	ctx context.Context,
	repo DailyRepo,
	playerID, date, puzzleID string,
	clock func() time.Time,
) (*repository.PlayRecord, int64, error) {
	playStart := time.Now()

	existing, err := repo.GetPlay(ctx, playerID, date)
	if err != nil {
		return nil, time.Since(playStart).Milliseconds(), err
	}
	if existing != nil {
		return existing, time.Since(playStart).Milliseconds(), nil
	}

	now := clock().UTC()
	putErr := repo.PutPlayStartedIfAbsent(ctx, playerID, date, puzzleID, now)
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
	// the winner's assignedAt (DP-19).
	winner, err := repo.GetPlay(ctx, playerID, date)
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
func parseSourcePartition(sp string) (int, string, error) {
	parts := strings.SplitN(sp, "#", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, "", errors.New("sourcePartition must be {size}#{mode}")
	}
	size, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", err
	}
	return size, parts[1], nil
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

// resolveDailyPlayer maps a request to (playerID, isAnonymous, ok)
// per DP-08/DP-10/DP-14. A signed-in Clerk user (set by
// auth.RequireAuth) wins over the X-Device-Id header so users who
// happen to send both end up with their stable user ID. Returns
// ok=false when neither identifier is present — the caller emits 401.
func resolveDailyPlayer(r *http.Request) (string, bool, bool) {
	if u, present := auth.UserFromContextOK(r.Context()); present && u != nil && u.ID != "" {
		return u.ID, false, true
	}
	if device := r.Header.Get(dailyDeviceHeader); device != "" {
		return device, true, true
	}
	return "", false, false
}

// writeDailyError emits the canonical {"error":"<msg>"} body for
// non-200 responses on the daily endpoint. Kept private to this file
// because chunks 2-4 will share it.
func writeDailyError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		log.Printf("daily get: response write failed: %v", err)
	}
}
