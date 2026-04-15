package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// StatusUpdater updates puzzle status in the repository.
type StatusUpdater interface {
	UpdateStatus(ctx context.Context, pk, sk, status string) error
}

// statusRequest is the expected JSON request body for status updates.
type statusRequest struct {
	Status string `json:"status"`
}

// StatusHandler creates an HTTP handler for PUT /puzzles/{id}/status.
// It validates the status value and updates the puzzle status in DynamoDB.
// Requires size and mode query params to construct the partition key.
func StatusHandler(updater StatusUpdater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Extract puzzle ID from URL path.
		puzzleID := chi.URLParam(r, "id")
		if puzzleID == "" {
			writeError(w, http.StatusBadRequest, "invalid_params", "puzzle ID is required")
			return
		}

		// Validate size query param.
		sizeStr := r.URL.Query().Get("size")
		if sizeStr == "" {
			writeError(w, http.StatusBadRequest, "invalid_params", "size query parameter is required")
			return
		}

		// Validate mode query param.
		mode := r.URL.Query().Get("mode")
		if mode == "" {
			writeError(w, http.StatusBadRequest, "invalid_params", "mode query parameter is required")
			return
		}
		if mode != ModeStandard && mode != ModeDouble {
			writeError(w, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'")
			return
		}

		// Parse request body.
		var req statusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}

		// Validate status value.
		if req.Status != "solved" && req.Status != "skipped" {
			writeError(w, http.StatusBadRequest, "invalid_params", "status must be 'solved' or 'skipped'")
			return
		}

		// Construct partition key and update.
		pk := fmt.Sprintf("%s#%s", sizeStr, mode)
		if err := updater.UpdateStatus(r.Context(), pk, puzzleID, req.Status); err != nil {
			log.Printf("error updating puzzle %s status to %s: %v", puzzleID, req.Status, err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update puzzle status")
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": req.Status}); err != nil {
			log.Printf("status handler write failed: %v", err)
		}
	}
}
