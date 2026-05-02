package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	// mu guards every counter / capture field below. The R2 fan-out
	// (perf: parallelize GetSchedule+GetPlay in daily handlers) calls
	// GetSchedule and GetPlay from two goroutines, so without this lock
	// `go test -race` flags concurrent writes to getScheduleCalls /
	// getPlayCalls. Locking everything keeps the fake unconditionally
	// thread-safe regardless of which methods a future handler decides
	// to fan out.
	mu sync.Mutex

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

	// candidate is returned by GetCandidate; nil means "no candidate".
	// candidateErr overrides the value when non-nil.
	candidate    *repository.CandidateRecord
	candidateErr error

	// finalizeErr is returned by FinalizeDailyTransaction. Nil means
	// success; repository.ErrScheduleAlreadyFinalized lets the test
	// exercise the race-loser branch of SyncFinalizeForToday.
	finalizeErr error

	getCandidateCalls int
	finalizeCalls     int

	// finalizeCaptured holds the arguments of the last
	// FinalizeDailyTransaction call so the test can assert mode +
	// sourcePartition routing (confirm vs. recycle).
	finalizeCaptured struct {
		date            string
		puzzleID        string
		sourcePartition string
		mode            repository.FinalizeMode
	}

	// submitErr controls SubmitPlayTransactionally; nil is success.
	submitErr   error
	submitCalls int

	// submitCaptured records the playerID, date, and SubmitInput passed
	// to SubmitPlayTransactionally so the test can assert auth-derived
	// flags (IsAnonymous, UserID) and the timestamps line up with what
	// the handler computed from the PLAY row + clock.
	submitCaptured struct {
		playerID string
		date     string
		input    repository.SubmitInput
	}

	// rankResult / rankErr control LeaderboardRank. The handler omits
	// the field on error and returns 200 anyway — submission is the
	// authoritative write; rank is best-effort.
	rankResult int
	rankErr    error
	rankCalls  int

	rankCaptured struct {
		date      string
		elapsedMs int64
		userID    string
	}
}

