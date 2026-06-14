package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	"github.com/eriksteenman/reign-game/backend/internal/mode"
	packsvc "github.com/eriksteenman/reign-game/backend/internal/service/pack"
)

// PackService is the application surface the admin-pack handlers depend
// on. Errors are mapped to HTTP status by the handlers via errors.Is /
// errors.As against the packsvc sentinels.
type PackService interface {
	Create(ctx context.Context, in *packsvc.CreateInput) error
	Update(ctx context.Context, slug string, in packsvc.UpdateInput) error
	Delete(ctx context.Context, slug string) error
	Get(ctx context.Context, slug string) (*packsvc.PackView, error)
	List(ctx context.Context) ([]packsvc.PackView, error)
	ListCandidates(ctx context.Context, size int, mode string) ([]packsvc.Candidate, error)
}

// ListPacksHandler handles GET /admin/packs.
func ListPacksHandler(svc PackService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		packs, err := svc.List(r.Context())
		if err != nil {
			log.Printf("admin pack: List failed: %v", err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list packs")
			return
		}

		views := make([]PackView, len(packs))
		for i := range packs {
			views[i] = packViewFrom(&packs[i])
		}
		if err := json.NewEncoder(w).Encode(views); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}

// GetPackHandler handles GET /admin/packs/{slug}.
func GetPackHandler(svc PackService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		slug := chi.URLParam(r, "slug")
		view, err := svc.Get(r.Context(), slug)
		if err != nil {
			if errors.Is(err, packsvc.ErrNotFound) {
				httperr.WriteError(w, http.StatusNotFound, "not_found",
					fmt.Sprintf("pack not found: %s", slug))
				return
			}
			log.Printf("admin pack: Get failed for %s: %v", slug, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get pack")
			return
		}

		if err := json.NewEncoder(w).Encode(packViewFrom(view)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}

// CreatePackHandler handles POST /admin/packs.
func CreatePackHandler(svc PackService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req PackCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}

		in := req.toCreateInput()
		if err := svc.Create(r.Context(), &in); err != nil {
			writePackWriteError(w, err, "create", req.Slug)
			return
		}

		view, err := svc.Get(r.Context(), req.Slug)
		if err != nil {
			log.Printf("admin pack: Get after Create failed for %s: %v", req.Slug, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to read created pack")
			return
		}

		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(packViewFrom(view)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}

// UpdatePackHandler handles PUT /admin/packs/{slug}.
func UpdatePackHandler(svc PackService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		slug := chi.URLParam(r, "slug")
		var req PackUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "invalid request body")
			return
		}

		if err := svc.Update(r.Context(), slug, req.toUpdateInput()); err != nil {
			if errors.Is(err, packsvc.ErrNotFound) {
				httperr.WriteError(w, http.StatusNotFound, "not_found",
					fmt.Sprintf("pack not found: %s", slug))
				return
			}
			writePackWriteError(w, err, "update", slug)
			return
		}

		view, err := svc.Get(r.Context(), slug)
		if err != nil {
			log.Printf("admin pack: Get after Update failed for %s: %v", slug, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to read updated pack")
			return
		}

		if err := json.NewEncoder(w).Encode(packViewFrom(view)); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}

// DeletePackHandler handles DELETE /admin/packs/{slug}.
func DeletePackHandler(svc PackService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		if err := svc.Delete(r.Context(), slug); err != nil {
			if errors.Is(err, packsvc.ErrNotFound) {
				httperr.WriteError(w, http.StatusNotFound, "not_found",
					fmt.Sprintf("pack not found: %s", slug))
				return
			}
			log.Printf("admin pack: Delete failed for %s: %v", slug, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete pack")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ListPackCandidatesHandler handles GET /admin/packs/candidates?size=&mode=.
func ListPackCandidatesHandler(svc PackService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		sizeStr := r.URL.Query().Get("size")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
			return
		}
		if status, code, msg := validateSize(size); status != 0 {
			httperr.WriteError(w, status, code, msg)
			return
		}
		modeName := r.URL.Query().Get("mode")
		if !mode.IsValid(modeName) {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'")
			return
		}

		candidates, err := svc.ListCandidates(r.Context(), size, modeName)
		if err != nil {
			if errors.Is(err, packsvc.ErrInvalidCombo) {
				httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "invalid size+mode combo")
				return
			}
			log.Printf("admin pack: ListCandidates failed for %d#%s: %v", size, modeName, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list candidates")
			return
		}

		views := make([]PackCandidate, len(candidates))
		for i := range candidates {
			views[i] = packCandidateFrom(&candidates[i])
		}
		if err := json.NewEncoder(w).Encode(views); err != nil {
			log.Printf("write response error: %v", err)
		}
	}
}

// writePackWriteError maps create/update service errors to HTTP status:
// 400 (invalid slug/combo), 409 (duplicate slug), 422 (invalid puzzle /
// empty publish), 500 otherwise.
func writePackWriteError(w http.ResponseWriter, err error, op, slug string) {
	switch {
	case errors.Is(err, packsvc.ErrInvalidSlug):
		httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "slug must be lowercase kebab-case")
	case errors.Is(err, packsvc.ErrInvalidCombo):
		httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "invalid size+mode combo")
	case errors.Is(err, packsvc.ErrAlreadyExists):
		httperr.WriteError(w, http.StatusConflict, "conflict",
			fmt.Sprintf("pack already exists: %s", slug))
	case errors.Is(err, packsvc.ErrEmptyPublish):
		httperr.WriteError(w, http.StatusUnprocessableEntity, "unprocessable",
			"cannot publish an empty pack")
	default:
		var invalid *packsvc.InvalidPuzzleError
		if errors.As(err, &invalid) {
			httperr.WriteError(w, http.StatusUnprocessableEntity, "unprocessable",
				fmt.Sprintf("puzzle %s is invalid: %s", invalid.PuzzleID, invalid.Reason))
			return
		}
		log.Printf("admin pack: %s failed for %s: %v", op, slug, err)
		httperr.WriteError(w, http.StatusInternalServerError, "internal_error",
			fmt.Sprintf("failed to %s pack", op))
	}
}
