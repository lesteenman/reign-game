package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	packsvc "github.com/eriksteenman/reign-game/backend/internal/service/pack"
)

// stubPublicPackService implements handler.PublicPackService for
// HTTP-boundary tests of the public pack read routes.
type stubPublicPackService struct {
	serveView   *packsvc.PackServeView
	serveErr    error
	listSummary []packsvc.PackSummary
	listErr     error
}

func (s *stubPublicPackService) ServePack(_ context.Context, _ string) (*packsvc.PackServeView, error) {
	return s.serveView, s.serveErr
}

func (s *stubPublicPackService) ListPublished(_ context.Context) ([]packsvc.PackSummary, error) {
	return s.listSummary, s.listErr
}

func publishedServeView() *packsvc.PackServeView {
	return &packsvc.PackServeView{
		Slug: "starter", Name: "Starter", Size: 7, Mode: "standard", PuzzleCount: 1,
		Puzzles: []packsvc.ServePuzzle{
			{
				ID:         "id-1",
				RegionMap:  [][]int{{0, 1}, {1, 0}},
				Difficulty: 3,
				MaxTier:    2,
				TierCounts: []int{0, 1, 2, 0, 0},
				TraceLen:   4,
				CreatedAt:  "2026-06-14T00:00:00Z",
				Seed:       42,
			},
		},
	}
}

func TestServePackHandler_200Shape_NoSolution(t *testing.T) {
	// Arrange
	svc := &stubPublicPackService{serveView: publishedServeView()}
	r := chi.NewRouter()
	r.Get("/packs/{slug}", handler.ServePackHandler(svc))
	req := httptest.NewRequest(http.MethodGet, "/packs/starter", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert — 200 with manifest + puzzles, and crucially no solution.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "solution") {
		t.Errorf("response must not contain 'solution': %s", body)
	}

	var resp struct {
		Pack struct {
			Slug        string `json:"slug"`
			Name        string `json:"name"`
			Size        int    `json:"size"`
			Mode        string `json:"mode"`
			PuzzleCount int    `json:"puzzleCount"`
		} `json:"pack"`
		Puzzles []struct {
			PuzzleID  string  `json:"puzzleId"`
			RegionMap [][]int `json:"regionMap"`
			Metadata  struct {
				Difficulty int    `json:"difficulty"`
				Seed       string `json:"seed"`
			} `json:"metadata"`
		} `json:"puzzles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Pack.Slug != "starter" || resp.Pack.Size != 7 || resp.Pack.Mode != "standard" || resp.Pack.PuzzleCount != 1 {
		t.Errorf("manifest = %+v", resp.Pack)
	}
	if len(resp.Puzzles) != 1 {
		t.Fatalf("len(puzzles) = %d, want 1", len(resp.Puzzles))
	}
	if resp.Puzzles[0].PuzzleID != "id-1" || resp.Puzzles[0].Metadata.Difficulty != 3 || resp.Puzzles[0].Metadata.Seed != "42" {
		t.Errorf("puzzle = %+v", resp.Puzzles[0])
	}
}

func TestServePackHandler_NotFound(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{"draft pack", "draft"},
		{"unknown slug", "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange — service returns ErrNotFound for both draft and unknown.
			svc := &stubPublicPackService{serveErr: packsvc.ErrNotFound}
			r := chi.NewRouter()
			r.Get("/packs/{slug}", handler.ServePackHandler(svc))
			req := httptest.NewRequest(http.MethodGet, "/packs/"+tt.slug, http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			r.ServeHTTP(rec, req)

			// Assert — identical 404 for draft and unknown (no leak).
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
			if strings.Contains(rec.Body.String(), "draft") {
				t.Errorf("404 body must not reveal draft status: %s", rec.Body.String())
			}
		})
	}
}

func TestServePackHandler_NoAuthRequired(t *testing.T) {
	// Arrange — the route is mounted with NO auth middleware, mirroring
	// the public /packs group in main.go. A request without any
	// Authorization header must still succeed.
	svc := &stubPublicPackService{serveView: publishedServeView()}
	r := chi.NewRouter()
	r.Get("/packs/{slug}", handler.ServePackHandler(svc)) // no RequireAuth
	req := httptest.NewRequest(http.MethodGet, "/packs/starter", http.NoBody)
	// Explicitly no Authorization header set.
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("unauthenticated request status = %d, want 200", rec.Code)
	}
}

func TestServePackHandler_InternalError(t *testing.T) {
	// Arrange — a non-NotFound error maps to 500.
	svc := &stubPublicPackService{serveErr: context.DeadlineExceeded}
	r := chi.NewRouter()
	r.Get("/packs/{slug}", handler.ServePackHandler(svc))
	req := httptest.NewRequest(http.MethodGet, "/packs/starter", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestListPublishedPacksHandler_SummaryShape(t *testing.T) {
	// Arrange — service already excludes drafts; handler emits summary shape.
	svc := &stubPublicPackService{listSummary: []packsvc.PackSummary{
		{Slug: "pub-a", Name: "A", Size: 7, Mode: "standard", PuzzleCount: 2},
		{Slug: "pub-c", Name: "C", Size: 9, Mode: "double", PuzzleCount: 5},
	}}
	r := chi.NewRouter()
	r.Get("/packs", handler.ListPublishedPacksHandler(svc))
	req := httptest.NewRequest(http.MethodGet, "/packs", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert — array of manifests, no puzzles array on any entry.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "puzzles") {
		t.Errorf("list response must not carry a puzzles array: %s", rec.Body.String())
	}
	var resp []struct {
		Slug        string `json:"slug"`
		PuzzleCount int    `json:"puzzleCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 2 || resp[0].Slug != "pub-a" || resp[0].PuzzleCount != 2 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestListPublishedPacksHandler_NoAuthRequired(t *testing.T) {
	// Arrange — no auth middleware mounted.
	svc := &stubPublicPackService{listSummary: nil}
	r := chi.NewRouter()
	r.Get("/packs", handler.ListPublishedPacksHandler(svc))
	req := httptest.NewRequest(http.MethodGet, "/packs", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	r.ServeHTTP(rec, req)

	// Assert — empty published set still returns 200 with [].
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
