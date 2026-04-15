package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// mockPuzzleFetcher implements handler.PuzzleFetcher for testing.
type mockPuzzleFetcher struct {
	nextReadyFunc  func(ctx context.Context, size int, mode string) (*repository.PuzzleRecord, error)
	markServedFunc func(ctx context.Context, pk string, sk string) error
}

func (m *mockPuzzleFetcher) NextReady(ctx context.Context, size int, mode string) (*repository.PuzzleRecord, error) {
	return m.nextReadyFunc(ctx, size, mode)
}

func (m *mockPuzzleFetcher) MarkServed(ctx context.Context, pk string, sk string) error {
	return m.markServedFunc(ctx, pk, sk)
}

func TestServeHandler(t *testing.T) {
	readyPuzzle := &repository.PuzzleRecord{
		GridSize:             7,
		Mode:                 "standard",
		ID:                   "puzzle-uuid-123",
		Status:               "ready",
		RegionMap:            [][]int{{0, 0, 1}, {0, 1, 1}, {2, 2, 1}},
		Solution:             [][]bool{{true, false, false}, {false, false, true}, {false, true, false}},
		Pipeline:             "iterative",
		Solver:               "propagation",
		Regions:              "bfs",
		RegionVariance:       0.0,
		GenerationDurationMs: 4200,
		CreatedAt:            "2026-04-15T10:30:00Z",
	}

	tests := []struct {
		name       string
		query      string
		puzzle     *repository.PuzzleRecord
		nextErr    error
		markErr    error
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
			name:       "DynamoDB query error returns 500",
			query:      "?size=7&mode=standard",
			nextErr:    errors.New("dynamodb error"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
		{
			name:       "MarkServed error returns 500",
			query:      "?size=7&mode=standard",
			puzzle:     readyPuzzle,
			markErr:    errors.New("dynamodb update error"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fetcher := &mockPuzzleFetcher{
				nextReadyFunc: func(_ context.Context, _ int, _ string) (*repository.PuzzleRecord, error) {
					if tt.nextErr != nil {
						return nil, tt.nextErr
					}
					return tt.puzzle, nil
				},
				markServedFunc: func(_ context.Context, _ string, _ string) error {
					return tt.markErr
				},
			}
			h := handler.ServeHandler(fetcher)

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

			// puzzleId must match.
			if resp["puzzleId"] != "puzzle-uuid-123" {
				t.Errorf("puzzleId = %v, want %q", resp["puzzleId"], "puzzle-uuid-123")
			}

			// gridSize must match.
			if int(resp["gridSize"].(float64)) != 7 {
				t.Errorf("gridSize = %v, want 7", resp["gridSize"])
			}

			// mode must match.
			if resp["mode"] != "standard" {
				t.Errorf("mode = %v, want %q", resp["mode"], "standard")
			}

			// regionMap must be present.
			if resp["regionMap"] == nil {
				t.Error("regionMap should not be nil")
			}

			// solution must NOT be present.
			if _, exists := resp["solution"]; exists {
				t.Error("solution should not be in response")
			}

			// metadata must be present with expected fields.
			metadata, ok := resp["metadata"].(map[string]interface{})
			if !ok {
				t.Fatal("metadata should be an object")
			}
			if metadata["pipeline"] != "iterative" {
				t.Errorf("metadata.pipeline = %v, want %q", metadata["pipeline"], "iterative")
			}
			if metadata["solver"] != "propagation" {
				t.Errorf("metadata.solver = %v, want %q", metadata["solver"], "propagation")
			}
			if metadata["regions"] != "bfs" {
				t.Errorf("metadata.regions = %v, want %q", metadata["regions"], "bfs")
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
