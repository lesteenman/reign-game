package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	packsvc "github.com/eriksteenman/reign-game/backend/internal/service/pack"
)

// PublicPackService is the application surface the public (unauthenticated)
// pack read handlers depend on. It is the read-only subset of the pack
// service — no create/update/delete reachable from these routes.
type PublicPackService interface {
	ServePack(ctx context.Context, slug string) (*packsvc.PackServeView, error)
	ListPublished(ctx context.Context) ([]packsvc.PackSummary, error)
}

// ServePackHandler handles GET /api/packs/{slug}. It returns a published
// pack's manifest plus every puzzle's play data (no solution). A draft
// or unknown slug returns 404 — indistinguishable, so drafts are not
// discoverable. No authentication is required.
func ServePackHandler(svc PublicPackService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		slug := chi.URLParam(r, "slug")
		view, err := svc.ServePack(r.Context(), slug)
		if err != nil {
			if errors.Is(err, packsvc.ErrNotFound) {
				httperr.WriteError(w, http.StatusNotFound, "not_found", "pack not found")
				return
			}
			log.Printf("packs serve: ServePack failed for %s: %v", slug, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to serve pack")
			return
		}

		if err := json.NewEncoder(w).Encode(packServeViewFrom(view)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}

// ListPublishedPacksHandler handles GET /api/packs. It lists published
// packs only, in summary shape (no puzzles array). Drafts are excluded.
// No authentication is required.
func ListPublishedPacksHandler(svc PublicPackService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		summaries, err := svc.ListPublished(r.Context())
		if err != nil {
			log.Printf("packs serve: ListPublished failed: %v", err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list packs")
			return
		}

		views := make([]PackManifest, len(summaries))
		for i := range summaries {
			views[i] = packSummaryFrom(&summaries[i])
		}
		if err := json.NewEncoder(w).Encode(views); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}
