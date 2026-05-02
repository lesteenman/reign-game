package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// fakeDailyRepo is a minimal in-memory stand-in for the daily-puzzle
// repository methods the GET handler calls. Each field controls one
// downstream call; counters expose call counts so race / refresh
// behavior can be asserted without time-of-day flakiness.
type fakeDailyRepo struct {
	scheduleByDate map[string]*repository.ScheduleRecord
	scheduleErr    error

	// puzzleByID is keyed by puzzleID alone; the handler resolves size
	// and mode from the schedule's sourcePartition before calling
	// GetPuzzle, so the fake doesn't need to model the full key.
	puzzleByID map[string]*repository.PuzzleRecord
	puzzleErr  error

	// playSequence is a queue of GetPlay return values consumed in
	// order — supports the race-loser path where GetPlay is called
	// twice (miss, then hit).
	playSequence    []*repository.PlayRecord
	playSequenceErr error

	// putPlayErr controls the PutPlayStartedIfAbsent return value;
	// nil means the put succeeded and a started row should be
	// synthesized for the test's assertions.
	putPlayErr error

	getScheduleCalls int
	getPuzzleCalls   int
	getPlayCalls     int
	putPlayCalls     int

	// putPlayCaptured holds the arguments of the last
	// PutPlayStartedIfAbsent call so the test can assert assignedAt
	// was stamped from the handler's clock (DP-19).
	putPlayCaptured struct {
		playerID   string
		date       string
		puzzleID   string
		assignedAt time.Time
	}
}

func (f *fakeDailyRepo) GetSchedule(_ context.Context, date string) (*repository.ScheduleRecord, error) {
	f.getScheduleCalls++
	if f.scheduleErr != nil {
		return nil, f.scheduleErr
	}
	return f.scheduleByDate[date], nil
}

func (f *fakeDailyRepo) GetPuzzle(_ context.Context, _ int, _, puzzleID string) (*repository.PuzzleRecord, error) {
	f.getPuzzleCalls++
	if f.puzzleErr != nil {
		return nil, f.puzzleErr
	}
	return f.puzzleByID[puzzleID], nil
}

func (f *fakeDailyRepo) GetPlay(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
	idx := f.getPlayCalls
	f.getPlayCalls++
	if f.playSequenceErr != nil {
		return nil, f.playSequenceErr
	}
	if idx >= len(f.playSequence) {
		return nil, nil
	}
	return f.playSequence[idx], nil
}

func (f *fakeDailyRepo) PutPlayStartedIfAbsent(_ context.Context, playerID, date, puzzleID string, assignedAt time.Time) error {
	f.putPlayCalls++
	f.putPlayCaptured.playerID = playerID
	f.putPlayCaptured.date = date
	f.putPlayCaptured.puzzleID = puzzleID
	f.putPlayCaptured.assignedAt = assignedAt
	return f.putPlayErr
}

// mountDailyWithRepo mounts the daily GET handler at /api/daily/{date}
// against a caller-supplied repo (typically *fakeDailyRepo). Used by
// chunk-2 tests that exercise the schedule + PLAY paths.
func mountDailyWithRepo(repo handler.DailyRepo) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/daily/{date}", handler.DailyGetHandler(repo, fixedClock()).ServeHTTP)
	return r
}

