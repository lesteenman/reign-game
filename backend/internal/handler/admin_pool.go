package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ConfigAndCountRepo defines the repository methods needed by AdminPoolHandler.
type ConfigAndCountRepo interface {
	GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error)
	CountReady(ctx context.Context, size int, mode string) (int, error)
}

type configResponse struct {
	Threshold   int  `json:"threshold"`
	Enabled     bool `json:"enabled"`
	MaxAttempts int  `json:"maxAttempts,omitempty"`
}

type comboStatus struct {
	Size       int            `json:"size"`
	Mode       string         `json:"mode"`
	Config     configResponse `json:"config"`
	ReadyCount int            `json:"readyCount"`
}

type adminPoolResponse struct {
	Combos []comboStatus `json:"combos"`
}

// AdminPoolHandler creates an HTTP handler for GET /admin/pool.
// It returns all config items with their ready puzzle counts.
func AdminPoolHandler(repo ConfigAndCountRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		configs, err := repo.GetAllConfigs(r.Context())
		if err != nil {
			log.Printf("admin pool: failed to get configs: %v", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve configs.")
			return
		}

		combos := make([]comboStatus, 0, len(configs))
		for _, cfg := range configs {
			readyCount := 0
			if cfg.Enabled {
				readyCount, err = repo.CountReady(r.Context(), cfg.Size, cfg.Mode)
				if err != nil {
					log.Printf("admin pool: failed to count ready for %d#%s: %v", cfg.Size, cfg.Mode, err)
					writeError(w, http.StatusInternalServerError, "internal_error", "Failed to count ready puzzles.")
					return
				}
			}

			combos = append(combos, comboStatus{
				Size: cfg.Size,
				Mode: cfg.Mode,
				Config: configResponse{
					Threshold:   cfg.Threshold,
					Enabled:     cfg.Enabled,
					MaxAttempts: cfg.MaxAttempts,
				},
				ReadyCount: readyCount,
			})
		}

		resp := adminPoolResponse{Combos: combos}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("admin pool: failed to write response: %v", err)
		}
	}
}
