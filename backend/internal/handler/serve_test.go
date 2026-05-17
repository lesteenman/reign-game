package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	servesvc "github.com/eriksteenman/reign-game/backend/internal/service/serve"
)

// stubServeService implements handler.ServeService for testing the
// HTTP boundary. The service-layer behaviors (claim race, replenish
// hook firing) are covered in internal/service/serve.
type stubServeService struct {
	puzzle *servesvc.PuzzleView
	err    error
}

func (s *stubServeService) NextPuzzle(_ context.Context, _ int, _ string) (*servesvc.PuzzleView, error) {
	return s.puzzle, s.err
}

func TestServeHandler(t *testing.T) {
	readyPuzzle := &servesvc.PuzzleView{
		GridSize:             7,
		Mode:                 "standard",
		ID:                   "puzzle-uuid-123",
		RegionMap:            [][]int{{0, 0, 1}, {0, 1, 1}, {2, 2, 1}},
		Difficulty:           2,
		MaxTier:              2,
		TierCounts:           []int{0, 3, 1, 0, 0},
		TraceLen:             4,
		GenerationDurationMs: 4200,
		CreatedAt:            "2026-04-15T10:30:00Z",
	}

	tests := []struct {
		name       string
		query      string
		puzzle     *servesvc.PuzzleView
		svcErr     error
		wantStatus int
		wantError  string
	}{
		{
			name:       "returns puzzle when available",
			query:      "?size=7&mode=standard",
			puzzle:     readyPuzzle,
			wantStatus: http.StatusOK,
		},
		{
			name:       "returns 404 when no puzzles available",
			query:      "?size=5&mode=standard",
			puzzle:     nil,
			wantStatus: http.StatusNotFound,
			wantError:  "no_puzzles_available",
		},
		{
			name:       "missing size returns 400",
			query:      "?mode=standard",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid size returns 400",
			query:      "?size=abc&mode=standard",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "missing mode returns 400",
			query:      "?size=7",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid mode returns 400",
			query:      "?size=7&mode=triple",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "service error returns 500",
			query:      "?size=7&mode=standard",
			svcErr:     errors.New("ddb error"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubServeService{puzzle: tt.puzzle, err: tt.svcErr}
			h := handler.ServeHandler(svc)
			req := httptest.NewRequest(http.MethodGet, "/puzzles/next"+tt.query, http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			h.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantError != "" {
				var errResp struct {
					Error   string `json:"error"`
					Message string `json:"message"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if errResp.Error != tt.wantError {
					t.Errorf("error = %q, want %q", errResp.Error, tt.wantError)
				}
				if errResp.Message == "" {
					t.Error("expected non-empty error message")
				}
				return
			}

			// Verify success response shape.
			var resp map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse success response: %v", err)
			}

			if resp["puzzleId"] != "puzzle-uuid-123" {
				t.Errorf("puzzleId = %v, want %q", resp["puzzleId"], "puzzle-uuid-123")
			}
			if int(resp["gridSize"].(float64)) != 7 {
				t.Errorf("gridSize = %v, want 7", resp["gridSize"])
			}
			if resp["mode"] != "standard" {
				t.Errorf("mode = %v, want %q", resp["mode"], "standard")
			}
			if resp["regionMap"] == nil {
				t.Error("regionMap should not be nil")
			}
			if _, exists := resp["solution"]; exists {
				t.Error("solution should not be in response")
			}

			metadata, ok := resp["metadata"].(map[string]interface{})
			if !ok {
				t.Fatal("metadata should be an object")
			}
			if int(metadata["difficulty"].(float64)) != 2 {
				t.Errorf("metadata.difficulty = %v, want 2", metadata["difficulty"])
			}
			if int(metadata["maxTier"].(float64)) != 2 {
				t.Errorf("metadata.maxTier = %v, want 2", metadata["maxTier"])
			}
			if int(metadata["traceLen"].(float64)) != 4 {
				t.Errorf("metadata.traceLen = %v, want 4", metadata["traceLen"])
			}
			if int(metadata["generationDurationMs"].(float64)) != 4200 {
				t.Errorf("metadata.generationDurationMs = %v, want 4200", metadata["generationDurationMs"])
			}
			if metadata["createdAt"] != "2026-04-15T10:30:00Z" {
				t.Errorf("metadata.createdAt = %v, want %q", metadata["createdAt"], "2026-04-15T10:30:00Z")
			}
		})
	}
}
