package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

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
// when outcome != solved.
type dailyResponse struct {
	PuzzleID        string  `json:"puzzleId"`
	Grid            int     `json:"grid"`
	Regions         [][]int `json:"regions"`
	AssignedAt      string  `json:"assignedAt"`
	Outcome         string  `json:"outcome"`
	ServerElapsedMs *int64  `json:"serverElapsedMs,omitempty"`
	SubmittedAt     *string `json:"submittedAt,omitempty"`
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
// neither identifier is present — caller emits 401.
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
// non-200 responses on the daily endpoint.
func writeDailyError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		log.Printf("daily: response write failed: %v", err)
	}
}