func (f *fakeDailyRepo) GetSchedule(_ context.Context, date string) (*repository.ScheduleRecord, error) {
	f.mu.Lock()
	f.getScheduleCalls++
	err := f.scheduleErr
	rec := f.scheduleByDate[date]
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (f *fakeDailyRepo) GetPuzzle(_ context.Context, _ int, _, puzzleID string) (*repository.PuzzleRecord, error) {
	f.mu.Lock()
	f.getPuzzleCalls++
	err := f.puzzleErr
	rec := f.puzzleByID[puzzleID]
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (f *fakeDailyRepo) GetPlay(_ context.Context, _, _ string) (*repository.PlayRecord, error) {
	f.mu.Lock()
	idx := f.getPlayCalls
	f.getPlayCalls++
	err := f.playSequenceErr
	var rec *repository.PlayRecord
	if idx < len(f.playSequence) {
		rec = f.playSequence[idx]
	}
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (f *fakeDailyRepo) PutPlayStartedIfAbsent(_ context.Context, playerID, date, puzzleID string, assignedAt time.Time) error {
	f.mu.Lock()
	f.putPlayCalls++
	f.putPlayCaptured.playerID = playerID
	f.putPlayCaptured.date = date
	f.putPlayCaptured.puzzleID = puzzleID
	f.putPlayCaptured.assignedAt = assignedAt
	err := f.putPlayErr
	f.mu.Unlock()
	return err
}

func (f *fakeDailyRepo) GetCandidate(_ context.Context) (*repository.CandidateRecord, error) {
	f.mu.Lock()
	f.getCandidateCalls++
	err := f.candidateErr
	rec := f.candidate
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (f *fakeDailyRepo) FinalizeDailyTransaction(_ context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error {
	f.mu.Lock()
	f.finalizeCalls++
	f.finalizeCaptured.date = date
	f.finalizeCaptured.puzzleID = puzzleID
	f.finalizeCaptured.sourcePartition = sourcePartition
	f.finalizeCaptured.mode = mode
	err := f.finalizeErr
	f.mu.Unlock()
	return err
}

// ListApprovedPool + PutCandidateIfAbsent are stubs to satisfy the
// DailyRepo interface (which is a structural superset of daily.Repo).
// The GET handler's sync-fallback path is not exercised by these tests
// — schedule presence is set up explicitly per test — so these stubs
// return zero values and never fire in practice.
func (f *fakeDailyRepo) ListApprovedPool(_ context.Context, _ int, _ string, _ bool, _ time.Time) ([]repository.PuzzleRecord, error) {
	return nil, nil
}

func (f *fakeDailyRepo) PutCandidateIfAbsent(_ context.Context, _, _ string) error {
	return nil
}

// submitErr is returned by SubmitPlayTransactionally. Nil means success.
// repository.ErrPlayNotInStartedState exercises the race-loser 409 path.
func (f *fakeDailyRepo) SubmitPlayTransactionally(_ context.Context, playerID, date string, submission *repository.SubmitInput) error {
	f.mu.Lock()
	f.submitCalls++
	f.submitCaptured.playerID = playerID
	f.submitCaptured.date = date
	if submission != nil {
		f.submitCaptured.input = *submission
	}
	err := f.submitErr
	f.mu.Unlock()
	return err
}

// LeaderboardRank returns rankResult, rankErr. Default zero values mean
// rank=0, err=nil — fine for anonymous tests where the handler MUST NOT
// call this method anyway.
func (f *fakeDailyRepo) LeaderboardRank(_ context.Context, date string, elapsedMs int64, userID string) (int, error) {
	f.mu.Lock()
	f.rankCalls++
	f.rankCaptured.date = date
	f.rankCaptured.elapsedMs = elapsedMs
	f.rankCaptured.userID = userID
	rank := f.rankResult
	err := f.rankErr
	f.mu.Unlock()
	if err != nil {
		return 0, err
	}
	return rank, nil
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
			name:       "device header alone passes auth (200)",
			userID:     "",
			deviceID:   "dev_abc",
			wantStatus: http.StatusOK,
		},
		{
			name:       "userID alone passes auth (200)",
			userID:     "user_xyz",
			deviceID:   "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "userID wins when both present (200)",
			userID:     "user_xyz",
			deviceID:   "dev_abc",
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			puzzleID := "puzzle-auth"
			date := "2026-05-02"
			repo := &fakeDailyRepo{
				scheduleByDate: map[string]*repository.ScheduleRecord{date: scheduleFixture(date, puzzleID)},
				puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: puzzleFixture(puzzleID)},
			}
			router := mountDailyWithRepo(repo)
			req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, http.NoBody)
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

// poolExhaustedErrorBody is the canonical phrase the GET handler
// returns in the 500 body when sync-fallback hits ErrPoolExhausted
// (DP-16). Keep this exactly in sync with handler/daily.go's
// poolExhaustedMessage constant.
const poolExhaustedErrorBody = "pool exhausted"

func TestDailyGetHandler_SyncFallback_Engaged_ConfirmPath(t *testing.T) {
	// Arrange — today's schedule missing; yesterday solved>0 + candidate
	// present means the decision tree picks confirm.
	puzzleID := "puzzle-confirm"
	yesterdayPuzzleID := "puzzle-yesterday"
	today := "2026-05-02"
	yesterday := "2026-05-01"
	yesterdayRow := &repository.ScheduleRecord{
		Date:            yesterday,
		PuzzleID:        yesterdayPuzzleID,
		AssignedAt:      "2026-05-01T00:00:00Z",
		SourcePartition: "9#standard",
		Counters:        repository.ScheduleCounters{Started: 5, Solved: 3},
	}
	finalizedToday := &repository.ScheduleRecord{
		Date:            today,
		PuzzleID:        puzzleID,
		AssignedAt:      "2026-05-02T12:00:00Z",
		SourcePartition: "9#standard",
	}
	repo := &fakeDailyRepo{
		// today absent on first read; sync-fallback finalize must
		// re-read it, so the fake materializes today's row only when
		// FinalizeDailyTransaction is invoked.
		scheduleByDate: map[string]*repository.ScheduleRecord{yesterday: yesterdayRow},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: puzzleFixture(puzzleID)},
		candidate: &repository.CandidateRecord{
			PuzzleID:        puzzleID,
			QueuedAt:        "2026-05-01T18:00:00Z",
			SourcePartition: "9#standard",
		},
	}
	// Activate today's row inside FinalizeDailyTransaction so the
	// sync-fallback re-read sees it. Wrapping the success path keeps
	// the seeding co-located with the test for readability.
	repo.finalizeErr = nil
	origRepoMutator := repo // closure capture
	finalizeOnce := func() {
		origRepoMutator.scheduleByDate[today] = finalizedToday
	}
	wrapper := &finalizeWrapperRepo{fakeDailyRepo: repo, onFinalize: finalizeOnce}
	router := mountDailyWithRepo(wrapper)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+today, http.NoBody)
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
		t.Fatalf("puzzleId: got %q want %q (confirm path returns candidate)", body.PuzzleID, puzzleID)
	}
	if body.AssignedAt != finalizedToday.AssignedAt {
		t.Fatalf("assignedAt: got %q want %q (canonical from re-read)", body.AssignedAt, finalizedToday.AssignedAt)
	}
	if repo.getCandidateCalls != 1 {
		t.Fatalf("GetCandidate calls: got %d want 1", repo.getCandidateCalls)
	}
	if repo.finalizeCalls != 1 {
		t.Fatalf("FinalizeDailyTransaction calls: got %d want 1", repo.finalizeCalls)
	}
	if repo.finalizeCaptured.mode != repository.FinalizeModeConfirm {
		t.Fatalf("FinalizeDailyTransaction mode: got %q want %q", repo.finalizeCaptured.mode, repository.FinalizeModeConfirm)
	}
	if repo.finalizeCaptured.puzzleID != puzzleID {
		t.Fatalf("FinalizeDailyTransaction puzzleID: got %q want %q", repo.finalizeCaptured.puzzleID, puzzleID)
	}
	// GetSchedule(today) on entry, GetSchedule(yesterday) inside sync,
	// then GetSchedule(today) again to fetch the canonical row = 3.
	if repo.getScheduleCalls != 3 {
		t.Fatalf("GetSchedule calls: got %d want 3 (entry + yesterday + canonical re-read)", repo.getScheduleCalls)
	}
}

func TestDailyGetHandler_SyncFallback_Engaged_RecyclePath(t *testing.T) {
	// Arrange — today missing, candidate absent, yesterday present.
	// Decision tree picks recycle and reuses yesterday's puzzleID.
	yesterdayPuzzleID := "puzzle-yesterday-recycle"
	today := "2026-05-02"
	yesterday := "2026-05-01"
	yesterdayRow := &repository.ScheduleRecord{
		Date:            yesterday,
		PuzzleID:        yesterdayPuzzleID,
		AssignedAt:      "2026-05-01T00:00:00Z",
		SourcePartition: "9#standard",
		Counters:        repository.ScheduleCounters{Started: 0, Solved: 0},
	}
	finalizedToday := &repository.ScheduleRecord{
		Date:            today,
		PuzzleID:        yesterdayPuzzleID,
		AssignedAt:      "2026-05-02T12:00:00Z",
		SourcePartition: "9#standard",
	}
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{yesterday: yesterdayRow},
		puzzleByID:     map[string]*repository.PuzzleRecord{yesterdayPuzzleID: puzzleFixture(yesterdayPuzzleID)},
		candidate:      nil, // pool empty
	}
	wrapper := &finalizeWrapperRepo{fakeDailyRepo: repo, onFinalize: func() {
		repo.scheduleByDate[today] = finalizedToday
	}}
	router := mountDailyWithRepo(wrapper)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+today, http.NoBody)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	if repo.finalizeCaptured.mode != repository.FinalizeModeRecycle {
		t.Fatalf("FinalizeDailyTransaction mode: got %q want %q", repo.finalizeCaptured.mode, repository.FinalizeModeRecycle)
	}
	if repo.finalizeCaptured.puzzleID != yesterdayPuzzleID {
		t.Fatalf("recycle path must reuse yesterday's puzzleID: got %q want %q", repo.finalizeCaptured.puzzleID, yesterdayPuzzleID)
	}
}

