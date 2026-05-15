package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	statussvc "github.com/eriksteenman/reign-game/backend/internal/service/status"
)

// StatusService is the application surface the status handler depends
// on. SetStatus returns statussvc.ErrPuzzleNotFound when the row does
// not exist; any other error is mapped to a 500.
type StatusService interface {
	SetStatus(ctx context.Context, size int, mode, puzzleID, status string) error
}

// statusRequest is the expected JSON request body for status updates.
type statusRequest struct {
	Status string `json:"status"`
}

// StatusHandler creates an HTTP handler for PUT /puzzles/{id}/status.
// It validates parameters and delegates the actual write to the
// status service.
func StatusHandler(svc StatusService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		puzzleID := chi.URLParam(r, "id")
		if puzzleID == "" {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "puzzle ID is required")
			return
		}

		// Size is parsed and range-checked so it can't address arbitrary
		// partitions (e.g. CONFIG) via the PK builder downstream.
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

		var req statusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}
		if req.Status != "solved" && req.Status != "skipped" {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "status must be 'solved' or 'skipped'")
			return
		}

		if err := svc.SetStatus(r.Context(), size, mode, puzzleID, req.Status); err != nil {
			if errors.Is(err, statussvc.ErrPuzzleNotFound) {
				httperr.WriteError(w, http.StatusNotFound, "not_found", "puzzle not found")
				return
			}
			log.Printf("status handler: SetStatus failed for puzzle %s -> %s: %v", puzzleID, req.Status, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to update puzzle status")
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": req.Status}); err != nil {
			log.Printf("status handler write failed: %v", err)
		}
	}
}
