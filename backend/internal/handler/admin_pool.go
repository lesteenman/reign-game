package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	poolsvc "github.com/eriksteenman/reign-game/backend/internal/service/pool"
)

// PoolService is the application surface the admin pool handler depends
// on. Orchestration (fetching configs, counting ready puzzles, emitting
// per-step DDB timing) lives in internal/service/pool.
type PoolService interface {
	LoadPool(ctx context.Context) ([]poolsvc.ComboEntry, error)
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
// It delegates orchestration to the pool service and maps the result
// to a JSON response.
func AdminPoolHandler(svc PoolService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		entries, err := svc.LoadPool(r.Context())
		if err != nil {
			log.Printf("admin pool handler: LoadPool failed: %v", err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve pool status.")
			return
		}

		combos := make([]ComboStatus, 0, len(entries))
		for _, e := range entries {
			combos = append(combos, ComboStatus{
				Size: e.Config.Size,
				Mode: e.Config.Mode,
				Config: ConfigBody{
					Threshold:   e.Config.Threshold,
					Enabled:     e.Config.Enabled,
					MaxAttempts: e.Config.MaxAttempts,
				},
				ReadyCount: e.ReadyCount,
			})
		}

		resp := adminPoolResponse{Combos: combos}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("admin pool handler: failed to write response: %v", err)
		}
	}
}