func TestDailyGetHandler_SyncFallback_PoolExhausted_500(t *testing.T) {
	// Arrange — today missing, candidate empty, yesterday missing.
	// Decision tree returns ErrPoolExhausted; handler must 500.
	today := "2026-05-02"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{},
		candidate:      nil,
	}
	router := mountDailyWithRepo(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+today, http.NoBody)
	req.Header.Set("X-Device-Id", "dev_abc")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want 500 (body=%q)", rec.Code, rec.Body.String())
	}
	if got := readErrorBody(t, rec); got != poolExhaustedErrorBody {
		t.Fatalf("error body: got %q want %q", got, poolExhaustedErrorBody)
	}
	if repo.finalizeCalls != 0 {
		t.Fatalf("FinalizeDailyTransaction must NOT be called when pool exhausted (calls=%d)", repo.finalizeCalls)
	}
}

func TestDailyGetHandler_SyncFallback_NotEngaged_Yesterday_404(t *testing.T) {
	// Arrange — yesterday's schedule missing. Sync-fallback only runs
	// for `today`; yesterday must 404 without engaging the algorithm.
	yesterday := "2026-05-01"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{},
	}
	router := mountDailyWithRepo(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+yesterday, http.NoBody)
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
	// Critical: sync-fallback must NOT engage for non-today dates.
	if repo.getCandidateCalls != 0 {
		t.Fatalf("GetCandidate must NOT be called for yesterday (calls=%d)", repo.getCandidateCalls)
	}
	if repo.finalizeCalls != 0 {
		t.Fatalf("FinalizeDailyTransaction must NOT be called for yesterday (calls=%d)", repo.finalizeCalls)
	}
	// Only the entry GetSchedule(yesterday) — no extra reads.
	if repo.getScheduleCalls != 1 {
		t.Fatalf("GetSchedule calls: got %d want 1 (no sync-fallback engagement)", repo.getScheduleCalls)
	}
}

