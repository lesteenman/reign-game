package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

// DailyRepo is the narrow surface the GET and POST handlers need from
// the repository layer. Decoupling on a small interface lets tests use
// a hand-rolled fake without LocalStack and keeps the production
// repository (*repository.PuzzleRepository) trivially compliant.
type DailyRepo interface {
	GetSchedule(ctx context.Context, date string) (*repository.ScheduleRecord, error)
	GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error)
	GetPlay(ctx context.Context, playerID, date string) (*repository.PlayRecord, error)
	PutPlayStartedIfAbsent(ctx context.Context, playerID, date, puzzleID string, assignedAt time.Time) error

	// GetCandidate + FinalizeDailyTransaction + ListApprovedPool +
	// PutCandidateIfAbsent are required by the sync-fallback path
	// (DP-05). DailyRepo is a structural superset of daily.Repo so a
	// DailyRepo value can be passed straight into
	// daily.SyncFinalizeForToday without an adapter.
	GetCandidate(ctx context.Context) (*repository.CandidateRecord, error)
	FinalizeDailyTransaction(ctx context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error
	ListApprovedPool(ctx context.Context, size int, mode string, excludeRecentlyDailied bool, now time.Time) ([]repository.PuzzleRecord, error)
	PutCandidateIfAbsent(ctx context.Context, puzzleID, sourcePartition string) error

	// SubmitPlayTransactionally + LeaderboardRank power POST
	// /api/daily/{date}/result (DP-12). The submit method commits the
	// PLAY-row update, the schedule counter ADD, and (signed-in only) the
	// leaderboard row in one TransactWriteItems; LeaderboardRank is
	// best-effort post-commit and never fails the response.
	SubmitPlayTransactionally(ctx context.Context, playerID, date string, submission *repository.SubmitInput) error
	LeaderboardRank(ctx context.Context, date string, elapsedMs int64, userID string) (int, error)
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

		readStart := time.Now()
		schedule, existingPlay, sErr, pErr := fetchScheduleAndPlay(ctx, repo, date, playerID)
		readMs := time.Since(readStart).Milliseconds()
		if sErr != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily get: 500 schedule_read_failed date=%s err=%v", date, sErr)
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

		if pErr != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily get: 500 play_read_failed date=%s player=%s err=%v", date, truncatePlayer(playerID), pErr)
			return
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

		play, playMs, err := materializePlayRow(ctx, repo, existingPlay, playerID, date, schedule.PuzzleID, clock)
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
		log.Printf("daily get: total_ms=%d read_ms=%d sync_ms=%d puzzle_ms=%d play_ms=%d path=%s player=%s",
			totalMs, readMs, syncMs, puzzleMs, playMs, r.URL.Path, truncatePlayer(playerID))
	})
}

// fetchScheduleAndPlay runs GetSchedule and GetPlay concurrently. They
// have no dependency — both are keyed off (date, playerID) alone — so
// fanning them out saves one DDB RTT per request on the daily hot path
// (R2). Returns (schedule, play, scheduleErr, playErr) so callers can
// surface different status codes for each path. Both errors may be set
// if both calls failed; callers are expected to handle scheduleErr
// first because the schedule drives sync-fallback and 404 routing,
// which the play read can't supersede.
func fetchScheduleAndPlay(
	ctx context.Context,
	repo DailyRepo,
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
		schedule, sErr = repo.GetSchedule(ctx, date)
	}()
	go func() {
		defer wg.Done()
		play, pErr = repo.GetPlay(ctx, playerID, date)
	}()
	wg.Wait()
	return schedule, play, sErr, pErr
}

