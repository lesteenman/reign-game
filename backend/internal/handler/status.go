package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
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

		// Validate size query param. Parsed as int and range-checked so
		// it can't be used to address arbitrary partitions (e.g. CONFIG)
		// via the PK builder.
		sizeStr := r.URL.Query().Get("size")
		if sizeStr == "" {
			writeError(w, http.StatusBadRequest, "invalid_params", "size query parameter is required")
			return
		}
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
			return
		}
		if status, code, msg := validateSize(size); status != 0 {
			writeError(w, status, code, msg)
			return
		}

		// Validate mode query param.
		mode := r.URL.Query().Get("mode")
		if mode == "" {
			writeError(w, http.StatusBadRequest, "invalid_params", "mode query parameter is required")
			return
		}
		if status, code, msg := validateMode(mode); status != 0 {
			writeError(w, status, code, msg)
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

		pk := fmt.Sprintf("%d#%s", size, mode)
		if err := updater.UpdateStatus(r.Context(), pk, puzzleID, req.Status); err != nil {
			if errors.Is(err, repository.ErrPuzzleNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "puzzle not found")
				return
			}
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