func TestDailyGetHandler_SyncFallback_RaceLoser(t *testing.T) {
	// Arrange — today missing on entry; FinalizeDailyTransaction
	// returns ErrScheduleAlreadyFinalized (another writer won); the
	// final GetSchedule(today) returns the winner's row.
	today := "2026-05-02"
	yesterday := "2026-05-01"
	winnerPuzzleID := "puzzle-winner"
	winnerRow := &repository.ScheduleRecord{
		Date:            today,
		PuzzleID:        winnerPuzzleID,
		AssignedAt:      "2026-05-02T11:59:00Z",
		SourcePartition: "9#standard",
	}
	yesterdayRow := &repository.ScheduleRecord{
		Date:            yesterday,
		PuzzleID:        "puzzle-y",
		AssignedAt:      "2026-05-01T00:00:00Z",
		SourcePartition: "9#standard",
		Counters:        repository.ScheduleCounters{Started: 1, Solved: 1},
	}
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{yesterday: yesterdayRow},
		puzzleByID:     map[string]*repository.PuzzleRecord{winnerPuzzleID: puzzleFixture(winnerPuzzleID)},
		candidate: &repository.CandidateRecord{
			PuzzleID:        "puzzle-loser",
			QueuedAt:        "2026-05-01T18:00:00Z",
			SourcePartition: "9#standard",
		},
		finalizeErr: repository.ErrScheduleAlreadyFinalized,
	}
	wrapper := &finalizeWrapperRepo{fakeDailyRepo: repo, onFinalize: func() {
		// Winner's row materializes during the (failed-from-our-side)
		// finalize attempt — sync's re-read must surface it.
		repo.scheduleByDate[today] = winnerRow
	}}
	router := mountDailyWithRepo(wrapper)
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+today, http.NoBody)
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
	if body.PuzzleID != winnerPuzzleID {
		t.Fatalf("race loser must surface winner's puzzleId: got %q want %q", body.PuzzleID, winnerPuzzleID)
	}
	// Note: body.AssignedAt is the PLAY-row stamp (the per-player
	// "first GET" instant), not the schedule's assignedAt — see DP-19
	// and TestDailyGetHandler_FirstGet_RaceLoser for the play-row race
	// invariant. The race tested here is on the schedule row, proven
	// by puzzleId resolving to the winner's puzzle.
	if repo.finalizeCalls != 1 {
		t.Fatalf("FinalizeDailyTransaction must be attempted once: got %d", repo.finalizeCalls)
	}
	// Entry GetSchedule(today) -> sync GetSchedule(yesterday) -> sync
	// re-read GetSchedule(today) after the race-loser branch.
	if repo.getScheduleCalls != 3 {
		t.Fatalf("GetSchedule calls: got %d want 3 (entry + yesterday + canonical re-read)", repo.getScheduleCalls)
	}
}

// finalizeWrapperRepo decorates fakeDailyRepo so a test can mutate the
// schedule map mid-call inside FinalizeDailyTransaction (success or
// race-loser branches both need the canonical re-read of today to
// succeed). Embedding fakeDailyRepo keeps every other method
// identical so existing test fixtures continue to compile.
type finalizeWrapperRepo struct {
	*fakeDailyRepo
	onFinalize func()
}

