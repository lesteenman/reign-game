package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	"github.com/eriksteenman/reign-game/backend/internal/queue"
	"github.com/eriksteenman/reign-game/backend/internal/replenish"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ConfigReader is kept here as a thin alias so tests/callers that built
// their own fakes against the handler-level interface keep compiling
// after the loop moved to internal/replenish. Same shape — Sweep needs
// only GetAllConfigs, not GetConfig.
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

// sweepConfigsAdapter wraps the narrow handler-level ConfigReader
// (GetAllConfigs only) into the replenish.ConfigReader interface,
// stubbing GetConfig out — Sweep never calls it. Lets the handler
// keep accepting any caller's ConfigReader implementation while
// internally delegating to the shared package.
type sweepConfigsAdapter struct {
	inner ConfigReader
}

func (a sweepConfigsAdapter) GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error) {
	return a.inner.GetAllConfigs(ctx)
}

func (sweepConfigsAdapter) GetConfig(_ context.Context, _ int, _ string) (*repository.ConfigRecord, error) {
	// Sweep never calls GetConfig; the adapter exists only to satisfy
	// the replenish.ConfigReader interface.
	return nil, nil
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
		Configs:   sweepConfigsAdapter{inner: configs},
		Counter:   counter,
		Publisher: publisher,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		filter, perr := parseReplenishFilter(r)
		if perr != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", perr.Error())
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
// query string. Empty values pass through as zero-value filters
// (matching the prior handler semantics — no filter == every combo).
func parseReplenishFilter(r *http.Request) (replenish.Filter, error) {
	var filter replenish.Filter
	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		s, err := strconv.Atoi(sizeStr)
		if err != nil {
			return filter, errors.New("size must be an integer")
		}
		filter.Size = s
	}
	if modeStr := r.URL.Query().Get("mode"); modeStr != "" {
		if modeStr != ModeStandard && modeStr != ModeDouble {
			return filter, errors.New("mode must be 'standard' or 'double'")
		}
		filter.Mode = modeStr
	}
	return filter, nil
}
