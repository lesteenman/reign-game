package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	"github.com/eriksteenman/reign-game/backend/internal/handler"
	dailysvc "github.com/eriksteenman/reign-game/backend/internal/service/daily"
)

// stubDailyService is a hand-rolled handler.DailyService used to keep
// the handler tests focused on HTTP-level concerns: param parsing,
// body validation, auth plumbing, error-code translation, and response
// shape. Service-layer semantics (schedule reads, sync-fallback, play
// materialization, solution validation) are exercised in the service
// tests.
type stubDailyService struct {
	getDailyView      *dailysvc.DailyView
	getDailyErr       error
	submitDailyResult *dailysvc.SubmitResult
	submitDailyErr    error

	// Observation.
	getDailyCalled    bool
	submitDailyCalled bool
	lastGetInput      dailysvc.GetInput
	lastSubmitInput   dailysvc.SubmitInput
}

func (s *stubDailyService) GetDaily(_ context.Context, in dailysvc.GetInput) (*dailysvc.DailyView, error) {
	s.getDailyCalled = true
	s.lastGetInput = in
	return s.getDailyView, s.getDailyErr
}

func (s *stubDailyService) SubmitDaily(_ context.Context, in dailysvc.SubmitInput) (*dailysvc.SubmitResult, error) {
	s.submitDailyCalled = true
	s.lastSubmitInput = in
	return s.submitDailyResult, s.submitDailyErr
}

// mountDailyGet mounts DailyGetHandler at GET /api/daily/{date} with
// OptionalAuth so the auth-matrix tests can inject user context.
func mountDailyGet(svc handler.DailyService) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/daily/{date}", handler.DailyGetHandler(svc).ServeHTTP)
	return r
}

// mountDailySubmit mounts DailySubmitHandler at POST
// /api/daily/{date}/result.
func mountDailySubmit(svc handler.DailyService) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/api/daily/{date}/result", handler.DailySubmitHandler(svc).ServeHTTP)
	return r
}

// withDailyUserID returns a request whose context carries a Clerk user.
func withDailyUserID(req *http.Request, userID string) *http.Request {
	ctx := auth.WithUserForTest(req.Context(), &clerk.User{ID: userID})
	return req.WithContext(ctx)
}

