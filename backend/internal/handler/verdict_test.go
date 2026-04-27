package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// fakeVerdictRepo is a hand-rolled in-memory VerdictRepo. It honors the
// row-family + summary-projection contract described in repository.md
// (VR-01..VR-10) so the handler tests exercise the same semantics the
// real DynamoDB repository implements.
type fakeVerdictRepo struct {
	puzzles  map[string]*repository.PuzzleRecord            // key = "size#mode#id"
	verdicts map[string]map[string]repository.VerdictRecord // outer = "size#mode#id", inner = SK ("role#raterId")

	// Failure injection knobs for VH-09 + error-propagation tests.
	getPuzzleErr  error
	putVerdictErr error
	recomputeErr  error
}

func newFakeVerdictRepo() *fakeVerdictRepo {
	return &fakeVerdictRepo{
		puzzles:  map[string]*repository.PuzzleRecord{},
		verdicts: map[string]map[string]repository.VerdictRecord{},
	}
}

func (f *fakeVerdictRepo) seed(size int, mode, id string) {
	key := fmt.Sprintf("%d#%s#%s", size, mode, id)
	f.puzzles[key] = &repository.PuzzleRecord{GridSize: size, Mode: mode, ID: id, Status: "ready"}
}

func (f *fakeVerdictRepo) GetPuzzle(_ context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error) {
	if f.getPuzzleErr != nil {
		return nil, f.getPuzzleErr
	}
	return f.puzzles[fmt.Sprintf("%d#%s#%s", size, mode, puzzleID)], nil
}

func (f *fakeVerdictRepo) PutVerdict(_ context.Context, v *repository.VerdictRecord) error {
	if f.putVerdictErr != nil {
		return f.putVerdictErr
	}
	puzKey := fmt.Sprintf("%d#%s#%s", v.GridSize, v.Mode, v.PuzzleID)
	if f.verdicts[puzKey] == nil {
		f.verdicts[puzKey] = map[string]repository.VerdictRecord{}
	}
	sk := fmt.Sprintf("%s#%s", v.RaterRole, v.RaterID)
	f.verdicts[puzKey][sk] = *v
	return nil
}

func (f *fakeVerdictRepo) RecomputeVerdictSummary(_ context.Context, size int, mode, puzzleID string) (repository.VerdictSummary, error) {
	if f.recomputeErr != nil {
		return repository.VerdictSummary{}, f.recomputeErr
	}
	puzKey := fmt.Sprintf("%d#%s#%s", size, mode, puzzleID)
	summary := repository.VerdictSummary{LastUpdatedAt: "2026-04-26T12:00:00Z"}
	// Iterate keys and index back into the map rather than `for _, v
	// := range f.verdicts[puzKey]` so the 144-byte VerdictRecord is
	// not copied per iteration (gocritic rangeValCopy).
	rows := f.verdicts[puzKey]
	for k := range rows {
		switch rows[k].Value {
		case "up":
			summary.Up++
		case "down":
			summary.Down++
		}
	}
	return summary, nil
}

// verdictResponseJSON mirrors the handler's response envelope for
// decoding in tests. Kept in this file (not the production code)
// because tests are the only consumer of the JSON shape.
type verdictResponseJSON struct {
	Summary repository.VerdictSummary `json:"summary"`
}

// validVerdictBody returns a body that passes all VH-03 checks.
func validVerdictBody() *bytes.Buffer {
	return bytes.NewBufferString(`{"value":"up","playTimeMs":42000,"outcome":"solved","clientVersion":"0.1.0"}`)
}

// TestVerdictHandler_AuthMatrix exercises the BM-01/BM-02 matrix that
// every /api/admin/* route inherits from RequireAuth + RequireAdmin
// (VH-01). Anonymous → 401, signed-in non-admin → 403, admin → 200.
func TestVerdictHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			repo := newFakeVerdictRepo()
			repo.seed(7, "standard", "abc")
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
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
		{"VH-04: skip rejected from verdict surface", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"skip","playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"negative playTimeMs", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"up","playTimeMs":-1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"playTimeMs as string", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"up","playTimeMs":"50","outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"outcome=completed rejected", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"value":"up","playTimeMs":1,"outcome":"completed","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"missing value field", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{"playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, http.StatusBadRequest},
		{"malformed JSON body", "/api/admin/puzzles/abc/verdict?size=7&mode=standard", `{not-json`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := newFakeVerdictRepo()
			repo.seed(7, "standard", "abc")
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
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
		})
	}
}

