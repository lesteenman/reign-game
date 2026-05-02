package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	"github.com/eriksteenman/reign-game/backend/internal/handler"
)

// fixedClock returns a clock function pinned to 2026-05-02 12:00:00 UTC.
// All date-window assertions in this file are anchored to that instant:
// today=2026-05-02, yesterday=2026-05-01.
func fixedClock() func() time.Time {
	t := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

// mountDaily mounts the daily GET handler at /api/daily/{date} and
// returns the chi router so tests can issue requests through it.
// The handler is constructed with a nil repository; chunk 1 doesn't
// touch the repo (501 stub on success), so a nil repo is sufficient
// to exercise the auth + date validation paths.
func mountDaily() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/daily/{date}", handler.DailyGetHandler(nil, fixedClock()).ServeHTTP)
	return r
}

// withUserID returns a request whose context carries a Clerk user
// with the given ID, simulating what auth.RequireAuth puts in place
// after verifying a session cookie.
func withUserID(req *http.Request, userID string) *http.Request {
	ctx := auth.WithUserForTest(req.Context(), &clerk.User{ID: userID})
	return req.WithContext(ctx)
}

// readErrorBody decodes a non-200 response body and returns the error
// field. Fails the test if the body isn't a JSON object with an
// "error" key.
func readErrorBody(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	msg, ok := body["error"]
	if !ok {
		t.Fatalf("response body missing error field: %v", body)
	}
	return msg
}

func TestDailyGetHandler_AuthMatrix(t *testing.T) {
	cases := []struct {
		name       string
		userID     string // empty => no userID in context
		deviceID   string // empty => no X-Device-Id header
		wantStatus int
		wantError  string // empty => not asserted
	}{
		{
			name:       "no userID and no device header returns 401",
			userID:     "",
			deviceID:   "",
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthenticated",
		},
		{
			name:       "device header alone passes auth (501 stub)",
			userID:     "",
			deviceID:   "dev_abc",
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:       "userID alone passes auth (501 stub)",
			userID:     "user_xyz",
			deviceID:   "",
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:       "userID wins when both present (501 stub)",
			userID:     "user_xyz",
			deviceID:   "dev_abc",
			wantStatus: http.StatusNotImplemented,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			router := mountDaily()
			req := httptest.NewRequest(http.MethodGet, "/api/daily/2026-05-02", nil)
			if tc.userID != "" {
				req = withUserID(req, tc.userID)
			}
			if tc.deviceID != "" {
				req.Header.Set("X-Device-Id", tc.deviceID)
			}
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantStatus {
				t.Fatalf("status: got %d want %d (body=%q)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantError != "" {
				if got := readErrorBody(t, rec); got != tc.wantError {
					t.Fatalf("error body: got %q want %q", got, tc.wantError)
				}
			}
		})
	}
}

func TestDailyGetHandler_DateMatrix(t *testing.T) {
	// Auth is satisfied via X-Device-Id for every row so we exercise
	// the date branch alone. Today (per fixedClock) is 2026-05-02.
	cases := []struct {
		name       string
		date       string
		wantStatus int
		wantError  string
	}{
		{name: "today returns 501 stub", date: "2026-05-02", wantStatus: http.StatusNotImplemented},
		{name: "yesterday returns 501 stub", date: "2026-05-01", wantStatus: http.StatusNotImplemented},
		{name: "tomorrow returns 404 out-of-window", date: "2026-05-03", wantStatus: http.StatusNotFound, wantError: "out of window"},
		{name: "two days ago returns 404 out-of-window", date: "2026-04-30", wantStatus: http.StatusNotFound, wantError: "out of window"},
		{name: "calendar-impossible date returns 400", date: "2026-13-99", wantStatus: http.StatusBadRequest, wantError: "invalid date"},
		{name: "non-date string returns 400", date: "not-a-date", wantStatus: http.StatusBadRequest, wantError: "invalid date"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			router := mountDaily()
			req := httptest.NewRequest(http.MethodGet, "/api/daily/"+tc.date, nil)
			req.Header.Set("X-Device-Id", "dev_abc")
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantStatus {
				t.Fatalf("status for %q: got %d want %d (body=%q)", tc.date, rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantError != "" {
				if got := readErrorBody(t, rec); got != tc.wantError {
					t.Fatalf("error body for %q: got %q want %q", tc.date, got, tc.wantError)
				}
			}
		})
	}
}
