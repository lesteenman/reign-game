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
	statussvc "github.com/eriksteenman/reign-game/backend/internal/service/status"
)

// stubStatusService implements handler.StatusService for HTTP-boundary tests.
type stubStatusService struct {
	err           error
	lastSize      int
	lastMode      string
	lastPuzzleID  string
	lastStatusArg string
}

func (s *stubStatusService) SetStatus(_ context.Context, size int, mode, puzzleID, status string) error {
	s.lastSize = size
	s.lastMode = mode
	s.lastPuzzleID = puzzleID
	s.lastStatusArg = status
	return s.err
}

func TestStatusHandler(t *testing.T) {
	tests := []struct {
		name         string
		puzzleID     string
		query        string
		body         string
		svcErr       error
		wantStatus   int
		wantError    string
		wantSize     int
		wantMode     string
		wantStatusArg string
	}{
		{
			name:          "updates status to solved",
			puzzleID:      "puzzle-uuid-123",
			query:         "?size=7&mode=standard",
			body:          `{"status":"solved"}`,
			wantStatus:    http.StatusOK,
			wantSize:      7,
			wantMode:      "standard",
			wantStatusArg: "solved",
		},
		{
			name:          "updates status to skipped",
			puzzleID:      "puzzle-uuid-456",
			query:         "?size=9&mode=double",
			body:          `{"status":"skipped"}`,
			wantStatus:    http.StatusOK,
			wantSize:      9,
			wantMode:      "double",
			wantStatusArg: "skipped",
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
			name:       "service not-found returns 404",
			puzzleID:   "puzzle-uuid-123",
			query:      "?size=7&mode=standard",
			body:       `{"status":"solved"}`,
			svcErr:     statussvc.ErrPuzzleNotFound,
			wantStatus: http.StatusNotFound,
			wantError:  "not_found",
		},
		{
			name:       "service error returns 500",
			puzzleID:   "puzzle-uuid-123",
			query:      "?size=7&mode=standard",
			body:       `{"status":"solved"}`,
			svcErr:     errors.New("dynamodb error"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubStatusService{err: tt.svcErr}
			h := handler.StatusHandler(svc)

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

			if svc.lastSize != tt.wantSize {
				t.Errorf("size = %d, want %d", svc.lastSize, tt.wantSize)
			}
			if svc.lastMode != tt.wantMode {
				t.Errorf("mode = %q, want %q", svc.lastMode, tt.wantMode)
			}
			if svc.lastPuzzleID != tt.puzzleID {
				t.Errorf("puzzleID = %q, want %q", svc.lastPuzzleID, tt.puzzleID)
			}
			if svc.lastStatusArg != tt.wantStatusArg {
				t.Errorf("status arg = %q, want %q", svc.lastStatusArg, tt.wantStatusArg)
			}

			var resp map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp["status"] != tt.wantStatusArg {
				t.Errorf("response status = %q, want %q", resp["status"], tt.wantStatusArg)
			}
		})
	}
}