// VH-05: 404 when the puzzle row does not exist. The verdict row family
// stays empty after the failed call.
func TestVerdictHandler_PuzzleNotFound(t *testing.T) {
	// Arrange
	repo := newFakeVerdictRepo()
	// no seed — GetPuzzle returns (nil, nil)
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
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
	if got := repo.verdicts; len(got) != 0 {
		t.Errorf("expected no verdict rows on 404 path; got %d puzzles with verdicts", len(got))
	}
}

// VH-08 + VH-10: same admin submitting up → down → up ends with one row
// holding value="up" and a summary of {up: 1, down: 0}.
func TestVerdictHandler_Idempotency(t *testing.T) {
	// Arrange
	repo := newFakeVerdictRepo()
	repo.seed(7, "standard", "abc")
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
	}, "admin")

	send := func(value string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"value":%q,"playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, value)
		req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, "/api/admin/puzzles/abc/verdict?size=7&mode=standard", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// Act
	for _, v := range []string{"up", "down", "up"} {
		rec := send(v)
		if rec.Code != http.StatusOK {
			t.Fatalf("submit %q failed: %d %s", v, rec.Code, rec.Body.String())
		}
	}

	// Assert: exactly one row, last value wins.
	puzKey := fmt.Sprintf("%d#%s#%s", 7, "standard", "abc")
	rows := repo.verdicts[puzKey]
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 verdict row after up/down/up; got %d", len(rows))
	}
	for _, row := range rows {
		if row.Value != "up" {
			t.Errorf("last-write value = %q, want \"up\"", row.Value)
		}
	}

	// Final response carries the recomputed summary {up:1, down:0}.
	rec := send("up")
	var resp verdictResponseJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode final response: %v", err)
	}
	if resp.Summary.Up != 1 || resp.Summary.Down != 0 {
		t.Errorf("final summary = %+v, want Up=1 Down=0", resp.Summary)
	}
}