func (w *finalizeWrapperRepo) FinalizeDailyTransaction(ctx context.Context, date, puzzleID, sourcePartition string, mode repository.FinalizeMode) error {
	if w.onFinalize != nil {
		w.onFinalize()
	}
	return w.fakeDailyRepo.FinalizeDailyTransaction(ctx, date, puzzleID, sourcePartition, mode)
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
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, http.NoBody)
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
	req := httptest.NewRequest(http.MethodGet, "/api/daily/"+date, http.NoBody)
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
	// In-window dates land on a 200 once the schedule + puzzle fakes
	// are seeded; out-of-window short-circuits before the repo is
	// touched (404), and malformed dates short-circuit at parse (400).
	cases := []struct {
		name       string
		date       string
		wantStatus int
		wantError  string
	}{
		{name: "today returns 200", date: "2026-05-02", wantStatus: http.StatusOK},
		{name: "yesterday returns 200", date: "2026-05-01", wantStatus: http.StatusOK},
		{name: "tomorrow returns 404 out-of-window", date: "2026-05-03", wantStatus: http.StatusNotFound, wantError: "out of window"},
		{name: "two days ago returns 404 out-of-window", date: "2026-04-30", wantStatus: http.StatusNotFound, wantError: "out of window"},
		{name: "calendar-impossible date returns 400", date: "2026-13-99", wantStatus: http.StatusBadRequest, wantError: "invalid date"},
		{name: "non-date string returns 400", date: "not-a-date", wantStatus: http.StatusBadRequest, wantError: "invalid date"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange — seed schedule+puzzle for both in-window dates
			// so the success-path rows can reach 200.
			puzzleID := "puzzle-date"
			repo := &fakeDailyRepo{
				scheduleByDate: map[string]*repository.ScheduleRecord{
					"2026-05-02": scheduleFixture("2026-05-02", puzzleID),
					"2026-05-01": scheduleFixture("2026-05-01", puzzleID),
				},
				puzzleByID: map[string]*repository.PuzzleRecord{puzzleID: puzzleFixture(puzzleID)},
			}
			router := mountDailyWithRepo(repo)
			req := httptest.NewRequest(http.MethodGet, "/api/daily/"+tc.date, http.NoBody)
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

// ------------------------------------------------------------------
// POST /api/daily/{date}/result — DailySubmitHandler tests
// ------------------------------------------------------------------

// solvedPuzzleFixture builds a 3x3 puzzle with a deterministic, valid
// solution layout — diagonal-style queen placement so the [][]bool
// pattern is unambiguous: cell(0,0), cell(1,1), cell(2,2) are true.
// 3x3 keeps body fixtures terse without losing rectangular shape.
func solvedPuzzleFixture(puzzleID string) *repository.PuzzleRecord {
	regions := [][]int{
		{0, 0, 0},
		{0, 0, 0},
		{0, 0, 0},
	}
	solution := [][]bool{
		{true, false, false},
		{false, true, false},
		{false, false, true},
	}
	return &repository.PuzzleRecord{
		ID:        puzzleID,
		GridSize:  3,
		Mode:      "standard",
		RegionMap: regions,
		Solution:  solution,
	}
}

// solved3x3Schedule mirrors scheduleFixture but returns a 3#standard
// partition matching solvedPuzzleFixture.
func solved3x3Schedule(date, puzzleID string) *repository.ScheduleRecord {
	return &repository.ScheduleRecord{
		Date:            date,
		PuzzleID:        puzzleID,
		AssignedAt:      "2026-05-02T00:00:00Z",
		SourcePartition: "3#standard",
	}
}

// startedPlayRow is the canonical "ready to submit" PLAY-row fixture.
// AssignedAt is set 30 seconds before the fixedClock instant so
// serverElapsedMs ends up at exactly 30000 — easy to assert.
func startedPlayRow(playerID, date, puzzleID string) *repository.PlayRecord {
	return &repository.PlayRecord{
		PlayerID:   playerID,
		Date:       date,
		Outcome:    "started",
		AssignedAt: "2026-05-02T11:59:30Z",
		PuzzleID:   puzzleID,
	}
}

// validSolvedBody is the JSON body for a valid 3x3 submission matching
// solvedPuzzleFixture's expected solution.
const validSolvedBody = `{"outcome":"solved","playTimeMs":29800,"solution":[[1,0,0],[0,1,0],[0,0,1]]}`

// mountDailySubmit mounts the submit handler at POST
// /api/daily/{date}/result against the supplied repo.
func mountDailySubmit(repo handler.DailyRepo) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/api/daily/{date}/result", handler.DailySubmitHandler(repo, fixedClock()).ServeHTTP)
	return r
}

// dailySubmitOKBody is the POST 200 response shape. leaderboardRank is
// pointer-typed so omitempty distinguishes anonymous (nil) from
// signed-in (set, even if 0 from a degenerate fake).
type dailySubmitOKBody struct {
	ServerElapsedMs int64 `json:"serverElapsedMs"`
	LeaderboardRank *int  `json:"leaderboardRank,omitempty"`
}

func TestDailySubmitHandler_AuthMatrix(t *testing.T) {
	// Arrange — same shape as the GET auth matrix; only the no-auth
	// row matters here because every authenticated row has its own
	// dedicated test below. The unauth row exists to lock in DP-14.
	repo := &fakeDailyRepo{}
	router := mountDailySubmit(repo)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/2026-05-02/result", strings.NewReader(validSolvedBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401 (body=%q)", rec.Code, rec.Body.String())
	}
	if got := readErrorBody(t, rec); got != "unauthenticated" {
		t.Fatalf("error body: got %q want %q", got, "unauthenticated")
	}
	if repo.submitCalls != 0 {
		t.Fatalf("SubmitPlayTransactionally must NOT be called when unauth (calls=%d)", repo.submitCalls)
	}
}

