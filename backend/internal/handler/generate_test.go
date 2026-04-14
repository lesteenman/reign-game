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
	}{
		{
			name:       "valid request size=5 mode=standard",
			query:      "?size=5&mode=standard",
			wantStatus: http.StatusOK,
			wantSize:   5,
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
			name:       "unsupported mode=double",
			query:      "?size=5&mode=double",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "both missing",
			query:      "",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/puzzles/generate"+tt.query, http.NoBody)
			rec := httptest.NewRecorder()

			handler.GenerateHandler(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
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

			// mode must be "standard".
			mode, ok := body["mode"].(string)
			if !ok || mode != "standard" {
				t.Errorf("mode = %v, want %q", body["mode"], "standard")
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
