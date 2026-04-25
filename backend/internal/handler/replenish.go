package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	"github.com/eriksteenman/reign-game/backend/internal/queue"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ConfigReader reads config records for dynamic combo discovery.
type ConfigReader interface {
	GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error)
}

// PoolCounter counts ready puzzles for a given size and mode.
type PoolCounter interface {
	CountReady(ctx context.Context, size int, mode string) (int, error)
}

// MessagePublisher publishes generation requests to an SQS queue.
type MessagePublisher interface {
	PublishGenerationRequest(ctx context.Context, req *queue.GenerationRequest) error
}

// triggeredEntry represents a size+mode combo that had generation requests triggered.
type triggeredEntry struct {
	Size  int    `json:"size"`
	Mode  string `json:"mode"`
	Count int    `json:"count"`
}

// skippedEntry represents a size+mode combo that had enough ready puzzles.
type skippedEntry struct {
	Size  int    `json:"size"`
	Mode  string `json:"mode"`
	Ready int    `json:"ready"`
}

// replenishResponse is the JSON response for the replenish endpoint.
type replenishResponse struct {
	Triggered []triggeredEntry `json:"triggered"`
	Skipped   []skippedEntry   `json:"skipped"`
}

// ReplenishHandler creates an HTTP handler for POST /admin/replenish.
// It checks pool levels for all (or filtered) size+mode combos and publishes
// SQS generation requests when pools are below threshold.
func ReplenishHandler(configs ConfigReader, counter PoolCounter, publisher MessagePublisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse optional filter parameters.
		filterSize := 0
		filterMode := ""

		if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
			s, err := strconv.Atoi(sizeStr)
			if err != nil {
				httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
				return
			}
			filterSize = s
		}
		if modeStr := r.URL.Query().Get("mode"); modeStr != "" {
			if modeStr != ModeStandard && modeStr != ModeDouble {
				httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'")
				return
			}
			filterMode = modeStr
		}

		allConfigs, err := configs.GetAllConfigs(r.Context())
		if err != nil {
			log.Printf("error fetching configs: %v", err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve configs")
			return
		}

		resp := replenishResponse{
			Triggered: []triggeredEntry{},
			Skipped:   []skippedEntry{},
		}

		for _, config := range allConfigs {
			if !config.Enabled {
				continue
			}

			// Apply filters if specified.
			if filterSize != 0 && config.Size != filterSize {
				continue
			}
			if filterMode != "" && config.Mode != filterMode {
				continue
			}

			count, err := counter.CountReady(r.Context(), config.Size, config.Mode)
			if err != nil {
				log.Printf("error counting ready puzzles for %dx%d %s: %v", config.Size, config.Size, config.Mode, err)
				httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to check pool levels")
				return
			}

			if count >= config.Threshold {
				resp.Skipped = append(resp.Skipped, skippedEntry{
					Size:  config.Size,
					Mode:  config.Mode,
					Ready: count,
				})
				continue
			}

			needed := config.Threshold - count
			for i := 0; i < needed; i++ {
				req := &queue.GenerationRequest{
					Size:        config.Size,
					Mode:        config.Mode,
					MaxAttempts: config.MaxAttempts,
				}
				if err := publisher.PublishGenerationRequest(r.Context(), req); err != nil {
					log.Printf("error publishing generation request for %dx%d %s: %v", config.Size, config.Size, config.Mode, err)
					httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to publish generation request")
					return
				}
			}

			resp.Triggered = append(resp.Triggered, triggeredEntry{
				Size:  config.Size,
				Mode:  config.Mode,
				Count: needed,
			})
		}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("replenish handler write failed: %v", err)
		}
	}
}
