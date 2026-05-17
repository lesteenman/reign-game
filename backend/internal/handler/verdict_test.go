package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	verdictsvc "github.com/eriksteenman/reign-game/backend/internal/service/verdict"
)

// stubVerdictService is a hand-rolled handler.VerdictService used to keep
// the handler tests focused on HTTP-level concerns (param validation, body
// validation, auth-context plumbing, error-code translation, response
// envelope). Stateful semantics like last-write-wins idempotency and
// multi-rater aggregation are exercised at the repository layer
// (puzzle_test.go) and the orchestration layer (verdict_test.go in
// internal/service/verdict).
type stubVerdictService struct {
	// summary is the success result returned by Submit when submitErr is nil.
	summary verdictsvc.Summary
	// submitErr is the error returned by Submit (nil on the happy path).
	submitErr error

	// Observation.
	submitCalled bool
	lastInput    verdictsvc.SubmitInput
}

func (s *stubVerdictService) Submit(_ context.Context, in *verdictsvc.SubmitInput) (verdictsvc.Summary, error) {
	s.submitCalled = true
	s.lastInput = *in
	return s.summary, s.submitErr
}

// verdictResponseJSON mirrors the handler's response envelope for
// decoding in tests. Kept in this file (not the production code)
// because tests are the only consumer of the JSON shape.
type verdictResponseJSON struct {
	Summary struct {
		Up            int    `json:"up"`
		Down          int    `json:"down"`
		LastUpdatedAt string `json:"lastUpdatedAt"`
	} `json:"summary"`
}

// validVerdictBody returns a body that passes all body-decode checks.
func validVerdictBody() *bytes.Buffer {
	return bytes.NewBufferString(`{"value":"up","playTimeMs":42000,"outcome":"solved","clientVersion":"0.1.0"}`)
}

// TestVerdictHandler_AuthMatrix exercises the auth matrix that every
// /api/admin/* route inherits from RequireAuth + RequireAdmin.
// Anonymous → 401, signed-in non-admin → 403, admin → 200.
func TestVerdictHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			svc := &stubVerdictService{
				summary: verdictsvc.Summary{Up: 1, Down: 0, LastUpdatedAt: "2026-05-16T10:00:00Z"},
			}
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(svc))
			}, roleForState(tc.state))

			req := newAdminRequest(tc.state, http.MethodPut, "/api/admin/puzzles/abc/verdict?size=7&mode=standard", validVerdictBody())
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tc.wantCode, rec.Body.String())
			}
		})
	}
}

