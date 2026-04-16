package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ConfigRepo defines the repository methods needed by config handlers.
type ConfigRepo interface {
	GetConfig(ctx context.Context, size int, mode string) (*repository.ConfigRecord, error)
	PutConfig(ctx context.Context, config *repository.ConfigRecord) error
	CreateConfig(ctx context.Context, config *repository.ConfigRecord) error
}

// configRequest is the JSON request body for config create/update.
type configRequest struct {
	Size           int     `json:"size"`
	Mode           string  `json:"mode"`
	Pipeline       string  `json:"pipeline"`
	Solver         string  `json:"solver"`
	Regions        string  `json:"regions"`
	RegionVariance float64 `json:"regionVariance"`
	Deducible      bool    `json:"deducible"`
	Concurrency    int     `json:"concurrency"`
	Threshold      int     `json:"threshold"`
	Enabled        bool    `json:"enabled"`
}

// validateConfigFields validates the config-specific fields of a configRequest.
// Returns (0, "", "") on success, or (status, errCode, errMsg) on failure.
func validateConfigFields(req *configRequest) (int, string, string) {
	if req.Pipeline != PipelineRegionFirst && req.Pipeline != PipelineIterative && req.Pipeline != PipelineConstraintAware {
		return http.StatusBadRequest, "invalid_params", "pipeline must be 'region-first', 'iterative', or 'constraint-aware'"
	}

	if req.Solver != SolverBacktrack && req.Solver != SolverPropagation {
		return http.StatusBadRequest, "invalid_params", "solver must be 'backtrack' or 'propagation'"
	}

	if req.Regions != RegionsBFS && req.Regions != RegionsWFC {
		return http.StatusBadRequest, "invalid_params", "regions must be 'bfs' or 'wfc'"
	}

	if math.IsNaN(req.RegionVariance) || math.IsInf(req.RegionVariance, 0) {
		return http.StatusBadRequest, "invalid_params", "regionVariance must be between 0.0 and 1.0"
	}
	if req.RegionVariance < 0.0 || req.RegionVariance > 1.0 {
		return http.StatusBadRequest, "invalid_params", "regionVariance must be between 0.0 and 1.0"
	}

	if req.Concurrency < 1 || req.Concurrency > 8 {
		return http.StatusBadRequest, "invalid_params", "concurrency must be between 1 and 8"
	}

	if req.Threshold < 1 {
		return http.StatusBadRequest, "invalid_params", "threshold must be at least 1"
	}

	return 0, "", ""
}

// buildConfigResponseMap builds a JSON-serializable map from a ConfigRecord.
func buildConfigResponseMap(rec *repository.ConfigRecord) map[string]any {
	return map[string]any{
		"size":           rec.Size,
		"mode":           rec.Mode,
		"pipeline":       rec.Pipeline,
		"solver":         rec.Solver,
		"regions":        rec.Regions,
		"regionVariance": rec.RegionVariance,
		"deducible":      rec.Deducible,
		"concurrency":    rec.Concurrency,
		"threshold":      rec.Threshold,
		"enabled":        rec.Enabled,
	}
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
			Size:           size,
			Mode:           mode,
			Pipeline:       req.Pipeline,
			Solver:         req.Solver,
			Regions:        req.Regions,
			RegionVariance: req.RegionVariance,
			Deducible:      req.Deducible,
			Concurrency:    req.Concurrency,
			Threshold:      req.Threshold,
			Enabled:        req.Enabled,
		}

		if err := repo.PutConfig(r.Context(), config); err != nil {
			log.Printf("PutConfig error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to update config")
			return
		}

		w.WriteHeader(http.StatusOK)
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
			Size:           req.Size,
			Mode:           req.Mode,
			Pipeline:       req.Pipeline,
			Solver:         req.Solver,
			Regions:        req.Regions,
			RegionVariance: req.RegionVariance,
			Deducible:      req.Deducible,
			Concurrency:    req.Concurrency,
			Threshold:      req.Threshold,
			Enabled:        req.Enabled,
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