func TestDailySubmitHandler_DateMatrix(t *testing.T) {
	cases := []struct {
		name       string
		date       string
		wantStatus int
		wantError  string
	}{
		{name: "tomorrow returns 404 out-of-window", date: "2026-05-03", wantStatus: http.StatusNotFound, wantError: "out of window"},
		{name: "two days ago returns 404", date: "2026-04-30", wantStatus: http.StatusNotFound, wantError: "out of window"},
		{name: "calendar-impossible date returns 400", date: "2026-13-99", wantStatus: http.StatusBadRequest, wantError: "invalid date"},
		{name: "non-date string returns 400", date: "not-a-date", wantStatus: http.StatusBadRequest, wantError: "invalid date"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			repo := &fakeDailyRepo{}
			router := mountDailySubmit(repo)
			req := httptest.NewRequest(http.MethodPost, "/api/daily/"+tc.date+"/result", strings.NewReader(validSolvedBody))
			req.Header.Set("X-Device-Id", "dev_abc")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantStatus {
				t.Fatalf("status: got %d want %d (body=%q)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if got := readErrorBody(t, rec); got != tc.wantError {
				t.Fatalf("error body: got %q want %q", got, tc.wantError)
			}
			if repo.submitCalls != 0 {
				t.Fatalf("SubmitPlayTransactionally must NOT be called for invalid date (calls=%d)", repo.submitCalls)
			}
		})
	}
}

func TestDailySubmitHandler_BodyValidation(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantError string
	}{
		{name: "empty body", body: ``, wantError: "invalid body"},
		{name: "wrong outcome", body: `{"outcome":"skipped","playTimeMs":1000,"solution":[[1,0,0],[0,1,0],[0,0,1]]}`, wantError: "invalid outcome"},
		{name: "missing outcome", body: `{"playTimeMs":1000,"solution":[[1,0,0],[0,1,0],[0,0,1]]}`, wantError: "invalid outcome"},
		{name: "negative playTimeMs", body: `{"outcome":"solved","playTimeMs":-5,"solution":[[1,0,0],[0,1,0],[0,0,1]]}`, wantError: "invalid playTimeMs"},
		{name: "non-rectangular solution", body: `{"outcome":"solved","playTimeMs":1000,"solution":[[1,0,0],[0,1],[0,0,1]]}`, wantError: "invalid solution"},
		{name: "cell value 2", body: `{"outcome":"solved","playTimeMs":1000,"solution":[[1,0,0],[0,2,0],[0,0,1]]}`, wantError: "invalid solution"},
		{name: "empty solution", body: `{"outcome":"solved","playTimeMs":1000,"solution":[]}`, wantError: "invalid solution"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange — auth + date + repo all good; only the body is malformed.
			puzzleID := "puzzle-body"
			date := "2026-05-02"
			player := "dev_abc"
			repo := &fakeDailyRepo{
				scheduleByDate: map[string]*repository.ScheduleRecord{date: solved3x3Schedule(date, puzzleID)},
				puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: solvedPuzzleFixture(puzzleID)},
				playSequence:   []*repository.PlayRecord{startedPlayRow(player, date, puzzleID)},
			}
			router := mountDailySubmit(repo)
			req := httptest.NewRequest(http.MethodPost, "/api/daily/"+date+"/result", strings.NewReader(tc.body))
			req.Header.Set("X-Device-Id", player)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d want 400 (body=%q)", rec.Code, rec.Body.String())
			}
			if got := readErrorBody(t, rec); got != tc.wantError {
				t.Fatalf("error body: got %q want %q", got, tc.wantError)
			}
			if repo.submitCalls != 0 {
				t.Fatalf("SubmitPlayTransactionally must NOT be called on body validation failure (calls=%d)", repo.submitCalls)
			}
		})
	}
}

func TestDailySubmitHandler_PlayNotStarted_400(t *testing.T) {
	// Arrange — GetPlay returns (nil, nil). Player must GET first; we
	// reject the submit before touching the transaction.
	puzzleID := "puzzle-no-play"
	date := "2026-05-02"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: solved3x3Schedule(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: solvedPuzzleFixture(puzzleID)},
		// playSequence empty → GetPlay returns nil
	}
	router := mountDailySubmit(repo)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/"+date+"/result", strings.NewReader(validSolvedBody))
	req.Header.Set("X-Device-Id", "dev_abc")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400 (body=%q)", rec.Code, rec.Body.String())
	}
	if got := readErrorBody(t, rec); got != "play not started" {
		t.Fatalf("error body: got %q want %q", got, "play not started")
	}
	if repo.submitCalls != 0 {
		t.Fatalf("SubmitPlayTransactionally must NOT be called when play not started (calls=%d)", repo.submitCalls)
	}
}

