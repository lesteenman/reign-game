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
	"github.com/eriksteenman/reign-game/backend/internal/service/replenish"
)

// ConfigReader is the narrow surface ReplenishHandler needs. It matches
// replenish.AllConfigsLister so callers can pass any list-all
// implementation directly.
type ConfigReader interface {
	GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error)
}

// PoolCounter is the per-combo "ready" count interface used by the
// admin sweep. Reactive top-ups deliberately skip the count.
type PoolCounter interface {
	CountReady(ctx context.Context, size int, mode string) (int, error)
}

// MessagePublisher publishes generation requests to the SQS queue.
type MessagePublisher interface {
	PublishGenerationRequest(ctx context.Context, req *queue.GenerationRequest) error
}

// ReplenishHandler creates an HTTP handler for POST /admin/replenish.
// It parses optional size+mode filters from the query string, then
// delegates to replenish.Sweep, which performs the per-combo
// "below threshold -> publish gap" loop and returns the result.
//
// JSON response shape is byte-identical to the previous inline-loop
// implementation: `{"triggered":[{"size","mode","count"}], "skipped":
// [{"size","mode","ready"}]}` with both slices always non-nil.
func ReplenishHandler(configs ConfigReader, counter PoolCounter, publisher MessagePublisher) http.HandlerFunc {
	deps := replenish.SweepDeps{
		Configs:   configs,
		Counter:   counter,
		Publisher: publisher,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		filter, status, code, msg := parseReplenishFilter(r)
		if status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}

		result, err := replenish.Sweep(r.Context(), deps, filter)
		if err != nil {
			log.Printf("replenish handler: sweep failed: %v", err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to replenish pool")
			return
		}

		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Printf("replenish handler write failed: %v", err)
		}
	}
}

// parseReplenishFilter pulls optional size and mode filters out of the
// query string. Returns (filter, 0, "", "") on success, or
// (zero-filter, status, errCode, errMsg) on validation failure —
// matching the same triple validateMode returns elsewhere in this
// package.
func parseReplenishFilter(r *http.Request) (filter replenish.Filter, status int, errCode, errMsg string) {
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		s, err := strconv.Atoi(sizeStr)
		if err != nil {
			return replenish.Filter{}, http.StatusBadRequest, "invalid_params", "size must be an integer"
		}
		filter.Size = s
	}
	if modeStr := r.URL.Query().Get("mode"); modeStr != "" {
		if s, code, msg := validateMode(modeStr); s != 0 {
			return replenish.Filter{}, s, code, msg
		}
		filter.Mode = modeStr
	}
	return filter, 0, "", ""
}