// dailyErrorBody decodes a non-200 response and returns the "error" field.
func dailyErrorBody(t *testing.T, rec *httptest.ResponseRecorder) string {
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

// happyGetView is a DailyView used for GET happy-path assertions.
func happyGetView() *dailysvc.DailyView {
	regions := make([][]int, 9)
	for i := range regions {
		regions[i] = make([]int, 9)
	}
	elapsed := int64(42000)
	submittedAt := "2026-05-02T12:00:00Z"
	return &dailysvc.DailyView{
		PuzzleID:        "puzzle-abc",
		Grid:            9,
		Regions:         regions,
		AssignedAt:      "2026-05-02T10:00:00Z",
		Outcome:         "solved",
		ServerElapsedMs: &elapsed,
		SubmittedAt:     &submittedAt,
	}
}

// validSubmitBody returns a POST body that passes all HTTP-level checks.
func validSubmitBody() *bytes.Buffer {
	return bytes.NewBufferString(`{"outcome":"solved","playTimeMs":42000,"solution":[[1,0],[0,1]]}`)
}

// ---------------------------------------------------------------------------
// GET auth matrix
// ---------------------------------------------------------------------------

func TestDailyGetHandler_AuthMatrix(t *testing.T) {
	cases := []struct {
		name       string
		userID     string
		deviceID   string
		wantStatus int
	}{
		{"no identity returns 401", "", "", http.StatusUnauthorized},
		{"device header alone passes (200)", "", "dev_abc", http.StatusOK},
		{"userID alone passes (200)", "user_xyz", "", http.StatusOK},
		{"userID wins over device (200)", "user_xyz", "dev_abc", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			svc := &stubDailyService{getDailyView: happyGetView()}
			router := mountDailyGet(svc)
			req := httptest.NewRequest(http.MethodGet, "/api/daily/2026-05-02", http.NoBody)
			if tc.userID != "" {
				req = withDailyUserID(req, tc.userID)
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
		})
	}
}

// ---------------------------------------------------------------------------
// GET happy path
// ---------------------------------------------------------------------------

func TestDailyGetHandler_HappyPath(t *testing.T) {
	// Arrange
	view := happyGetView()
	svc := &stubDailyService{getDailyView: view}
	router := mountDailyGet(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/2026-05-02", http.NoBody)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	var body struct {
		PuzzleID        string  `json:"puzzleId"`
		Grid            int     `json:"grid"`
		AssignedAt      string  `json:"assignedAt"`
		Outcome         string  `json:"outcome"`
		ServerElapsedMs *int64  `json:"serverElapsedMs"`
		SubmittedAt     *string `json:"submittedAt"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.PuzzleID != view.PuzzleID {
		t.Errorf("puzzleId: got %q want %q", body.PuzzleID, view.PuzzleID)
	}
	if body.Grid != view.Grid {
		t.Errorf("grid: got %d want %d", body.Grid, view.Grid)
	}
	if body.Outcome != view.Outcome {
		t.Errorf("outcome: got %q want %q", body.Outcome, view.Outcome)
	}
	if body.ServerElapsedMs == nil || *body.ServerElapsedMs != *view.ServerElapsedMs {
		t.Errorf("serverElapsedMs: got %v want %v", body.ServerElapsedMs, view.ServerElapsedMs)
	}
	if body.SubmittedAt == nil || *body.SubmittedAt != *view.SubmittedAt {
		t.Errorf("submittedAt: got %v want %v", body.SubmittedAt, view.SubmittedAt)
	}
	if !svc.getDailyCalled {
		t.Error("GetDaily was not called")
	}
	if svc.lastGetInput.PlayerID != "dev_abc" || !svc.lastGetInput.IsAnonymous {
		t.Errorf("GetInput: got (%q, anon=%t) want (dev_abc, true)", svc.lastGetInput.PlayerID, svc.lastGetInput.IsAnonymous)
	}
	if svc.lastGetInput.Date != "2026-05-02" {
		t.Errorf("GetInput.Date: got %q want 2026-05-02", svc.lastGetInput.Date)
	}
}

// ---------------------------------------------------------------------------
// GET sentinel error → status code mappings
// ---------------------------------------------------------------------------

func TestDailyGetHandler_SentinelErrors(t *testing.T) {
	cases := []struct {
		name     string
		svcErr   error
		wantCode int
		wantMsg  string
	}{
		{"ErrInvalidDate -> 400", dailysvc.ErrInvalidDate, http.StatusBadRequest, "invalid date"},
		{"ErrOutOfWindow -> 404", dailysvc.ErrOutOfWindow, http.StatusNotFound, "out of window"},
		{"ErrScheduleNotFinalized -> 404", dailysvc.ErrScheduleNotFinalized, http.StatusNotFound, "schedule not finalized"},
		{"ErrPoolExhausted -> 500", dailysvc.ErrPoolExhausted, http.StatusInternalServerError, "pool exhausted"},
		{"unknown error -> 500", errors.New("unexpected"), http.StatusInternalServerError, "internal error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			svc := &stubDailyService{getDailyErr: tc.svcErr}
			router := mountDailyGet(svc)
			req := httptest.NewRequest(http.MethodGet, "/api/daily/2026-05-02", http.NoBody)
			req.Header.Set("X-Device-Id", "dev_abc")
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Fatalf("status: got %d want %d", rec.Code, tc.wantCode)
			}
			if got := dailyErrorBody(t, rec); got != tc.wantMsg {
				t.Errorf("error body: got %q want %q", got, tc.wantMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// POST auth matrix
// ---------------------------------------------------------------------------

func TestDailySubmitHandler_AuthMatrix(t *testing.T) {
	cases := []struct {
		name       string
		userID     string
		deviceID   string
		wantStatus int
	}{
		{"no identity returns 401", "", "", http.StatusUnauthorized},
		{"device header alone passes (200)", "", "dev_abc", http.StatusOK},
		{"userID alone passes (200)", "user_xyz", "", http.StatusOK},
		{"userID wins over device (200)", "user_xyz", "dev_abc", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			rank := 1
			svc := &stubDailyService{submitDailyResult: &dailysvc.SubmitResult{
				ServerElapsedMs: 42000,
				LeaderboardRank: &rank,
			}}
			router := mountDailySubmit(svc)
			req := httptest.NewRequest(http.MethodPost, "/api/daily/2026-05-02/result", validSubmitBody())
			if tc.userID != "" {
				req = withDailyUserID(req, tc.userID)
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
		})
	}
}

// ---------------------------------------------------------------------------
// POST body validation (HTTP-level concerns)
// ---------------------------------------------------------------------------

func TestDailySubmitHandler_BodyValidation(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{"malformed JSON", `{not-json`, "invalid body"},
		{"missing outcome field", `{"playTimeMs":1000,"solution":[[1,0]]}`, "invalid outcome"},
		{"outcome not solved", `{"outcome":"skipped","playTimeMs":1000,"solution":[[1,0]]}`, "invalid outcome"},
		{"outcome null", `{"outcome":null,"playTimeMs":1000,"solution":[[1,0]]}`, "invalid outcome"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			svc := &stubDailyService{}
			router := mountDailySubmit(svc)
			req := httptest.NewRequest(http.MethodPost, "/api/daily/2026-05-02/result",
				bytes.NewBufferString(tc.body))
			req.Header.Set("X-Device-Id", "dev_abc")
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d want 400 (body=%q)", rec.Code, rec.Body.String())
			}
			if got := dailyErrorBody(t, rec); got != tc.wantMsg {
				t.Errorf("error body: got %q want %q", got, tc.wantMsg)
			}
			if svc.submitDailyCalled {
				t.Error("SubmitDaily must NOT be called when the request is rejected during validation")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// POST happy path
// ---------------------------------------------------------------------------

func TestDailySubmitHandler_HappyPath(t *testing.T) {
	// Arrange
	rank := 3
	result := &dailysvc.SubmitResult{ServerElapsedMs: 42000, LeaderboardRank: &rank}
	svc := &stubDailyService{submitDailyResult: result}
	router := mountDailySubmit(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/2026-05-02/result", validSubmitBody())
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	var body struct {
		ServerElapsedMs int64 `json:"serverElapsedMs"`
		LeaderboardRank *int  `json:"leaderboardRank"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ServerElapsedMs != 42000 {
		t.Errorf("serverElapsedMs: got %d want 42000", body.ServerElapsedMs)
	}
	if body.LeaderboardRank == nil || *body.LeaderboardRank != 3 {
		t.Errorf("leaderboardRank: got %v want 3", body.LeaderboardRank)
	}
	if !svc.submitDailyCalled {
		t.Error("SubmitDaily was not called")
	}
	if svc.lastSubmitInput.PlayerID != "dev_abc" || !svc.lastSubmitInput.IsAnonymous {
		t.Errorf("SubmitInput: got (%q, anon=%t) want (dev_abc, true)", svc.lastSubmitInput.PlayerID, svc.lastSubmitInput.IsAnonymous)
	}
	if svc.lastSubmitInput.Date != "2026-05-02" {
		t.Errorf("SubmitInput.Date: got %q want 2026-05-02", svc.lastSubmitInput.Date)
	}
	if svc.lastSubmitInput.PlayTimeMs != 42000 {
		t.Errorf("SubmitInput.PlayTimeMs: got %d want 42000", svc.lastSubmitInput.PlayTimeMs)
	}
}

// ---------------------------------------------------------------------------
// POST sentinel error → status code mappings
// ---------------------------------------------------------------------------

func TestDailySubmitHandler_SentinelErrors(t *testing.T) {
	cases := []struct {
		name     string
		svcErr   error
		wantCode int
		wantMsg  string
	}{
		{"ErrInvalidDate -> 400", dailysvc.ErrInvalidDate, http.StatusBadRequest, "invalid date"},
		{"ErrOutOfWindow -> 404", dailysvc.ErrOutOfWindow, http.StatusNotFound, "out of window"},
		{"ErrScheduleNotFinalized -> 404", dailysvc.ErrScheduleNotFinalized, http.StatusNotFound, "schedule not finalized"},
		{"ErrPoolExhausted -> 500", dailysvc.ErrPoolExhausted, http.StatusInternalServerError, "pool exhausted"},
		{"ErrPlayNotStarted -> 400", dailysvc.ErrPlayNotStarted, http.StatusBadRequest, "play not started"},
		{"ErrAlreadySolved -> 409", dailysvc.ErrAlreadySolved, http.StatusConflict, "already solved"},
		{"ErrInvalidSolution -> 400", dailysvc.ErrInvalidSolution, http.StatusBadRequest, "invalid solution"},
		{"ErrInvalidPlayTime -> 400", dailysvc.ErrInvalidPlayTime, http.StatusBadRequest, "invalid playTimeMs"},
		{"ErrNegativeClockSkew -> 500", dailysvc.ErrNegativeClockSkew, http.StatusInternalServerError, "internal error"},
		{"unknown error -> 500", errors.New("unexpected"), http.StatusInternalServerError, "internal error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			svc := &stubDailyService{submitDailyErr: tc.svcErr}
			router := mountDailySubmit(svc)
			req := httptest.NewRequest(http.MethodPost, "/api/daily/2026-05-02/result", validSubmitBody())
			req.Header.Set("X-Device-Id", "dev_abc")
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Fatalf("status: got %d want %d", rec.Code, tc.wantCode)
			}
			if got := dailyErrorBody(t, rec); got != tc.wantMsg {
				t.Errorf("error body: got %q want %q", got, tc.wantMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// POST: anonymous via X-Device-Id gets through (resolveDailyPlayer plumbing)
// ---------------------------------------------------------------------------

func TestDailySubmitHandler_AnonymousDeviceIDPlumbing(t *testing.T) {
	// Arrange
	svc := &stubDailyService{submitDailyResult: &dailysvc.SubmitResult{ServerElapsedMs: 1000}}
	router := mountDailySubmit(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/2026-05-02/result", validSubmitBody())
	req.Header.Set("X-Device-Id", "device-plumbing-test")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	if !svc.submitDailyCalled {
		t.Fatal("SubmitDaily was not called")
	}
	if svc.lastSubmitInput.PlayerID != "device-plumbing-test" {
		t.Errorf("PlayerID: got %q want device-plumbing-test", svc.lastSubmitInput.PlayerID)
	}
	if !svc.lastSubmitInput.IsAnonymous {
		t.Error("IsAnonymous: got false, want true for device-header player")
	}
}