// VH-08: two distinct admins voting → two rows, summary reflects both.
func TestVerdictHandler_MultiRater(t *testing.T) {
	// Arrange — two routers, each mounted with a different test admin
	// identity. Same fake repo across both so the verdict rows
	// accumulate.
	repo := newFakeVerdictRepo()
	repo.seed(7, "standard", "abc")

	send := func(role, value string) *httptest.ResponseRecorder {
		// roleForState only knows two real states; we want two
		// distinct admin user IDs, so we override the verifier path
		// directly via mountAdminWithAuth's role argument and hand-
		// craft the cookie state.
		router := mountAdminWithAuth(func(r chi.Router) {
			r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
		}, role)
		body := fmt.Sprintf(`{"value":%q,"playTimeMs":1,"outcome":"solved","clientVersion":"0.1.0"}`, value)
		req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, "/api/admin/puzzles/abc/verdict?size=7&mode=standard", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// fakeAuthVerifier produces a different sub for "admin" vs "user";
	// to get two distinct admin identities we need a stronger fake. As
	// a workaround for this test, write directly via the repo (bypassing
	// the handler) for the second rater — the handler-level multi-rater
	// path is exercised in the repository-level multi-rater test
	// (puzzle_test.go TestRecomputeVerdictSummary "mixed ups and downs").
	rec := send("admin", "up")
	if rec.Code != http.StatusOK {
		t.Fatalf("first admin verdict failed: %d %s", rec.Code, rec.Body.String())
	}
	// Second rater via repo seam.
	_ = repo.PutVerdict(context.Background(), &repository.VerdictRecord{
		GridSize: 7, Mode: "standard", PuzzleID: "abc",
		RaterRole: "admin", RaterID: "user_other_admin",
		Value: "down", PlayTimeMs: 1, Outcome: "solved", ClientVersion: "0.1.0",
	})

	// Act: third admin submission triggers a recompute that should
	// reflect both rows. Use the same admin as #1 — should not double-
	// count (idempotency).
	rec = send("admin", "up")
	if rec.Code != http.StatusOK {
		t.Fatalf("re-submit failed: %d %s", rec.Code, rec.Body.String())
	}
	var resp verdictResponseJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Assert
	if resp.Summary.Up != 1 || resp.Summary.Down != 1 {
		t.Errorf("summary = %+v, want Up=1 Down=1", resp.Summary)
	}
}

// VH-06: a malicious caller adding a `raterId` field to the JSON body
// must NOT influence the persisted row's SK. The persisted raterId
// always comes from the Clerk session.
func TestVerdictHandler_RaterIDFromSessionNotBody(t *testing.T) {
	// Arrange
	repo := newFakeVerdictRepo()
	repo.seed(7, "standard", "abc")
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
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
	puzKey := fmt.Sprintf("%d#%s#%s", 7, "standard", "abc")
	rows := repo.verdicts[puzKey]
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	for sk, row := range rows {
		if row.RaterID == "imposter" {
			t.Error("RaterID was sourced from the request body — VH-06 violated")
		}
		if sk != "admin#"+testAdminSub {
			t.Errorf("SK = %q, want %q", sk, "admin#"+testAdminSub)
		}
	}
}

// VH-09: PutVerdict succeeds, RecomputeVerdictSummary fails. Handler
// still returns 200 — the verdict row is canonical, the summary is a
// cached projection that re-converges on the next vote.
func TestVerdictHandler_RecomputeFailureStill200(t *testing.T) {
	// Arrange
	repo := newFakeVerdictRepo()
	repo.seed(7, "standard", "abc")
	repo.recomputeErr = errors.New("dynamodb transient error")
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
	}, "admin")

	req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, "/api/admin/puzzles/abc/verdict?size=7&mode=standard", validVerdictBody())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (recompute failure must not fail the request); body = %s", rec.Code, rec.Body.String())
	}
	// And the verdict row IS in the repo — the user-visible action
	// succeeded.
	puzKey := fmt.Sprintf("%d#%s#%s", 7, "standard", "abc")
	if len(repo.verdicts[puzKey]) != 1 {
		t.Errorf("expected verdict row to be present despite recompute failure; got %d", len(repo.verdicts[puzKey]))
	}
}

// PutVerdict failure → 500. The user-visible action did NOT succeed; we
// surface that.
func TestVerdictHandler_PutFailure500(t *testing.T) {
	// Arrange
	repo := newFakeVerdictRepo()
	repo.seed(7, "standard", "abc")
	repo.putVerdictErr = errors.New("dynamodb write failed")
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
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

// VH-04 cross-flow: a successful verdict write does NOT change the
// puzzle's status attribute.
func TestVerdictHandler_DoesNotMutatePuzzleStatus(t *testing.T) {
	// Arrange
	repo := newFakeVerdictRepo()
	repo.seed(7, "standard", "abc")
	originalStatus := repo.puzzles[fmt.Sprintf("%d#%s#%s", 7, "standard", "abc")].Status
	router := mountAdminWithAuth(func(r chi.Router) {
		r.Put("/puzzles/{id}/verdict", handler.VerdictHandler(repo))
	}, "admin")

	req := newAdminRequest(fakeAuthAdminRole, http.MethodPut, "/api/admin/puzzles/abc/verdict?size=7&mode=standard", validVerdictBody())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Act
	router.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("verdict failed: %d %s", rec.Code, rec.Body.String())
	}
	gotStatus := repo.puzzles[fmt.Sprintf("%d#%s#%s", 7, "standard", "abc")].Status
	if gotStatus != originalStatus {
		t.Errorf("puzzle status mutated by verdict write: got %q, want %q", gotStatus, originalStatus)
	}
}