func TestDailySubmitHandler_AlreadySolved_409(t *testing.T) {
	// Arrange — PLAY row already solved. Idempotency without round-trip.
	puzzleID := "puzzle-solved"
	date := "2026-05-02"
	player := "dev_abc"
	solved := &repository.PlayRecord{
		PlayerID:        player,
		Date:            date,
		Outcome:         "solved",
		AssignedAt:      "2026-05-02T11:00:00Z",
		SubmittedAt:     "2026-05-02T11:00:42Z",
		PuzzleID:        puzzleID,
		ServerElapsedMs: 42000,
	}
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: solved3x3Schedule(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: solvedPuzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{solved},
	}
	router := mountDailySubmit(repo)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/"+date+"/result", strings.NewReader(validSolvedBody))
	req.Header.Set("X-Device-Id", player)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d want 409 (body=%q)", rec.Code, rec.Body.String())
	}
	if got := readErrorBody(t, rec); got != "already solved" {
		t.Fatalf("error body: got %q want %q", got, "already solved")
	}
	if repo.submitCalls != 0 {
		t.Fatalf("SubmitPlayTransactionally must NOT be called when already solved (calls=%d)", repo.submitCalls)
	}
}

func TestDailySubmitHandler_InvalidSolution_400(t *testing.T) {
	// Arrange — submitted grid does not match puzzle.Solution (cell flipped).
	puzzleID := "puzzle-mismatch"
	date := "2026-05-02"
	player := "dev_abc"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: solved3x3Schedule(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: solvedPuzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{startedPlayRow(player, date, puzzleID)},
	}
	router := mountDailySubmit(repo)
	wrongBody := `{"outcome":"solved","playTimeMs":1000,"solution":[[0,1,0],[0,1,0],[0,0,1]]}`
	req := httptest.NewRequest(http.MethodPost, "/api/daily/"+date+"/result", strings.NewReader(wrongBody))
	req.Header.Set("X-Device-Id", player)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400 (body=%q)", rec.Code, rec.Body.String())
	}
	if got := readErrorBody(t, rec); got != "invalid solution" {
		t.Fatalf("error body: got %q want %q", got, "invalid solution")
	}
	if repo.submitCalls != 0 {
		t.Fatalf("SubmitPlayTransactionally must NOT be called on invalid solution (calls=%d)", repo.submitCalls)
	}
}

func TestDailySubmitHandler_HappyPath_Anonymous(t *testing.T) {
	// Arrange
	puzzleID := "puzzle-anon"
	date := "2026-05-02"
	player := "dev_abc"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: solved3x3Schedule(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: solvedPuzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{startedPlayRow(player, date, puzzleID)},
	}
	router := mountDailySubmit(repo)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/"+date+"/result", strings.NewReader(validSolvedBody))
	req.Header.Set("X-Device-Id", player)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	var body dailySubmitOKBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ServerElapsedMs != 30000 {
		t.Fatalf("serverElapsedMs: got %d want 30000 (now - assignedAt)", body.ServerElapsedMs)
	}
	if body.LeaderboardRank != nil {
		t.Fatalf("leaderboardRank must be omitted for anonymous (got %d)", *body.LeaderboardRank)
	}
	if repo.submitCalls != 1 {
		t.Fatalf("SubmitPlayTransactionally calls: got %d want 1", repo.submitCalls)
	}
	if !repo.submitCaptured.input.IsAnonymous {
		t.Fatalf("SubmitInput.IsAnonymous: got false want true")
	}
	if repo.submitCaptured.input.UserID != "" {
		t.Fatalf("SubmitInput.UserID: got %q want empty (anonymous)", repo.submitCaptured.input.UserID)
	}
	if repo.submitCaptured.input.ClientMs != 29800 {
		t.Fatalf("SubmitInput.ClientMs: got %d want 29800", repo.submitCaptured.input.ClientMs)
	}
	if repo.submitCaptured.input.PuzzleID != puzzleID {
		t.Fatalf("SubmitInput.PuzzleID: got %q want %q", repo.submitCaptured.input.PuzzleID, puzzleID)
	}
	if repo.rankCalls != 0 {
		t.Fatalf("LeaderboardRank must NOT be called for anonymous (calls=%d)", repo.rankCalls)
	}
}

