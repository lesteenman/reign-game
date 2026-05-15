package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/eriksteenman/reign-game/backend/internal/httperr"
	"github.com/eriksteenman/reign-game/backend/internal/mode"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// ServeService is the application surface the serve handler depends
// on. The handler stays unaware of persistence, race handling, and
// reactive replenish — all of that lives in internal/service/serve.
//
// NextPuzzle returns (nil, nil) when no puzzle is available, whether
// the pool is empty or another replica won the claim race. The handler
// maps that to 404 either way.
type ServeService interface {
	NextPuzzle(ctx context.Context, size int, mode string) (*repository.PuzzleRecord, error)
}

// serveMetadata is the metadata object included in the serve response.
// Seed is encoded as a JSON string so JavaScript clients don't lose
// precision on int64 values beyond the 2^53 safe-integer boundary.
// Seed is omitted for pre-R-06C puzzles that don't have one on record.
type serveMetadata struct {
	Difficulty           int    `json:"difficulty"`
	MaxTier              int    `json:"maxTier"`
	TierCounts           []int  `json:"tierCounts"`
	TraceLen             int    `json:"traceLen"`
	GenerationDurationMs int64  `json:"generationDurationMs"`
	CreatedAt            string `json:"createdAt"`
	Seed                 string `json:"seed,omitempty"`
}

// serveResponse is the JSON response for the serve endpoint.
type serveResponse struct {
	PuzzleID  string        `json:"puzzleId"`
	GridSize  int           `json:"gridSize"`
	Mode      string        `json:"mode"`
	RegionMap [][]int       `json:"regionMap"`
	Metadata  serveMetadata `json:"metadata"`
}

// ServeHandler creates an HTTP handler for GET /puzzles/next. The
// handler validates query parameters and delegates orchestration to
// the serve service; on success it maps the puzzle record to a JSON
// response without the solution.
func ServeHandler(svc ServeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		sizeStr := r.URL.Query().Get("size")
		if sizeStr == "" {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "size parameter is required")
			return
		}
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "size must be an integer")
			return
		}

		modeName := r.URL.Query().Get("mode")
		if modeName == "" {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "mode parameter is required")
			return
		}
		if !mode.IsValid(modeName) {
			httperr.WriteError(w, http.StatusBadRequest, "invalid_params", "mode must be 'standard' or 'double'")
			return
		}

		puzzle, err := svc.NextPuzzle(r.Context(), size, modeName)
		if err != nil {
			log.Printf("serve handler: NextPuzzle failed for %dx%d %s: %v", size, size, modeName, err)
			httperr.WriteError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch puzzle")
			return
		}
		if puzzle == nil {
			httperr.WriteError(w, http.StatusNotFound, "no_puzzles_available", "No puzzles available for this size and mode. Try again shortly.")
			return
		}

		metadata := serveMetadata{
			Difficulty:           puzzle.Difficulty,
			MaxTier:              puzzle.MaxTier,
			TierCounts:           puzzle.TierCounts,
			TraceLen:             puzzle.TraceLen,
			GenerationDurationMs: puzzle.GenerationDurationMs,
			CreatedAt:            puzzle.CreatedAt,
		}
		// Only ship seed for puzzles that have one recorded — pre-R-06C
		// rows have Seed=0 and there's no way to regenerate them, so
		// emitting "0" would mislead anyone trying to reproduce.
		if puzzle.Seed != 0 {
			metadata.Seed = strconv.FormatInt(puzzle.Seed, 10)
		}
		resp := serveResponse{
			PuzzleID:  puzzle.ID,
			GridSize:  puzzle.GridSize,
			Mode:      puzzle.Mode,
			RegionMap: puzzle.RegionMap,
			Metadata:  metadata,
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("serve handler write failed: %v", err)
		}
	}
}
