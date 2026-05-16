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

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
	configsvc "github.com/eriksteenman/reign-game/backend/internal/service/config"
)

// maxConfigThreshold caps how many ready puzzles a single CONFIG item can
// demand. Replenish enqueues one SQS message per unit of threshold-minus-count,
// so an unbounded threshold combined with an unauthenticated admin surface
// (see KI-009) would let any caller amplify a single HTTP request into
// arbitrary SQS load. 50 is generous for real pool sizes.
const maxConfigThreshold = 50

// ConfigService is the application surface the admin-config handlers
// depend on. Update returns configsvc.ErrNotFound when the target row
// is absent; Create returns configsvc.ErrAlreadyExists on duplicates.
type ConfigService interface {
	Update(ctx context.Context, record *repository.ConfigRecord) error
	Create(ctx context.Context, record *repository.ConfigRecord) error
}

// UpdateConfigHandler handles PUT /admin/config/{size}/{mode}.
func UpdateConfigHandler(svc ConfigService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		sizeStr := chi.URLParam(r, "size")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
			return
		}
		if status, code, msg := validateSize(size); status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}

		mode := chi.URLParam(r, "mode")
		if status, code, msg := validateMode(mode); status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}

		var req ConfigUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}
		if status, code, msg := validateConfigBody(&req.ConfigBody); status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}

		record := req.toRecord(size, mode)
		if err := svc.Update(r.Context(), record); err != nil {
			if errors.Is(err, configsvc.ErrNotFound) {
				httperr.WriteError(w, http.StatusNotFound, "not_found",
					fmt.Sprintf("config not found for %dx%d %s", size, size, mode))
				return
			}
			log.Printf("admin config: Update failed for %d#%s: %v", size, mode, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update config")
			return
		}

		if err := json.NewEncoder(w).Encode(configViewFrom(record)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}

// CreateConfigHandler handles POST /admin/config.
func CreateConfigHandler(svc ConfigService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req ConfigCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}
		if status, code, msg := validateSize(req.Size); status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}
		if status, code, msg := validateMode(req.Mode); status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}
		if status, code, msg := validateConfigBody(&req.ConfigBody); status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}

		record := req.toRecord()
		if err := svc.Create(r.Context(), record); err != nil {
			if errors.Is(err, configsvc.ErrAlreadyExists) {
				httperr.WriteError(w, http.StatusConflict, "conflict",
					fmt.Sprintf("config already exists for %dx%d %s", req.Size, req.Size, req.Mode))
				return
			}
			log.Printf("admin config: Create failed for %d#%s: %v", req.Size, req.Mode, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create config")
			return
		}

		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(configViewFrom(record)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}