func TestDailySubmitHandler_HappyPath_SignedIn(t *testing.T) {
	// Arrange
	puzzleID := "puzzle-signed-in"
	date := "2026-05-02"
	userID := "user_xyz"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: solved3x3Schedule(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: solvedPuzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{startedPlayRow(userID, date, puzzleID)},
		rankResult:     7,
	}
	router := mountDailySubmit(repo)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/"+date+"/result", strings.NewReader(validSolvedBody))
	req = withUserID(req, userID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	var body dailySubmitOKBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ServerElapsedMs != 30000 {
		t.Fatalf("serverElapsedMs: got %d want 30000", body.ServerElapsedMs)
	}
	if body.LeaderboardRank == nil || *body.LeaderboardRank != 7 {
		t.Fatalf("leaderboardRank: got %v want 7", body.LeaderboardRank)
	}
	if repo.submitCaptured.input.IsAnonymous {
		t.Fatalf("SubmitInput.IsAnonymous: got true want false")
	}
	if repo.submitCaptured.input.UserID != userID {
		t.Fatalf("SubmitInput.UserID: got %q want %q", repo.submitCaptured.input.UserID, userID)
	}
	if repo.rankCalls != 1 {
		t.Fatalf("LeaderboardRank calls: got %d want 1", repo.rankCalls)
	}
	if repo.rankCaptured.elapsedMs != 30000 {
		t.Fatalf("LeaderboardRank elapsedMs: got %d want 30000", repo.rankCaptured.elapsedMs)
	}
	if repo.rankCaptured.userID != userID {
		t.Fatalf("LeaderboardRank userID: got %q want %q", repo.rankCaptured.userID, userID)
	}
}

func TestDailySubmitHandler_NegativeClockSkew_500(t *testing.T) {
	// Arrange — PLAY row's AssignedAt is 30 minutes in the future
	// relative to fixedClock() (2026-05-02 12:00:00 UTC). The handler
	// must refuse the submission rather than clamp serverElapsedMs to
	// 0, which would corrupt the leaderboard SK ordering.
	puzzleID := "puzzle-skew"
	date := "2026-05-02"
	player := "dev_skew"
	skewedPlay := startedPlayRow(player, date, puzzleID)
	skewedPlay.AssignedAt = "2026-05-02T12:30:00Z"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: solved3x3Schedule(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: solvedPuzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{skewedPlay},
	}
	router := mountDailySubmit(repo)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/"+date+"/result", strings.NewReader(validSolvedBody))
	req.Header.Set("X-Device-Id", player)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want 500 (body=%q)", rec.Code, rec.Body.String())
	}
	if got := readErrorBody(t, rec); got != "internal error" {
		t.Fatalf("error body: got %q want %q", got, "internal error")
	}
	if repo.submitCalls != 0 {
		t.Fatalf("SubmitPlayTransactionally must NOT be called on negative clock skew (calls=%d)", repo.submitCalls)
	}
}

func TestDailySubmitHandler_RaceLoser_409(t *testing.T) {
	// Arrange — concurrent submit won; transaction returns the sentinel.
	puzzleID := "puzzle-race"
	date := "2026-05-02"
	player := "dev_abc"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: solved3x3Schedule(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: solvedPuzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{startedPlayRow(player, date, puzzleID)},
		submitErr:      repository.ErrPlayNotInStartedState,
	}
	router := mountDailySubmit(repo)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/"+date+"/result", strings.NewReader(validSolvedBody))
	req.Header.Set("X-Device-Id", player)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d want 409 (body=%q)", rec.Code, rec.Body.String())
	}
	if got := readErrorBody(t, rec); got != "already solved" {
		t.Fatalf("error body: got %q want %q", got, "already solved")
	}
}

func TestDailySubmitHandler_RankFetchFailure_StillReturns200(t *testing.T) {
	// Arrange — submit succeeds; LeaderboardRank fails. The row is
	// already written; the handler must return 200 and omit the rank.
	puzzleID := "puzzle-rank-err"
	date := "2026-05-02"
	userID := "user_rank_err"
	repo := &fakeDailyRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{date: solved3x3Schedule(date, puzzleID)},
		puzzleByID:     map[string]*repository.PuzzleRecord{puzzleID: solvedPuzzleFixture(puzzleID)},
		playSequence:   []*repository.PlayRecord{startedPlayRow(userID, date, puzzleID)},
		rankErr:        errBoom,
	}
	router := mountDailySubmit(repo)
	req := httptest.NewRequest(http.MethodPost, "/api/daily/"+date+"/result", strings.NewReader(validSolvedBody))
	req = withUserID(req, userID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	var body dailySubmitOKBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ServerElapsedMs != 30000 {
		t.Fatalf("serverElapsedMs: got %d want 30000", body.ServerElapsedMs)
	}
	if body.LeaderboardRank != nil {
		t.Fatalf("leaderboardRank must be omitted on rank error (got %d)", *body.LeaderboardRank)
	}
	if repo.submitCalls != 1 {
		t.Fatalf("submit must have been called once (got %d)", repo.submitCalls)
	}
}

// errBoom is a sentinel test-only error used to exercise repo error
// paths without crafting a real DDB error.
var errBoom = errSentinel("boom")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
