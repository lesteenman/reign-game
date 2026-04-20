package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// PuzzleFetcher retrieves and marks puzzles from the pool.
type PuzzleFetcher interface {
	NextReady(ctx context.Context, size int, mode string) (*repository.PuzzleRecord, error)
	MarkServed(ctx context.Context, pk, sk string) error
}

// serveMetadata is the metadata object included in the serve response.
type serveMetadata struct {
	Difficulty           int    `json:"difficulty"`
	MaxTier              int    `json:"maxTier"`
	TierCounts           []int  `json:"tierCounts"`
	TraceLen             int    `json:"traceLen"`
	GenerationDurationMs int64  `json:"generationDurationMs"`
	CreatedAt            string `json:"createdAt"`
}

// serveResponse is the JSON response for the serve endpoint.
type serveResponse struct {
	PuzzleID  string        `json:"puzzleId"`
	GridSize  int           `json:"gridSize"`
	Mode      string        `json:"mode"`
	RegionMap [][]int       `json:"regionMap"`
	Metadata  serveMetadata `json:"metadata"`
}

// ServeHandler creates an HTTP handler for GET /puzzles/next.
// It retrieves the next ready puzzle for the requested size and mode,
// marks it as served, and returns it without the solution.
func ServeHandler(fetcher PuzzleFetcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Validate size parameter.
		sizeStr := r.URL.Query().Get("size")
		if sizeStr == "" {
			writeError(w, http.StatusBadRequest, "invalid_params", "size parameter is required")
			return
		}
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
			return
		}

		// Validate mode parameter.
		mode := r.URL.Query().Get("mode")
		if mode == "" {
			writeError(w, http.StatusBadRequest, "invalid_params", "mode parameter is required")
			return
		}
		if mode != ModeStandard && mode != ModeDouble {
			writeError(w, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'")
			return
		}

		// Fetch next ready puzzle.
		puzzle, err := fetcher.NextReady(r.Context(), size, mode)
		if err != nil {
			log.Printf("error fetching next puzzle for %dx%d %s: %v", size, size, mode, err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch puzzle")
			return
		}

		if puzzle == nil {
			writeError(w, http.StatusNotFound, "no_puzzles_available", "No puzzles available for this size and mode. Try again shortly.")
			return
		}

		// Mark as served.
		pk := fmt.Sprintf("%d#%s", size, mode)
		if err := fetcher.MarkServed(r.Context(), pk, puzzle.ID); err != nil {
			log.Printf("error marking puzzle %s as served: %v", puzzle.ID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to serve puzzle")
			return
		}

		resp := serveResponse{
			PuzzleID:  puzzle.ID,
			GridSize:  puzzle.GridSize,
			Mode:      puzzle.Mode,
			RegionMap: puzzle.RegionMap,
			Metadata: serveMetadata{
				Difficulty:           puzzle.Difficulty,
				MaxTier:              puzzle.MaxTier,
				TierCounts:           puzzle.TierCounts,
				TraceLen:             puzzle.TraceLen,
				GenerationDurationMs: puzzle.GenerationDurationMs,
				CreatedAt:            puzzle.CreatedAt,
			},
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("serve handler write failed: %v", err)
		}
	}
}
