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
)

// maxConfigThreshold caps how many ready puzzles a single CONFIG item can
// demand. Replenish enqueues one SQS message per unit of threshold-minus-count,
// so an unbounded threshold combined with an unauthenticated admin surface
// (see KI-009) would let any caller amplify a single HTTP request into
// arbitrary SQS load. 50 is generous for real pool sizes.
const maxConfigThreshold = 50

// ConfigRepo defines the repository methods needed by config handlers.
type ConfigRepo interface {
	GetConfig(ctx context.Context, size int, mode string) (*repository.ConfigRecord, error)
	PutConfig(ctx context.Context, config *repository.ConfigRecord) error
	CreateConfig(ctx context.Context, config *repository.ConfigRecord) error
}

// UpdateConfigHandler handles PUT /admin/config/{size}/{mode}.
func UpdateConfigHandler(repo ConfigRepo) http.HandlerFunc {
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

		existing, err := repo.GetConfig(r.Context(), size, mode)
		if err != nil {
			log.Printf("GetConfig error: %v", err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to check existing config")
			return
		}
		if existing == nil {
			httperr.WriteError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("config not found for %dx%d %s", size, size, mode))
			return
		}

		record := req.toRecord(size, mode)
		if err := repo.PutConfig(r.Context(), record); err != nil {
			log.Printf("PutConfig error: %v", err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update config")
			return
		}

		if err := json.NewEncoder(w).Encode(configViewFrom(record)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}

// CreateConfigHandler handles POST /admin/config.
func CreateConfigHandler(repo ConfigRepo) http.HandlerFunc {
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
		if err := repo.CreateConfig(r.Context(), record); err != nil {
			var alreadyExists *repository.ConfigAlreadyExistsError
			if errors.As(err, &alreadyExists) {
				httperr.WriteError(w, http.StatusConflict, "conflict",
					fmt.Sprintf("config already exists for %dx%d %s", req.Size, req.Size, req.Mode))
				return
			}
			log.Printf("CreateConfig error: %v", err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create config")
			return
		}

		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(configViewFrom(record)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}
