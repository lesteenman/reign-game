package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/eriksteenman/reign-game/backend/internal/queue"
)

// PoolThreshold is the minimum number of ready puzzles per size+mode combo.
// If a combo falls below this, replenish publishes generation requests.
const PoolThreshold = 3

// sizeModeCombos defines all supported size+mode combinations for the pool.
var sizeModeCombos = []struct {
	Size int
	Mode string
}{
	{5, ModeStandard},
	{7, ModeStandard},
	{9, ModeStandard},
	{7, ModeDouble},
	{9, ModeDouble},
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
func ReplenishHandler(counter PoolCounter, publisher MessagePublisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse optional filter parameters.
		filterSize := 0
		filterMode := ""

		if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
			s, err := strconv.Atoi(sizeStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
				return
			}
			filterSize = s
		}
		if modeStr := r.URL.Query().Get("mode"); modeStr != "" {
			if modeStr != ModeStandard && modeStr != ModeDouble {
				writeError(w, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'")
				return
			}
			filterMode = modeStr
		}

		resp := replenishResponse{
			Triggered: []triggeredEntry{},
			Skipped:   []skippedEntry{},
		}

		for _, combo := range sizeModeCombos {
			// Apply filters if specified.
			if filterSize != 0 && combo.Size != filterSize {
				continue
			}
			if filterMode != "" && combo.Mode != filterMode {
				continue
			}

			count, err := counter.CountReady(r.Context(), combo.Size, combo.Mode)
			if err != nil {
				log.Printf("error counting ready puzzles for %dx%d %s: %v", combo.Size, combo.Size, combo.Mode, err)
				writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check pool levels")
				return
			}

			if count >= PoolThreshold {
				resp.Skipped = append(resp.Skipped, skippedEntry{
					Size:  combo.Size,
					Mode:  combo.Mode,
					Ready: count,
				})
				continue
			}

			needed := PoolThreshold - count
			for i := 0; i < needed; i++ {
				req := &queue.GenerationRequest{
					Size:           combo.Size,
					Mode:           combo.Mode,
					Pipeline:       PipelineIterative,
					Solver:         SolverPropagation,
					Regions:        RegionsBFS,
					RegionVariance: 0.0,
					Deducible:      true,
					Concurrency:    1,
				}
				if err := publisher.PublishGenerationRequest(r.Context(), req); err != nil {
					log.Printf("error publishing generation request for %dx%d %s: %v", combo.Size, combo.Size, combo.Mode, err)
					writeError(w, http.StatusInternalServerError, "internal_error", "Failed to publish generation request")
					return
				}
			}

			resp.Triggered = append(resp.Triggered, triggeredEntry{
				Size:  combo.Size,
				Mode:  combo.Mode,
				Count: needed,
			})
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("replenish handler write failed: %v", err)
		}
	}
}
