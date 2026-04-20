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

// configRequest is the JSON request body for config create/update.
type configRequest struct {
	Size        int    `json:"size"`
	Mode        string `json:"mode"`
	Threshold   int    `json:"threshold"`
	Enabled     bool   `json:"enabled"`
	MaxAttempts int    `json:"maxAttempts,omitempty"`
}

// validateConfigFields validates the config-specific fields of a configRequest.
// Returns (0, "", "") on success, or (status, errCode, errMsg) on failure.
func validateConfigFields(req *configRequest) (status int, errCode, errMsg string) {
	if req.Threshold < 1 || req.Threshold > maxConfigThreshold {
		return http.StatusBadRequest, "invalid_params", fmt.Sprintf("threshold must be between 1 and %d", maxConfigThreshold)
	}
	if req.MaxAttempts < 0 {
		return http.StatusBadRequest, "invalid_params", "maxAttempts must be >= 0"
	}
	return 0, "", ""
}

// buildConfigResponseMap builds a JSON-serializable map from a ConfigRecord.
func buildConfigResponseMap(rec *repository.ConfigRecord) map[string]any {
	resp := map[string]any{
		"size":      rec.Size,
		"mode":      rec.Mode,
		"threshold": rec.Threshold,
		"enabled":   rec.Enabled,
	}
	if rec.MaxAttempts > 0 {
		resp["maxAttempts"] = rec.MaxAttempts
	}
	return resp
}

// UpdateConfigHandler handles PUT /admin/config/{size}/{mode}.
func UpdateConfigHandler(repo ConfigRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse size from URL.
		sizeStr := chi.URLParam(r, "size")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
			return
		}
		if size < 3 || size > 15 {
			writeError(w, http.StatusBadRequest, "invalid_params", "size must be between 3 and 15")
			return
		}

		// Parse mode from URL.
		mode := chi.URLParam(r, "mode")
		if mode != ModeStandard && mode != ModeDouble {
			writeError(w, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'")
			return
		}

		// Decode request body.
		var req configRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}

		// Validate config fields.
		if status, code, msg := validateConfigFields(&req); status != 0 {
			writeError(w, status, code, msg)
			return
		}

		// Check config exists.
		existing, err := repo.GetConfig(r.Context(), size, mode)
		if err != nil {
			log.Printf("GetConfig error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to check existing config")
			return
		}
		if existing == nil {
			writeError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("config not found for %dx%d %s", size, size, mode))
			return
		}

		// Build and save config.
		config := &repository.ConfigRecord{
			Size:        size,
			Mode:        mode,
			Threshold:   req.Threshold,
			Enabled:     req.Enabled,
			MaxAttempts: req.MaxAttempts,
		}

		if err := repo.PutConfig(r.Context(), config); err != nil {
			log.Printf("PutConfig error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to update config")
			return
		}

		if err := json.NewEncoder(w).Encode(buildConfigResponseMap(config)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}

// CreateConfigHandler handles POST /admin/config.
func CreateConfigHandler(repo ConfigRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Decode request body.
		var req configRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}

		// Validate size and mode from body.
		if req.Size < 3 || req.Size > 15 {
			writeError(w, http.StatusBadRequest, "invalid_params", "size must be between 3 and 15")
			return
		}
		if req.Mode != ModeStandard && req.Mode != ModeDouble {
			writeError(w, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'")
			return
		}

		// Validate config fields.
		if status, code, msg := validateConfigFields(&req); status != 0 {
			writeError(w, status, code, msg)
			return
		}

		// Build and create config.
		config := &repository.ConfigRecord{
			Size:        req.Size,
			Mode:        req.Mode,
			Threshold:   req.Threshold,
			Enabled:     req.Enabled,
			MaxAttempts: req.MaxAttempts,
		}

		if err := repo.CreateConfig(r.Context(), config); err != nil {
			var alreadyExists *repository.ConfigAlreadyExistsError
			if errors.As(err, &alreadyExists) {
				writeError(w, http.StatusConflict, "conflict",
					fmt.Sprintf("config already exists for %dx%d %s", req.Size, req.Size, req.Mode))
				return
			}
			log.Printf("CreateConfig error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create config")
			return
		}

		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(buildConfigResponseMap(config)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}