func TestVerdictHandler_Validation(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		want int
	}{
		{"missing size query param", "/api/admin/puzzles/abc/verdict?mode=standard", `{"value":"up","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"missing mode query param", "/api/admin/puzzles/abc/verdict?size=7", `{"value":"up","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"non-numeric size", "/api/admin/puzzles/abc/verdict?size=foo&mode=standard", `{"value":"up","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"out-of-range size", "/api/admin/puzzles/abc/verdict?size=99&mode=standard", `{"value":"up","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"unknown mode", "/api/admin/puzzles/abc/verdict?size=7&mode=triple", `{"value":"up","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"value uppercase rejected", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"UP","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"value=upvote rejected", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"upvote","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"skip rejected from verdict surface", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"skip","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"negative playTimeMs", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"up","playTimeMs":-1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"playTimeMs as string", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"up","playTimeMs":"50","outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"outcome=completed rejected", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"up","playTimeMs":1,"outcome":"completed","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"missing value field", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"malformed JSON body", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{not-json`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubVerdictService{}
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(svc))
			}, "admin")

			req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.want, rec.Body.String())
			}
			if svc.submitCalled {
				t.Error("Submit must NOT be called when the request is rejected during validation")
			}
		})
	}
}

// ErrPuzzleNotFound from the service translates to 404.
func TestVerdictHandler_PuzzleNotFound(t *testing.T) {
	// Arrange
	svc := &stubVerdictService{submitErr: verdictsvc.ErrPuzzleNotFound}
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(svc))
	}, "admin")

	req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, "/api/admin/puzzles/missing/verdict?size=7&mode=standard", validVerdictBody())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

// A malicious caller adding a `raterId` field to the JSON body must NOT
// influence the RaterID that the handler passes to the service. The handler
// derives RaterID exclusively from the Clerk session.
func TestVerdictHandler_RaterIDFromSessionNotBody(t *testing.T) {
	// Arrange
	svc := &stubVerdictService{
		summary: verdictsvc.Summary{Up: 1, Down: 0, LastUpdatedAt: "2026-05-16T10:00:00Z"},
	}
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(svc))
	}, "admin")

	body := `{"value":"up","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0","raterId":"imposter"}`
	req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, "/api/admin/puzzles/abc/verdict?size=7&mode=standard", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if !svc.submitCalled {
		t.Fatal("Submit was not invoked")
	}
	if svc.lastInput.RaterID == "imposter" {
		t.Error("RaterID was sourced from the request body, want Clerk session ID")
	}
	if svc.lastInput.RaterID != testAdminSub {
		t.Errorf("RaterID = %q, want %q (from Clerk session)", svc.lastInput.RaterID, testAdminSub)
	}
	if svc.lastInput.RaterRole != "admin" {
		t.Errorf("RaterRole = %q, want %q", svc.lastInput.RaterRole, "admin")
	}
}

// When the service returns success with an empty summary (best-effort
// recompute swallowed a transient failure or benign race), the handler
// still returns 200. The user-visible action succeeded.
func TestVerdictHandler_EmptySummaryStill200(t *testing.T) {
	// Arrange
	svc := &stubVerdictService{summary: verdictsvc.Summary{}}
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(svc))
	}, "admin")

	req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, "/api/admin/puzzles/abc/verdict?size=7&mode=standard", validVerdictBody())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (empty summary must not fail the request); body = %s", rec.Code, rec.Body.String())
	}
}

// A generic Submit error (e.g. PutVerdict failure wrapped by the service)
// translates to 500.
func TestVerdictHandler_ServiceErrorReturns500(t *testing.T) {
	// Arrange
	svc := &stubVerdictService{submitErr: errors.New("dynamodb write failed")}
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(svc))
	}, "admin")

	req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, "/api/admin/puzzles/abc/verdict?size=7&mode=standard", validVerdictBody())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
}

// The response envelope carries the recomputed summary as plain fields
// {up, down, lastUpdatedAt} so the UI can render new totals without a
// follow-up GET.
func TestVerdictHandler_HappyPathResponseShape(t *testing.T) {
	// Arrange
	svc := &stubVerdictService{
		summary: verdictsvc.Summary{Up: 3, Down: 1, LastUpdatedAt: "2026-05-16T10:00:00Z"},
	}
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(svc))
	}, "admin")

	req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, "/api/admin/puzzles/abc/verdict?size=7&mode=standard", validVerdictBody())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var resp verdictResponseJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Summary.Up != 3 {
		t.Errorf("response Summary.up = %d, want 3", resp.Summary.Up)
	}
	if resp.Summary.Down != 1 {
		t.Errorf("response Summary.down = %d, want 1", resp.Summary.Down)
	}
	if resp.Summary.LastUpdatedAt != "2026-05-16T10:00:00Z" {
		t.Errorf("response Summary.lastUpdatedAt = %q, want %q", resp.Summary.LastUpdatedAt, "2026-05-16T10:00:00Z")
	}

	// And the handler passed the URL/body fields through to the service.
	if !svc.submitCalled {
		t.Fatal("Submit was not invoked")
	}
	if svc.lastInput.GridSize != 7 || svc.lastInput.Mode != "standard" || svc.lastInput.PuzzleID != "abc" {
		t.Errorf("SubmitInput key = (%d,%s,%s), want (7,standard,abc)", svc.lastInput.GridSize, svc.lastInput.Mode, svc.lastInput.PuzzleID)
	}
	if svc.lastInput.Value != "up" || svc.lastInput.Outcome != "solved" || svc.lastInput.PlayTimeMs != 42000 || svc.lastInput.ClientVersion != "0.1.0" {
		t.Errorf("SubmitInput body fields = (%s,%s,%d,%s), want (up,solved,42000,0.1.0)",
			svc.lastInput.Value, svc.lastInput.Outcome, svc.lastInput.PlayTimeMs, svc.lastInput.ClientVersion)
	}
}