// fixedClock returns a clock function pinned to 2026-05-02 12:00:00 UTC.
// All date-window assertions in this file are anchored to that instant:
// today=2026-05-02, yesterday=2026-05-01.
func fixedClock() func() time.Time {
	t := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

// mountDaily mounts the daily GET handler at /api/daily/{date} with a
// nil repo. Used by tests that fail before any repo call (auth, date
// validation) — passing a real fake would just be noise.
func mountDaily() *chi.Mux {
	return mountDailyWithRepo(nil)
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

// dailyOKBody is the JSON shape the GET 200 response decodes into.
// Mirrors DP-09 exactly; pointer fields capture omitted-when-absent
// semantics for serverElapsedMs and submittedAt.
type dailyOKBody struct {
	PuzzleID        string  `json:"puzzleId"`
	Grid            int     `json:"grid"`
	Regions         [][]int `json:"regions"`
	AssignedAt      string  `json:"assignedAt"`
	Outcome         string  `json:"outcome"`
	ServerElapsedMs *int64  `json:"serverElapsedMs,omitempty"`
	SubmittedAt     *string `json:"submittedAt,omitempty"`
}

// scheduleFixture builds a schedule row for date pointing at puzzleID
// in the 9#standard partition. Used by the schedule+PLAY tests below.
func scheduleFixture(date, puzzleID string) *repository.ScheduleRecord {
	return &repository.ScheduleRecord{
		Date:            date,
		PuzzleID:        puzzleID,
		AssignedAt:      "2026-05-02T00:00:00Z",
		SourcePartition: "9#standard",
	}
}

// puzzleFixture builds a PuzzleRecord with a 9x9 region map keyed by
// puzzleID. The region map is intentionally trivial — all cells in
// region 0 — so tests can compare with reflect.DeepEqual cheaply.
func puzzleFixture(puzzleID string) *repository.PuzzleRecord {
	regions := make([][]int, 9)
	for i := range regions {
		regions[i] = make([]int, 9)
	}
	return &repository.PuzzleRecord{
		ID:        puzzleID,
		GridSize:  9,
		Mode:      "standard",
		RegionMap: regions,
		// Solution intentionally non-empty so tests fail loudly if the
		// handler accidentally serializes it.
		Solution: [][]bool{{true}},
	}
}

func TestDailyGetHandler_ScheduleAbsent(t *testing.T) {
	// Arrange
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{},
	}
	router := mountDailyWithRepo(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/2026-05-02", nil)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want 404 (body=%q)", rec.Code, rec.Body.String())
	}
	if got := readErrorBody(t, rec); got != "schedule not finalized" {
		t.Fatalf("error body: got %q want %q", got, "schedule not finalized")
	}
	if repo.getPuzzleCalls != 0 {
		t.Fatalf("getPuzzle should not be called when schedule absent (calls=%d)", repo.getPuzzleCalls)
	}
}

func TestDailyGetHandler_FirstGet_CreatesPlayRow(t *testing.T) {
	// Arrange
	puzzleID := "puzzle-001"
	date := "2026-05-02"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: scheduleFixture(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: puzzleFixture(puzzleID)},
		// playSequence empty → GetPlay returns (nil, nil) on first call;
		// PutPlayStartedIfAbsent succeeds (putPlayErr=nil); no second
		// GetPlay needed.
	}
	router := mountDailyWithRepo(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, nil)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	var body dailyOKBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.PuzzleID != puzzleID {
		t.Fatalf("puzzleId: got %q want %q", body.PuzzleID, puzzleID)
	}
	if body.Grid != 9 {
		t.Fatalf("grid: got %d want 9", body.Grid)
	}
	if len(body.Regions) != 9 {
		t.Fatalf("regions length: got %d want 9", len(body.Regions))
	}
	if body.Outcome != "started" {
		t.Fatalf("outcome: got %q want %q", body.Outcome, "started")
	}
	wantAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	if body.AssignedAt != wantAt {
		t.Fatalf("assignedAt: got %q want %q", body.AssignedAt, wantAt)
	}
	if body.ServerElapsedMs != nil {
		t.Fatalf("serverElapsedMs must be omitted when not solved (got %v)", *body.ServerElapsedMs)
	}
	if body.SubmittedAt != nil {
		t.Fatalf("submittedAt must be omitted when not solved (got %v)", *body.SubmittedAt)
	}
	if repo.putPlayCalls != 1 {
		t.Fatalf("PutPlayStartedIfAbsent calls: got %d want 1", repo.putPlayCalls)
	}
	if repo.putPlayCaptured.puzzleID != puzzleID {
		t.Fatalf("PutPlayStartedIfAbsent puzzleID: got %q want %q", repo.putPlayCaptured.puzzleID, puzzleID)
	}
	if repo.putPlayCaptured.playerID != "dev_abc" {
		t.Fatalf("PutPlayStartedIfAbsent playerID: got %q want dev_abc", repo.putPlayCaptured.playerID)
	}
}

