package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
)

func TestGenerateHandlerSizeRejection(t *testing.T) {
	// Verify the handler rejects out-of-range sizes immediately (no generation).
	tests := []struct {
		name string
		size string
	}{
		{name: "size=2 rejected", size: "2"},
		{name: "size=1 rejected", size: "1"},
		{name: "size=0 rejected", size: "0"},
		{name: "size=-1 rejected", size: "-1"},
		{name: "size=16 rejected", size: "16"},
		{name: "size=100 rejected", size: "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(http.MethodGet, "/puzzles/generate?size="+tt.size+"&mode=standard", http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			handler.GenerateHandler(rec, req)

			// Assert
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for size=%s, got %d", tt.size, rec.Code)
			}
		})
	}
}

func TestGenerateHandler(t *testing.T) {
	type errorResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantError  string // empty means expect 200 success
		wantSize   int    // expected gridSize in response (0 = use default check)
		wantMode   string // expected mode in response
	}{
		{
			name:       "valid request size=5 mode=standard",
			query:      "?size=5&mode=standard",
			wantStatus: http.StatusOK,
			wantSize:   5,
			wantMode:   "standard",
		},
		{
			name:       "missing size",
			query:      "?mode=standard",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid size not a number",
			query:      "?size=abc&mode=standard",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "size=2 too small",
			query:      "?size=2&mode=standard",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "size=0 too small",
			query:      "?size=0&mode=standard",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "size=16 too large",
			query:      "?size=16&mode=standard",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "size=100 too large",
			query:      "?size=100&mode=standard",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "missing mode",
			query:      "?size=5",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid mode",
			query:      "?size=5&mode=triple",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "both missing",
			query:      "",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		// Strategy parameter validation.
		{
			name:       "invalid pipeline",
			query:      "?size=5&mode=standard&pipeline=invalid",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid solver",
			query:      "?size=5&mode=standard&solver=invalid",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid regions",
			query:      "?size=5&mode=standard&regions=invalid",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "regionVariance negative",
			query:      "?size=5&mode=standard&regionVariance=-0.1",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "regionVariance too large",
			query:      "?size=5&mode=standard&regionVariance=1.1",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "regionVariance not a number",
			query:      "?size=5&mode=standard&regionVariance=abc",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "regionVariance=NaN rejected",
			query:      "?size=5&mode=standard&regionVariance=NaN",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		// Solution-first pipeline not exposed via API.
		{
			name:       "solution-first pipeline rejected",
			query:      "?size=5&mode=standard&pipeline=solution-first",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		// Valid strategy combinations.
		{
			name:       "explicit defaults: region-first + propagation + bfs",
			query:      "?size=5&mode=standard&pipeline=region-first&solver=propagation&regions=bfs",
			wantStatus: http.StatusOK,
			wantSize:   5,
			wantMode:   "standard",
		},
		{
			name:       "iterative pipeline with backtrack solver and wfc regions",
			query:      "?size=5&mode=standard&pipeline=iterative&solver=backtrack&regions=wfc",
			wantStatus: http.StatusOK,
			wantSize:   5,
			wantMode:   "standard",
		},
		{
			name:       "constraint-aware pipeline with propagation solver",
			query:      "?size=5&mode=standard&pipeline=constraint-aware&solver=propagation",
			wantStatus: http.StatusOK,
			wantSize:   5,
			wantMode:   "standard",
		},
		{
			name:       "regionVariance=0.5 with iterative pipeline",
			query:      "?size=5&mode=standard&pipeline=iterative&regionVariance=0.5",
			wantStatus: http.StatusOK,
			wantSize:   5,
			wantMode:   "standard",
		},
		{
			name:       "regionVariance=0.0 explicit",
			query:      "?size=5&mode=standard&regionVariance=0.0",
			wantStatus: http.StatusOK,
			wantSize:   5,
			wantMode:   "standard",
		},
		{
			name:       "regionVariance=1.0 max",
			query:      "?size=5&mode=standard&regionVariance=1.0",
			wantStatus: http.StatusOK,
			wantSize:   5,
			wantMode:   "standard",
		},
		// Double Queens mode.
		{
			name:       "mode=double with size=9",
			query:      "?size=9&mode=double&deducible=false",
			wantStatus: http.StatusOK,
			wantSize:   9,
			wantMode:   "double",
		},
		// Concurrency parameter validation.
		{
			name:       "concurrency=0 rejected",
			query:      "?size=5&mode=standard&concurrency=0",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "concurrency=9 rejected",
			query:      "?size=5&mode=standard&concurrency=9",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "concurrency=abc rejected",
			query:      "?size=5&mode=standard&concurrency=abc",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "concurrency=-1 rejected",
			query:      "?size=5&mode=standard&concurrency=-1",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "concurrency=2 valid",
			query:      "?size=5&mode=standard&concurrency=2",
			wantStatus: http.StatusOK,
			wantSize:   5,
			wantMode:   "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(http.MethodGet, "/puzzles/generate"+tt.query, http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			handler.GenerateHandler(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			contentType := rec.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
			}

			if tt.wantError != "" {
				var errBody errorResp
				if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if errBody.Error != tt.wantError {
					t.Errorf("error = %q, want %q", errBody.Error, tt.wantError)
				}
				if errBody.Message == "" {
					t.Error("expected non-empty error message")
				}
				return
			}

			// Success case: verify response shape.
			var body map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to parse success response: %v", err)
			}

			expectedSize := tt.wantSize

			// puzzleId must be a non-empty string.
			puzzleID, ok := body["puzzleId"].(string)
			if !ok || puzzleID == "" {
				t.Error("expected non-empty puzzleId string")
			}

			// gridSize must match requested size.
			gridSize, ok := body["gridSize"].(float64)
			if !ok || int(gridSize) != expectedSize {
				t.Errorf("gridSize = %v, want %d", body["gridSize"], expectedSize)
			}

			// mode must match expected mode.
			wantMode := tt.wantMode
			if wantMode == "" {
				wantMode = "standard"
			}
			mode, ok := body["mode"].(string)
			if !ok || mode != wantMode {
				t.Errorf("mode = %v, want %q", body["mode"], wantMode)
			}

			// regionMap must be an NxN 2D array of ints.
			regionMap, ok := body["regionMap"].([]interface{})
			if !ok {
				t.Fatal("regionMap is not an array")
			}
			if len(regionMap) != expectedSize {
				t.Fatalf("regionMap has %d rows, want %d", len(regionMap), expectedSize)
			}
			for i, row := range regionMap {
				rowArr, ok := row.([]interface{})
				if !ok {
					t.Fatalf("regionMap[%d] is not an array", i)
				}
				if len(rowArr) != expectedSize {
					t.Fatalf("regionMap[%d] has %d cols, want %d", i, len(rowArr), expectedSize)
				}
				for j, val := range rowArr {
					if _, ok := val.(float64); !ok {
						t.Errorf("regionMap[%d][%d] = %T, want number", i, j, val)
					}
				}
			}

			// solution must NOT be present.
			if _, exists := body["solution"]; exists {
				t.Error("solution should not be in response")
			}
		})
	}
}
