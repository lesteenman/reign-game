package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// dailyDateLayout is the canonical YYYY-MM-DD format expected by
// GET /api/daily/{date}. Dates outside this layout are rejected with
// 400 per DP-07.
const dailyDateLayout = "2006-01-02"

// dailyDeviceHeader is the header carrying the anonymous device
// identifier when no Clerk session cookie is present (DP-08).
const dailyDeviceHeader = "X-Device-Id"

// DailyGetHandler creates an HTTP handler for GET /api/daily/{date}.
//
// Chunk 1 implements only the gate: date validation (DP-07),
// auth-context resolution (DP-08, DP-10, DP-14), and a 501
// not-implemented stub for requests that pass both. Schedule and
// PLAY-row reads land in chunk 2; sync fallback in chunk 3.
//
// The clock argument lets tests pin "today" for the
// [yesterdayUTC, todayUTC] window check; production callers pass
// time.Now.
func DailyGetHandler(_ *repository.PuzzleRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

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

		requested := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		if requested.Before(yesterdayUTC) || requested.After(todayUTC) {
			writeDailyError(w, http.StatusNotFound, "out of window")
			log.Printf("daily get: 404 out_of_window date=%s player=%s anon=%t", date, playerID, isAnonymous)
			return
		}

		writeDailyError(w, http.StatusNotImplemented, "not implemented")
		log.Printf("daily get: 501 stub date=%s player=%s anon=%t", date, playerID, isAnonymous)
	})
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