// materializePlayRow returns the PLAY row for (playerID, date),
// creating it on first GET. On the race-loser branch it re-reads the
// row so the caller surfaces the winner's assignedAt — DP-19's
// "assignedAt is set once, never overwritten" invariant lives here.
// existingPlay is the result of an upstream GetPlay (typically from
// fetchScheduleAndPlay's parallel fan-out); when non-nil the function
// short-circuits and returns it directly, avoiding a second DDB read.
// When nil, the started-row creation + race-loser re-read path runs as
// before. playMs is the wall-clock cost of any PLAY-related DDB calls
// this function issues itself (zero on the cache-hit path).
func materializePlayRow(
	ctx context.Context,
	repo DailyRepo,
	existingPlay *repository.PlayRecord,
	playerID, date, puzzleID string,
	clock func() time.Time,
) (*repository.PlayRecord, int64, error) {
	if existingPlay != nil {
		return existingPlay, 0, nil
	}

	playStart := time.Now()
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
func resolveDailyPlayer(r *http.Request) (playerID string, isAnonymous, ok bool) {
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

// dailySubmitRequest is the POST /api/daily/{date}/result body shape
// per DP-11. Decoded into pointers so the handler can distinguish
// missing fields from zero values for outcome and playTimeMs.
type dailySubmitRequest struct {
	Outcome    *string `json:"outcome"`
	PlayTimeMs *int64  `json:"playTimeMs"`
	Solution   [][]int `json:"solution"`
}

// dailySubmitResponse is the POST 200 response shape. JSON keys match
// the GET handler's serverElapsedMs casing (DP-09) so the SPA can
// share field readers across endpoints. LeaderboardRank is pointer-typed
// so omitempty drops it for anonymous submits and rank-fetch failures.
type dailySubmitResponse struct {
	ServerElapsedMs int64 `json:"serverElapsedMs"`
	LeaderboardRank *int  `json:"leaderboardRank,omitempty"`
}

// playNotStartedMessage maps to HTTP 400 — the player must call GET
// (which materializes the PLAY row) before POSTing a result. DP-21
// doesn't pin a status code for this path explicitly, but 400 is the
// right family: the request is structurally valid yet semantically
// requires server state the caller hasn't established. Keeping it 400
// rather than 404 avoids confusing the "no daily today" case (which
// already 404s on GET) with the "submit before start" case.
const playNotStartedMessage = "play not started"

// DailySubmitHandler creates the POST /api/daily/{date}/result handler
// per DP-11..DP-14. Flow:
//
//  1. auth + date window (chunks 1+2 helpers, unchanged)
//  2. parse + validate body (outcome, playTimeMs, solution shape)
//  3. GetPlay — 400 when missing, 409 when already solved
//  4. GetSchedule + GetPuzzle to validate the submitted solution
//     against the canonical puzzle (DP-11)
//  5. SubmitPlayTransactionally — 409 on race-loser, 500 on other
//     errors (DP-22 structured log)
//  6. LeaderboardRank for signed-in players; on error log a WARN and
//     omit the field — the row is already written
//
// The clock argument lets tests pin "now" so serverElapsedMs is
// deterministic; production callers pass time.Now.
func DailySubmitHandler(repo DailyRepo, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		start := time.Now()

		playerID, isAnonymous, ok := resolveDailyPlayer(r)
		if !ok {
			writeDailyError(w, http.StatusUnauthorized, "unauthenticated")
			log.Printf("daily submit: 401 unauthenticated path=%s", r.URL.Path)
			return
		}

		date := chi.URLParam(r, "date")
		parsed, err := time.ParseInLocation(dailyDateLayout, date, time.UTC)
		if err != nil {
			writeDailyError(w, http.StatusBadRequest, "invalid date")
			log.Printf("daily submit: 400 invalid_date date=%q player=%s", date, truncatePlayer(playerID))
			return
		}
		now := clock().UTC()
		todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		yesterdayUTC := todayUTC.AddDate(0, 0, -1)
		requested := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		if requested.Before(yesterdayUTC) || requested.After(todayUTC) {
			writeDailyError(w, http.StatusNotFound, "out of window")
			log.Printf("daily submit: 404 out_of_window date=%s player=%s", date, truncatePlayer(playerID))
			return
		}

		body, validationErr := parseAndValidateSubmitBody(r)
		if validationErr != nil {
			writeDailyError(w, http.StatusBadRequest, validationErr.Error())
			log.Printf("daily submit: 400 %s date=%s player=%s", validationErr.Error(), date, truncatePlayer(playerID))
			return
		}

		ctx := r.Context()

		readStart := time.Now()
		schedule, play, sErr, pErr := fetchScheduleAndPlay(ctx, repo, date, playerID)
		readMs := time.Since(readStart).Milliseconds()
		if pErr != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily submit: 500 play_read_failed date=%s player=%s err=%v", date, truncatePlayer(playerID), pErr)
			return
		}
		if play == nil {
			writeDailyError(w, http.StatusBadRequest, playNotStartedMessage)
			log.Printf("daily submit: 400 play_not_started date=%s player=%s", date, truncatePlayer(playerID))
			return
		}
		if play.Outcome == repository.PlayOutcomeSolved {
			writeDailyError(w, http.StatusConflict, "already solved")
			log.Printf("daily submit: 409 already_solved date=%s player=%s", date, truncatePlayer(playerID))
			return
		}
		if sErr != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily submit: 500 schedule_read_failed date=%s err=%v", date, sErr)
			return
		}
		if schedule == nil {
			// PLAY row exists but no schedule row — system invariant
			// violation; we never finalize a PLAY for a date whose
			// schedule has already aged out. 500 with a loud log so ops
			// notices.
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily submit: 500 schedule_missing_for_play date=%s player=%s", date, truncatePlayer(playerID))
			return
		}

		size, mode, err := parseSourcePartition(schedule.SourcePartition)
		if err != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily submit: 500 source_partition_malformed date=%s sourcePartition=%q err=%v", date, schedule.SourcePartition, err)
			return
		}

		puzzleStart := time.Now()
		puzzle, err := repo.GetPuzzle(ctx, size, mode, schedule.PuzzleID)
		puzzleMs := time.Since(puzzleStart).Milliseconds()
		if err != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily submit: 500 puzzle_read_failed date=%s puzzleId=%s err=%v", date, schedule.PuzzleID, err)
			return
		}
		if puzzle == nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily submit: 500 puzzle_missing date=%s puzzleId=%s", date, schedule.PuzzleID)
			return
		}

		if !solutionMatches(body.Solution, puzzle.Solution) {
			writeDailyError(w, http.StatusBadRequest, "invalid solution")
			log.Printf("daily submit: 400 invalid_solution date=%s player=%s puzzleId=%s", date, truncatePlayer(playerID), schedule.PuzzleID)
			return
		}

		assignedAt, err := time.Parse(time.RFC3339, play.AssignedAt)
		if err != nil {
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily submit: 500 assignedAt_parse_failed date=%s assignedAt=%q err=%v", date, play.AssignedAt, err)
			return
		}
		serverElapsedMs := now.Sub(assignedAt).Milliseconds()
		if serverElapsedMs < 0 {
			// Defense: assignedAt > now means clock skew between the PLAY-row
			// writer and submit handler. Refuse the submission rather than
			// clamp — clamping to 0 would silently push a clock-skewed solve
			// to the top of the leaderboard SK, corrupting ranking. The repo
			// also defends against this (see repository/daily.go), but
			// catching here returns a clearer error code than letting the
			// transaction fail.
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			log.Printf("daily submit: 500 negative_clock_skew date=%s assignedAt=%s now=%s delta_ms=%d player=%s",
				date, assignedAt.Format(time.RFC3339), now.Format(time.RFC3339), serverElapsedMs, truncatePlayer(playerID))
			return
		}

		submitInput := repository.SubmitInput{
			PuzzleID:    schedule.PuzzleID,
			AssignedAt:  assignedAt,
			SubmittedAt: now,
			ClientMs:    *body.PlayTimeMs,
			IsAnonymous: isAnonymous,
		}
		if !isAnonymous {
			submitInput.UserID = playerID
		}

		submitStart := time.Now()
		submitErr := repo.SubmitPlayTransactionally(ctx, playerID, date, &submitInput)
		submitMs := time.Since(submitStart).Milliseconds()
		if submitErr != nil {
			if errors.Is(submitErr, repository.ErrPlayNotInStartedState) {
				writeDailyError(w, http.StatusConflict, "already solved")
				log.Printf("daily submit: 409 race_loser date=%s player=%s", date, truncatePlayer(playerID))
				return
			}
			writeDailyError(w, http.StatusInternalServerError, "internal error")
			// DP-22: structured failure log. Raw err is logged but never
			// echoed in the response body — a DDB error message could
			// leak internal table names / capacity hints.
			log.Printf("daily submit: 500 transaction_failed date=%s player=%s err=%v", date, truncatePlayer(playerID), submitErr)
			return
		}

		var rankMs int64
		resp := dailySubmitResponse{ServerElapsedMs: serverElapsedMs}
		if !isAnonymous {
			rankStart := time.Now()
			rank, rankErr := repo.LeaderboardRank(ctx, date, serverElapsedMs, playerID)
			rankMs = time.Since(rankStart).Milliseconds()
			if rankErr != nil {
				log.Printf("WARN: daily submit: rank_fetch_failed date=%s player=%s err=%v", date, truncatePlayer(playerID), rankErr)
			} else {
				resp.LeaderboardRank = &rank
			}
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("daily submit: response write failed: %v", err)
			return
		}

		totalMs := time.Since(start).Milliseconds()
		log.Printf("daily submit: total_ms=%d read_ms=%d puzzle_ms=%d submit_ms=%d rank_ms=%d path=%s player=%s",
			totalMs, readMs, puzzleMs, submitMs, rankMs, r.URL.Path, truncatePlayer(playerID))
	})
}

// parseAndValidateSubmitBody decodes the POST body and enforces the
// DP-11 contract. Returns a typed *dailySubmitRequest with non-nil
// pointer fields on success, or a sentinel-shaped error whose message
// is safe to surface in the response body (no DDB internals, no
// stacktrace leakage).
func parseAndValidateSubmitBody(r *http.Request) (*dailySubmitRequest, error) {
	var body dailySubmitRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil {
		return nil, errors.New("invalid body")
	}

	if body.Outcome == nil || *body.Outcome != "solved" {
		return nil, errors.New("invalid outcome")
	}
	if body.PlayTimeMs == nil || *body.PlayTimeMs < 0 {
		return nil, errors.New("invalid playTimeMs")
	}
	if !solutionShapeValid(body.Solution) {
		return nil, errors.New("invalid solution")
	}
	return &body, nil
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
