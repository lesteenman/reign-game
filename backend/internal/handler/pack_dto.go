package handler

import (
	"strconv"

	packsvc "github.com/eriksteenman/reign-game/backend/internal/service/pack"
)

// The handler layer speaks explicit DTOs for PACK items. packsvc views
// are the service-level projections; these DTOs shape the HTTP request
// and response surfaces.

// PackCreateRequest is the JSON body of POST /api/admin/packs. Slug,
// size, and mode live in the body because the endpoint has no URL path
// parameters.
type PackCreateRequest struct {
	Slug      string   `json:"slug"`
	Name      string   `json:"name"`
	Size      int      `json:"size"`
	Mode      string   `json:"mode"`
	PuzzleIDs []string `json:"puzzleIds"`
	Published bool     `json:"published"`
}

// PackUpdateRequest is the JSON body of PUT /api/admin/packs/{slug}.
// Slug, size, and mode are immutable and not carried here.
type PackUpdateRequest struct {
	Name      string   `json:"name"`
	PuzzleIDs []string `json:"puzzleIds"`
	Published bool     `json:"published"`
}

// PackView is the JSON shape returned for a single pack.
type PackView struct {
	Slug      string   `json:"slug"`
	Name      string   `json:"name"`
	Size      int      `json:"size"`
	Mode      string   `json:"mode"`
	Published bool     `json:"published"`
	PuzzleIDs []string `json:"puzzleIds"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

// PackCandidate is the per-puzzle shape returned by the candidates
// endpoint — mirrors the serve DTO (puzzleId, regionMap, metadata), no
// solution.
type PackCandidate struct {
	PuzzleID  string            `json:"puzzleId"`
	RegionMap [][]int           `json:"regionMap"`
	Metadata  packCandidateMeta `json:"metadata"`
}

// packCandidateMeta mirrors the serve metadata block (minus seed, which
// pack assembly does not need).
type packCandidateMeta struct {
	Difficulty           int    `json:"difficulty"`
	MaxTier              int    `json:"maxTier"`
	TierCounts           []int  `json:"tierCounts"`
	TraceLen             int    `json:"traceLen"`
	GenerationDurationMs int64  `json:"generationDurationMs"`
	CreatedAt            string `json:"createdAt"`
}

// PackServeView is the public read response for GET /api/packs/{slug}:
// the manifest plus every puzzle's play data, no solution.
type PackServeView struct {
	Pack    PackManifest      `json:"pack"`
	Puzzles []PackServePuzzle `json:"puzzles"`
}

// PackManifest is the manifest block carried by both the serve and list
// responses. Size and mode are pack-level (single-combo pack).
type PackManifest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Size        int    `json:"size"`
	Mode        string `json:"mode"`
	PuzzleCount int    `json:"puzzleCount"`
}

// PackServePuzzle is one puzzle's play data within a served pack —
// mirrors the serve DTO (puzzleId, regionMap, metadata), no solution.
type PackServePuzzle struct {
	PuzzleID  string        `json:"puzzleId"`
	RegionMap [][]int       `json:"regionMap"`
	Metadata  serveMetadata `json:"metadata"`
}

// packServeViewFrom builds the serve response from a packsvc.PackServeView.
func packServeViewFrom(v *packsvc.PackServeView) PackServeView {
	puzzles := make([]PackServePuzzle, len(v.Puzzles))
	for i := range v.Puzzles {
		p := &v.Puzzles[i]
		meta := serveMetadata{
			Difficulty:           p.Difficulty,
			MaxTier:              p.MaxTier,
			TierCounts:           p.TierCounts,
			TraceLen:             p.TraceLen,
			GenerationDurationMs: p.GenerationDurationMs,
			CreatedAt:            p.CreatedAt,
		}
		// Match the serve DTO: omit seed for pre-R-06C rows (Seed=0)
		// rather than emitting a misleading "0".
		if p.Seed != 0 {
			meta.Seed = strconv.FormatInt(p.Seed, 10)
		}
		puzzles[i] = PackServePuzzle{
			PuzzleID:  p.ID,
			RegionMap: p.RegionMap,
			Metadata:  meta,
		}
	}
	return PackServeView{
		Pack: PackManifest{
			Slug:        v.Slug,
			Name:        v.Name,
			Size:        v.Size,
			Mode:        v.Mode,
			PuzzleCount: v.PuzzleCount,
		},
		Puzzles: puzzles,
	}
}

// packSummaryFrom builds a list-response manifest from a packsvc.PackSummary.
func packSummaryFrom(s *packsvc.PackSummary) PackManifest {
	return PackManifest{
		Slug:        s.Slug,
		Name:        s.Name,
		Size:        s.Size,
		Mode:        s.Mode,
		PuzzleCount: s.PuzzleCount,
	}
}

// packViewFrom builds a handler PackView from a packsvc.PackView.
func packViewFrom(v *packsvc.PackView) PackView {
	ids := v.PuzzleIDs
	if ids == nil {
		ids = []string{}
	}
	return PackView{
		Slug:      v.Slug,
		Name:      v.Name,
		Size:      v.Size,
		Mode:      v.Mode,
		Published: v.Published,
		PuzzleIDs: ids,
		CreatedAt: v.CreatedAt,
		UpdatedAt: v.UpdatedAt,
	}
}

// packCandidateFrom builds a PackCandidate from a packsvc.Candidate.
func packCandidateFrom(c *packsvc.Candidate) PackCandidate {
	return PackCandidate{
		PuzzleID:  c.ID,
		RegionMap: c.RegionMap,
		Metadata: packCandidateMeta{
			Difficulty: c.Difficulty,
			MaxTier:    c.MaxTier,
			TierCounts: c.TierCounts,
			TraceLen:   c.TraceLen,
			CreatedAt:  c.CreatedAt,
		},
	}
}

// toCreateInput maps a PackCreateRequest to a packsvc.CreateInput.
func (req *PackCreateRequest) toCreateInput() packsvc.CreateInput {
	return packsvc.CreateInput{
		Slug:      req.Slug,
		Name:      req.Name,
		Size:      req.Size,
		Mode:      req.Mode,
		PuzzleIDs: req.PuzzleIDs,
		Published: req.Published,
	}
}

// toUpdateInput maps a PackUpdateRequest to a packsvc.UpdateInput.
func (req *PackUpdateRequest) toUpdateInput() packsvc.UpdateInput {
	return packsvc.UpdateInput{
		Name:      req.Name,
		PuzzleIDs: req.PuzzleIDs,
		Published: req.Published,
	}
}