func TestDailyGetHandler_FirstGet_RaceLoser(t *testing.T) {
	// Arrange
	puzzleID := "puzzle-002"
	date := "2026-05-02"
	winnerAssignedAt := "2026-05-02T11:59:00Z" // strictly before the fixedClock instant
	winnerRow := &repository.PlayRecord{
		PlayerID:   "dev_abc",
		Date:       date,
		Outcome:    "started",
		AssignedAt: winnerAssignedAt,
		PuzzleID:   puzzleID,
	}
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: scheduleFixture(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: puzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{nil, winnerRow}, // miss, then hit
		putPlayErr:     repository.ErrPlayAlreadyExists,
	}
	router := mountDailyWithRepo(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, nil)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	var body dailyOKBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// DP-19: race loser must surface the winner's assignedAt, NOT
	// the value we tried to put.
	if body.AssignedAt != winnerAssignedAt {
		t.Fatalf("assignedAt: got %q want %q (winner's stamp)", body.AssignedAt, winnerAssignedAt)
	}
	if body.Outcome != "started" {
		t.Fatalf("outcome: got %q want started", body.Outcome)
	}
	if repo.getPlayCalls != 2 {
		t.Fatalf("GetPlay calls: got %d want 2 (miss+winner-read)", repo.getPlayCalls)
	}
	if repo.putPlayCalls != 1 {
		t.Fatalf("PutPlayStartedIfAbsent calls: got %d want 1", repo.putPlayCalls)
	}
}

func TestDailyGetHandler_RefreshGet_PreservesAssignedAt(t *testing.T) {
	// Arrange
	puzzleID := "puzzle-003"
	date := "2026-05-02"
	existingAssignedAt := "2026-05-02T08:00:00Z"
	existing := &repository.PlayRecord{
		PlayerID:   "dev_abc",
		Date:       date,
		Outcome:    "started",
		AssignedAt: existingAssignedAt,
		PuzzleID:   puzzleID,
	}
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: scheduleFixture(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: puzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{existing},
	}
	router := mountDailyWithRepo(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, nil)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	var body dailyOKBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.AssignedAt != existingAssignedAt {
		t.Fatalf("assignedAt: got %q want %q (must be preserved on refresh)", body.AssignedAt, existingAssignedAt)
	}
	if repo.putPlayCalls != 0 {
		t.Fatalf("PutPlayStartedIfAbsent must NOT be called on refresh (calls=%d)", repo.putPlayCalls)
	}
	if repo.getPlayCalls != 1 {
		t.Fatalf("GetPlay calls: got %d want 1 (no race retry)", repo.getPlayCalls)
	}
}

func TestDailyGetHandler_AlreadySolved_IncludesElapsedAndSubmitted(t *testing.T) {
	// Arrange
	puzzleID := "puzzle-004"
	date := "2026-05-02"
	solvedRow := &repository.PlayRecord{
		PlayerID:        "dev_abc",
		Date:            date,
		Outcome:         "solved",
		AssignedAt:      "2026-05-02T08:00:00Z",
		SubmittedAt:     "2026-05-02T08:00:42Z",
		PuzzleID:        puzzleID,
		ServerElapsedMs: 42000,
	}
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: scheduleFixture(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: puzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{solvedRow},
	}
	router := mountDailyWithRepo(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, nil)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	var body dailyOKBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Outcome != "solved" {
		t.Fatalf("outcome: got %q want solved", body.Outcome)
	}
	if body.ServerElapsedMs == nil || *body.ServerElapsedMs != 42000 {
		t.Fatalf("serverElapsedMs: got %v want 42000", body.ServerElapsedMs)
	}
	if body.SubmittedAt == nil || *body.SubmittedAt != "2026-05-02T08:00:42Z" {
		t.Fatalf("submittedAt: got %v want %q", body.SubmittedAt, "2026-05-02T08:00:42Z")
	}
}

func TestDailyGetHandler_PuzzleMissing_500(t *testing.T) {
	// Arrange — schedule references a puzzle that no longer exists.
	puzzleID := "ghost-puzzle"
	date := "2026-05-02"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: scheduleFixture(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{}, // empty → GetPuzzle returns (nil, nil)
	}
	router := mountDailyWithRepo(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, nil)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want 500 (body=%q)", rec.Code, rec.Body.String())
	}
}

func TestDailyGetHandler_SourcePartitionMalformed_500(t *testing.T) {
	// Arrange — schedule sourcePartition cannot be parsed into size+mode.
	date := "2026-05-02"
	bad := scheduleFixture(date, "puzzle-005")
	bad.SourcePartition = "garbage"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: bad},
	}
	router := mountDailyWithRepo(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, nil)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want 500 (body=%q)", rec.Code, rec.Body.String())
	}
	if repo.getPuzzleCalls != 0 {
		t.Fatalf("GetPuzzle must not be called when sourcePartition is malformed (calls=%d)", repo.getPuzzleCalls)
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
