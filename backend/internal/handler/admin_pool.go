package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ConfigAndCountRepo defines the repository methods needed by AdminPoolHandler.
type ConfigAndCountRepo interface {
	GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error)
	CountReady(ctx context.Context, size int, mode string) (int, error)
}

// ComboStatus is the per-combo entry in the pool status response.
// Config nests a ConfigBody (shared with the flat ConfigView DTO) so
// there is one canonical shape for the threshold/enabled/maxAttempts
// fields across every CONFIG-bearing API surface.
type ComboStatus struct {
	Size       int        `json:"size"`
	Mode       string     `json:"mode"`
	Config     ConfigBody `json:"config"`
	ReadyCount int        `json:"readyCount"`
}

type adminPoolResponse struct {
	Combos []ComboStatus `json:"combos"`
}

// AdminPoolHandler creates an HTTP handler for GET /admin/pool.
// It returns all config items with their ready puzzle counts.
//
// Logs per-DDB-call timing on every request: GetAllConfigs latency
// plus a per-combo CountReady latency map. Useful for diagnosing
// "the pool page feels slow" — the breakdown surfaces whether the
// bottleneck is the configs query or one of the count-ready scans.
func AdminPoolHandler(repo ConfigAndCountRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		handlerStart := time.Now()

		configsStart := time.Now()
		configs, err := repo.GetAllConfigs(r.Context())
		configsMs := time.Since(configsStart).Milliseconds()
		if err != nil {
			log.Printf("admin pool: failed to get configs: %v configs_ms=%d", err, configsMs)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve configs.")
			return
		}

		combos := make([]ComboStatus, 0, len(configs))
		// Each entry: "<size>#<mode>=<ms>" — terse so a single log line
		// shows the per-combo breakdown without bloating the format.
		countTimings := make([]string, 0, len(configs))
		for _, cfg := range configs {
			readyCount := 0
			countMs := int64(-1) // sentinel for "skipped (config disabled)"
			if cfg.Enabled {
				countStart := time.Now()
				readyCount, err = repo.CountReady(r.Context(), cfg.Size, cfg.Mode)
				countMs = time.Since(countStart).Milliseconds()
				if err != nil {
					log.Printf("admin pool: failed to count ready for %d#%s: %v count_%d#%s_ms=%d", cfg.Size, cfg.Mode, err, cfg.Size, cfg.Mode, countMs)
					httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to count ready puzzles.")
					return
				}
			}
			countTimings = append(countTimings, fmt.Sprintf("%d#%s=%dms", cfg.Size, cfg.Mode, countMs))

			combos = append(combos, ComboStatus{
				Size:       cfg.Size,
				Mode:       cfg.Mode,
				Config:     configBodyFrom(&cfg),
				ReadyCount: readyCount,
			})
		}

		resp := adminPoolResponse{Combos: combos}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("admin pool: failed to write response: %v", err)
		}

		log.Printf(
			"admin pool: total_ms=%d configs_ms=%d combos=%d count_breakdown=[%s]",
			time.Since(handlerStart).Milliseconds(),
			configsMs,
			len(configs),
			strings.Join(countTimings, " "),
		)
	}
}
