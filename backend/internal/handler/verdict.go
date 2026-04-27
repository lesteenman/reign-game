package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/auth"
	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// VerdictRepo is the minimal repository surface VerdictHandler depends on.
// Defined as an interface so tests substitute a fake without spinning up
// a real DynamoDB client.
type VerdictRepo interface {
	GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error)
	PutVerdict(ctx context.Context, v *repository.VerdictRecord) error
	RecomputeVerdictSummary(ctx context.Context, size int, mode, puzzleID string) (repository.VerdictSummary, error)
}

// verdictRequest is the decoded JSON body for PUT verdict (VH-03).
// raterId is intentionally NOT a field — VH-06 requires the rater ID to
// come from the authenticated session, never from caller-controlled input.
type verdictRequest struct {
	Value         string `json:"value"`
	PlayTimeMs    int64  `json:"playTimeMs"`
	Outcome       string `json:"outcome"`
	ClientVersion string `json:"clientVersion"`
}

// verdictResponse is the success envelope returned to the frontend (VH-08).
// Carries the recomputed summary so the UI can render the new totals
// without a follow-up GET.
type verdictResponse struct {
	Summary repository.VerdictSummary `json:"summary"`
}

// VerdictHandler creates the HTTP handler for
// PUT /api/admin/puzzles/{id}/verdict.
//
// Mounted inside the existing /api/admin route group so RequireAuth +
// RequireAdmin run once at the group level (VH-01). The handler trusts
// the middleware to have already proven the caller is signed in and
// holds the admin role; it does not re-check.
func VerdictHandler(repo VerdictRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// VH-02: required path + query parameters.
		puzzleID := chi.URLParam(r, "id")
		if puzzleID == "" {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "puzzle ID is required")
			return
		}

		sizeStr := r.URL.Query().Get("size")
		if sizeStr == "" {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "size query parameter is required")
			return
		}
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
			return
		}
		if status, code, msg := validateSize(size); status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}

		mode := r.URL.Query().Get("mode")
		if mode == "" {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "mode query parameter is required")
			return
		}
		if status, code, msg := validateMode(mode); status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}

		// VH-03: decode + validate the body. DisallowUnknownFields is NOT
		// used — VH-03 says unknown fields are ignored (so a body that
		// includes a stray `raterId` decodes cleanly without it landing
		// on the verdict row, satisfying VH-06).
		var req verdictRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}

		// VH-03 + VH-04: case-sensitive value, skip rejected.
		if req.Value != "up" && req.Value != "down" {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "value must be 'up' or 'down'")
			return
		}
		if req.Outcome != "solved" && req.Outcome != "skipped" {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "outcome must be 'solved' or 'skipped'")
			return
		}
		if req.PlayTimeMs < 0 {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "playTimeMs must be non-negative")
			return
		}

		// VH-06: rater ID comes from the Clerk session via the auth
		// middleware. Never the request body.
		user := auth.UserFromContext(r.Context())
		if user == nil {
			// Defense-in-depth: RequireAuth should have returned 401 long
			// before this. If we reach here without a user, something is
			// wrong with the middleware chain — surface a 500 rather than
			// silently writing a row with raterId="".
			log.Printf("verdict: UserFromContext returned nil after RequireAuth puzzle=%s", puzzleID)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "authentication context missing")
			return
		}
		raterID := user.ID

		// VH-05: 404 if the puzzle does not exist before writing a
		// verdict row.
		puzzle, err := repo.GetPuzzle(r.Context(), size, mode, puzzleID)
		if err != nil {
			log.Printf("verdict: get puzzle failed puzzle=%s err=%v", puzzleID, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to look up puzzle.")
			return
		}
		if puzzle == nil {
			httperr.WriteError(w, http.StatusNotFound, "not_found", "puzzle not found")
			return
		}

		// VH-07: raterRole is hard-coded "admin" this phase. When a
		// public-rater role lands, this becomes one line:
		//     raterRole := getClerkRole(user)
		const raterRole = "admin"

		v := &repository.VerdictRecord{
			GridSize:      size,
			Mode:          mode,
			PuzzleID:      puzzleID,
			RaterRole:     raterRole,
			RaterID:       raterID,
			Value:         req.Value,
			PlayTimeMs:    req.PlayTimeMs,
			Outcome:       req.Outcome,
			ClientVersion: req.ClientVersion,
		}

		// VH-10: PutVerdict is unconditional; SK collision overwrites.
		if err := repo.PutVerdict(r.Context(), v); err != nil {
			log.Printf("verdict: put failed puzzle=%s rater=%s err=%v", puzzleID, raterID, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to record verdict.")
			return
		}

		// VH-08 + VH-09: recompute the cached summary projection.
		// Recompute failure does NOT fail the request — the row family
		// is canonical, the summary is a cached projection that re-
		// converges on the next vote.
		summary, recomputeErr := repo.RecomputeVerdictSummary(r.Context(), size, mode, puzzleID)
		if recomputeErr != nil {
			// ConditionalCheckFailed (puzzle deleted between PutVerdict
			// and RecomputeVerdictSummary) is a benign race — the row
			// family still has our verdict; the projection will be
			// reconciled when Phase 9's analysis agent runs.
			if errors.Is(recomputeErr, repository.ErrPuzzleNotFound) {
				log.Printf("WARN: verdict: summary recompute hit ErrPuzzleNotFound (race) puzzle=%s rater=%s", puzzleID, raterID)
			} else {
				log.Printf("WARN: verdict: summary recompute failed puzzle=%s rater=%s err=%v", puzzleID, raterID, recomputeErr)
			}
		}

		// VH-11: log success with rater + value, never the session token.
		log.Printf("verdict: write puzzle=%s rater=%s value=%s outcome=%s", puzzleID, raterID, req.Value, req.Outcome)

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(verdictResponse{Summary: summary}); err != nil {
			log.Printf("verdict: write response failed puzzle=%s err=%v", puzzleID, err)
		}
	}
}
