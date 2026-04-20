package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
)

func TestGenerateHandlerValidation(t *testing.T) {
	type errorResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantError  string
	}{
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
			name:       "size=16 too large",
			query:      "?size=16&mode=standard",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "missing mode",
			query:      "?size=7",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid mode",
			query:      "?size=7&mode=triple",
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
			// Arrange
			req := httptest.NewRequest(http.MethodGet, "/puzzles/generate"+tt.query, http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			handler.GenerateHandler(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
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
			}
		})
	}
}

func TestGenerateHandlerSuccess(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/puzzles/generate?size=7&mode=standard", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	handler.GenerateHandler(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse success response: %v", err)
	}

	puzzleID, ok := body["puzzleId"].(string)
	if !ok || puzzleID == "" {
		t.Error("expected non-empty puzzleId string")
	}
	if int(body["gridSize"].(float64)) != 7 {
		t.Errorf("gridSize = %v, want 7", body["gridSize"])
	}
	if body["mode"] != "standard" {
		t.Errorf("mode = %v, want %q", body["mode"], "standard")
	}
	regionMap, ok := body["regionMap"].([]interface{})
	if !ok {
		t.Fatal("regionMap is not an array")
	}
	if len(regionMap) != 7 {
		t.Errorf("regionMap has %d rows, want 7", len(regionMap))
	}
	if _, exists := body["solution"]; exists {
		t.Error("solution should not be in response")
	}
}

func TestGenerateHandlerIgnoresLegacyParams(t *testing.T) {
	// Arrange — legacy pipeline/solver/regions params must not cause 400.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/puzzles/generate?size=7&mode=standard&pipeline=iterative&solver=propagation&regions=bfs&regionVariance=0.5&concurrency=2",
		http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	handler.GenerateHandler(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}
