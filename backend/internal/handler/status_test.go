package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
)

// mockStatusUpdater implements handler.StatusUpdater for testing.
type mockStatusUpdater struct {
	updateFunc func(ctx context.Context, pk string, sk string, status string) error
	lastPK     string
	lastSK     string
	lastStatus string
}

func (m *mockStatusUpdater) UpdateStatus(ctx context.Context, pk, sk, status string) error {
	m.lastPK = pk
	m.lastSK = sk
	m.lastStatus = status
	return m.updateFunc(ctx, pk, sk, status)
}

func TestStatusHandler(t *testing.T) {
	tests := []struct {
		name       string
		puzzleID   string
		query      string
		body       string
		updateErr  error
		wantStatus int
		wantError  string
		wantPK     string
		wantSK     string
	}{
		{
			name:       "updates status to solved",
			puzzleID:   "puzzle-uuid-123",
			query:      "?size=7&mode=standard",
			body:       `{"status":"solved"}`,
			wantStatus: http.StatusOK,
			wantPK:     "7#standard",
			wantSK:     "puzzle-uuid-123",
		},
		{
			name:       "updates status to skipped",
			puzzleID:   "puzzle-uuid-456",
			query:      "?size=9&mode=double",
			body:       `{"status":"skipped"}`,
			wantStatus: http.StatusOK,
			wantPK:     "9#double",
			wantSK:     "puzzle-uuid-456",
		},
		{
			name:       "invalid status returns 400",
			puzzleID:   "puzzle-uuid-123",
			query:      "?size=7&mode=standard",
			body:       `{"status":"deleted"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "empty status returns 400",
			puzzleID:   "puzzle-uuid-123",
			query:      "?size=7&mode=standard",
			body:       `{"status":""}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "missing size query param returns 400",
			puzzleID:   "puzzle-uuid-123",
			query:      "?mode=standard",
			body:       `{"status":"solved"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "missing mode query param returns 400",
			puzzleID:   "puzzle-uuid-123",
			query:      "?size=7",
			body:       `{"status":"solved"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid mode query param returns 400",
			puzzleID:   "puzzle-uuid-123",
			query:      "?size=7&mode=triple",
			body:       `{"status":"solved"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "invalid request body returns 400",
			puzzleID:   "puzzle-uuid-123",
			query:      "?size=7&mode=standard",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_params",
		},
		{
			name:       "DynamoDB error returns 500",
			puzzleID:   "puzzle-uuid-123",
			query:      "?size=7&mode=standard",
			body:       `{"status":"solved"}`,
			updateErr:  errors.New("dynamodb error"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			updater := &mockStatusUpdater{
				updateFunc: func(_ context.Context, _ string, _ string, _ string) error {
					return tt.updateErr
				},
			}
			h := handler.StatusHandler(updater)

			// Set up chi router context for URL params.
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.puzzleID)

			req := httptest.NewRequest(http.MethodPut,
				"/puzzles/"+tt.puzzleID+"/status"+tt.query,
				strings.NewReader(tt.body),
			)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
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
				return
			}

			// Verify the updater was called with correct params.
			if updater.lastPK != tt.wantPK {
				t.Errorf("PK = %q, want %q", updater.lastPK, tt.wantPK)
			}
			if updater.lastSK != tt.wantSK {
				t.Errorf("SK = %q, want %q", updater.lastSK, tt.wantSK)
			}

			// Verify response echoes the status.
			var resp map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			// The status in response should match the request body status.
			var reqBody struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal([]byte(tt.body), &reqBody); err == nil {
				if resp["status"] != reqBody.Status {
					t.Errorf("response status = %q, want %q", resp["status"], reqBody.Status)
				}
			}
		})
	}
}
