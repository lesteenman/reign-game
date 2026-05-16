package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	configsvc "github.com/eriksteenman/reign-game/backend/internal/service/config"
)

// ConfigModesService is the application surface ConfigModesHandler
// needs — narrower than ConfigService so a public endpoint can't
// accidentally call a write method via the same interface.
type ConfigModesService interface {
	ListEnabledModes(ctx context.Context) ([]configsvc.ConfigView, error)
}

// ModeEntry is one {size, mode} pair in the public modes listing.
type ModeEntry struct {
	Size int    `json:"size"`
	Mode string `json:"mode"`
}

// ConfigModesResponse is the JSON shape of GET /api/config/modes.
// Modes is always a non-nil slice so the frontend distinguishes
// "fetch failed" from "no enabled combos" by the array's presence.
type ConfigModesResponse struct {
	Modes []ModeEntry `json:"modes"`
}

// ConfigModesHandler serves GET /api/config/modes. Lists every
// (size, mode) combo with enabled=true — the subset of CONFIG data a
// free-user landing page needs to render its mode buttons. Thresholds,
// ready counts, and other admin data are NOT exposed here. This
// endpoint is public by design; it carries no information that has
// not already been publicly observable via /api/puzzles/next lookups
// by size+mode.
func ConfigModesHandler(svc ConfigModesService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Short cache lets a CDN absorb repeat hits from the landing
		// page without making the payload stale beyond a minute.
		// CONFIG rows only change on admin edits, so a 60 s TTL is
		// well within freshness expectations.
		w.Header().Set("Cache-Control", "public, max-age=60")

		configs, err := svc.ListEnabledModes(r.Context())
		if err != nil {
			log.Printf("config modes: ListEnabledModes failed: %v", err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve configured modes.")
			return
		}

		// Always non-nil so encoding yields `"modes":[]` not `"modes":null`.
		modes := make([]ModeEntry, 0, len(configs))
		for _, cfg := range configs {
			modes = append(modes, ModeEntry{Size: cfg.Size, Mode: cfg.Mode})
		}

		resp := ConfigModesResponse{Modes: modes}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("config modes: failed to write response: %v", err)
		}
	}
}
