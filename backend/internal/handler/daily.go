package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	dailysvc "github.com/eriksteenman/reign-game/backend/internal/service/daily"
)

// poolExhaustedMessage is the canonical 500 body phrase emitted when
// sync-fallback hits daily.ErrPoolExhausted. Stable phrase: the
// frontend keys off it for a "we're working on it" graceful UI.
const poolExhaustedMessage = "pool exhausted"

// dailyDeviceHeader is the header carrying the anonymous device
// identifier when no Clerk session cookie is present.
const dailyDeviceHeader = "X-Device-Id"

// playNotStartedMessage maps to HTTP 400 — the player must call GET
// (which materializes the PLAY row) before POSTing a result.
const playNotStartedMessage = "play not started"

// DailyService is the application surface the GET and POST daily
// handlers depend on. Defined as an interface so tests substitute a
// stub without spinning up a real DynamoDB-backed service.
type DailyService interface {
	GetDaily(ctx context.Context, in dailysvc.GetInput) (*dailysvc.DailyView, error)
	SubmitDaily(ctx context.Context, in dailysvc.SubmitInput) (*dailysvc.SubmitResult, error)
}

// dailyResponse is the GET 200 response shape. ServerElapsedMs and
// SubmittedAt are pointer-typed so `omitempty` drops them entirely
// when outcome != solved. IsRecycle is always present (no omitempty) so
// the contract is unambiguous: explicit false on non-recycle days.
type dailyResponse struct {
	PuzzleID        string  `json:"puzzleId"`
	Grid            int     `json:"grid"`
	Regions         [][]int `json:"regions"`
	AssignedAt      string  `json:"assignedAt"`
	Outcome         string  `json:"outcome"`
	ServerElapsedMs *int64  `json:"serverElapsedMs,omitempty"`
	SubmittedAt     *string `json:"submittedAt,omitempty"`
	IsRecycle       bool    `json:"isRecycle"`
}

// dailySubmitRequest is the POST /api/daily/{date}/result body shape.
// Decoded into pointers so the handler can distinguish missing fields
// from zero values.
type dailySubmitRequest struct {
	Outcome    *string `json:"outcome"`
	PlayTimeMs *int64  `json:"playTimeMs"`
	Solution   [][]int `json:"solution"`
}

// dailySubmitResponse is the POST 200 response shape. LeaderboardRank
// is pointer-typed so omitempty drops it for anonymous submits and
// rank-fetch failures.
type dailySubmitResponse struct {
	ServerElapsedMs int64 `json:"serverElapsedMs"`
	LeaderboardRank *int  `json:"leaderboardRank,omitempty"`
}

// DailyGetHandler creates an HTTP handler for GET /api/daily/{date}.
// It delegates all orchestration to svc.GetDaily and maps sentinel
// errors to the appropriate HTTP status codes.
func DailyGetHandler(svc DailyService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		playerID, isAnonymous, ok := resolveDailyPlayer(r)
		if !ok {
			writeDailyError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}

		date := chi.URLParam(r, "date")

		view, err := svc.GetDaily(r.Context(), dailysvc.GetInput{
			PlayerID:    playerID,
			IsAnonymous: isAnonymous,
			Date:        date,
		})
		if err != nil {
			writeDailyGetError(w, err)
			return
		}

		resp := dailyResponse{
			PuzzleID:        view.PuzzleID,
			Grid:            view.Grid,
			Regions:         view.Regions,
			AssignedAt:      view.AssignedAt,
			Outcome:         view.Outcome,
			ServerElapsedMs: view.ServerElapsedMs,
			SubmittedAt:     view.SubmittedAt,
			IsRecycle:       view.IsRecycle,
		}
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("daily get: response write failed: %v", err)
		}
	})
}

// writeDailyGetError translates service sentinel errors to HTTP status
// codes for the GET endpoint.
func writeDailyGetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dailysvc.ErrInvalidDate):
		writeDailyError(w, http.StatusBadRequest, "invalid date")
	case errors.Is(err, dailysvc.ErrOutOfWindow):
		writeDailyError(w, http.StatusNotFound, "out of window")
	case errors.Is(err, dailysvc.ErrScheduleNotFinalized):
		writeDailyError(w, http.StatusNotFound, "schedule not finalized")
	case errors.Is(err, dailysvc.ErrPoolExhausted):
		writeDailyError(w, http.StatusInternalServerError, poolExhaustedMessage)
	default:
		log.Printf("daily get: internal error: %v", err)
		writeDailyError(w, http.StatusInternalServerError, "internal error")
	}
}

// DailySubmitHandler creates the POST /api/daily/{date}/result handler.
// It decodes the body, enforces the outcome field check (HTTP-level
// concern), then delegates to svc.SubmitDaily.
func DailySubmitHandler(svc DailyService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		playerID, isAnonymous, ok := resolveDailyPlayer(r)
		if !ok {
			writeDailyError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}

		date := chi.URLParam(r, "date")

		var body dailySubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeDailyError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if body.Outcome == nil || *body.Outcome != "solved" {
			writeDailyError(w, http.StatusBadRequest, "invalid outcome")
			return
		}

		var playTimeMs int64
		if body.PlayTimeMs != nil {
			playTimeMs = *body.PlayTimeMs
		}

		result, err := svc.SubmitDaily(r.Context(), dailysvc.SubmitInput{
			PlayerID:    playerID,
			IsAnonymous: isAnonymous,
			Date:        date,
			PlayTimeMs:  playTimeMs,
			Solution:    body.Solution,
		})
		if err != nil {
			writeDailySubmitError(w, err)
			return
		}

		resp := dailySubmitResponse{
			ServerElapsedMs: result.ServerElapsedMs,
			LeaderboardRank: result.LeaderboardRank,
		}
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("daily submit: response write failed: %v", err)
		}
	})
}

// writeDailySubmitError translates service sentinel errors to HTTP
// status codes for the POST endpoint.
func writeDailySubmitError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dailysvc.ErrInvalidDate):
		writeDailyError(w, http.StatusBadRequest, "invalid date")
	case errors.Is(err, dailysvc.ErrOutOfWindow):
		writeDailyError(w, http.StatusNotFound, "out of window")
	case errors.Is(err, dailysvc.ErrScheduleNotFinalized):
		writeDailyError(w, http.StatusNotFound, "schedule not finalized")
	case errors.Is(err, dailysvc.ErrPoolExhausted):
		writeDailyError(w, http.StatusInternalServerError, poolExhaustedMessage)
	case errors.Is(err, dailysvc.ErrPlayNotStarted):
		writeDailyError(w, http.StatusBadRequest, playNotStartedMessage)
	case errors.Is(err, dailysvc.ErrAlreadySolved):
		writeDailyError(w, http.StatusConflict, "already solved")
	case errors.Is(err, dailysvc.ErrInvalidSolution):
		writeDailyError(w, http.StatusBadRequest, "invalid solution")
	case errors.Is(err, dailysvc.ErrInvalidPlayTime):
		writeDailyError(w, http.StatusBadRequest, "invalid playTimeMs")
	case errors.Is(err, dailysvc.ErrNegativeClockSkew):
		writeDailyError(w, http.StatusInternalServerError, "internal error")
	default:
		log.Printf("daily submit: internal error: %v", err)
		writeDailyError(w, http.StatusInternalServerError, "internal error")
	}
}

// resolveDailyPlayer maps a request to (playerID, isAnonymous, ok).
// A signed-in Clerk user wins over X-Device-Id so users who happen to
// send both end up with their stable user ID. Returns ok=false when
// neither identifier is present (caller emits 401). A present-but-
// malformed X-Device-Id is treated as absent — the caller sees the
// same 401 as a missing header. This is the "generic error messages
// in auth" rule (backend/CLAUDE.md): no client-observable difference
// between "you didn't authenticate" and "your header failed
// validation", so attackers can't probe the validator's bounds.
func resolveDailyPlayer(r *http.Request) (playerID string, isAnonymous, ok bool) {
	if u, present := auth.UserFromContextOK(r.Context()); present && u != nil && u.ID != "" {
		return u.ID, false, true
	}
	device := r.Header.Get(dailyDeviceHeader)
	if device == "" {
		return "", false, false
	}
	if err := validateDeviceID(device); err != nil {
		return "", false, false
	}
	return device, true, true
}

// deviceIDPattern accepts URL-safe ASCII identifiers between 16 and
// 64 characters. The shape is what `crypto.randomUUID()` and similar
// browser APIs produce (with or without dashes) plus opaque tokens
// the client might generate via base64url. Length cap shields the
// header path from megabyte-scale payloads; the charset rule blocks
// control characters and anything that needs escaping downstream.
var deviceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

const (
	deviceIDMinLength = 16
	deviceIDMaxLength = 64
)

// errInvalidDeviceID is the sentinel for X-Device-Id values that fail
// shape validation. Returned by validateDeviceID and surfaced as HTTP
// 400 by the daily handlers.
var errInvalidDeviceID = errors.New("invalid device id")

// validateDeviceID enforces the X-Device-Id shape contract:
// length 16-64 chars + URL-safe ASCII only. Returns errInvalidDeviceID
// on violation. An empty string is invalid; callers should branch on
// non-empty before calling.
func validateDeviceID(id string) error {
	if len(id) < deviceIDMinLength || len(id) > deviceIDMaxLength {
		return errInvalidDeviceID
	}
	if !deviceIDPattern.MatchString(id) {
		return errInvalidDeviceID
	}
	return nil
}

// writeDailyError emits the canonical {"error":"<msg>"} body for
// non-200 responses on the daily endpoint.
func writeDailyError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		log.Printf("daily: response write failed: %v", err)
	}
}
