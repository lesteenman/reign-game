package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
)

func TestHealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		wantStatusCode int
		wantBody       map[string]string
		wantHeader     string
	}{
		{
			name:           "returns 200 with status ok",
			method:         http.MethodGet,
			wantStatusCode: http.StatusOK,
			wantBody:       map[string]string{"status": "ok"},
			wantHeader:     "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			rec := httptest.NewRecorder()

			handler.HealthCheck(rec, req)

			// Verify status code.
			if rec.Code != tt.wantStatusCode {
				t.Errorf("status code: got %d, want %d", rec.Code, tt.wantStatusCode)
			}

			// Verify Content-Type header.
			gotHeader := rec.Header().Get("Content-Type")
			if gotHeader != tt.wantHeader {
				t.Errorf("Content-Type: got %q, want %q", gotHeader, tt.wantHeader)
			}

			// Verify response body.
			var gotBody map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&gotBody); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			for key, wantVal := range tt.wantBody {
				if gotVal, ok := gotBody[key]; !ok || gotVal != wantVal {
					t.Errorf("body[%q]: got %q, want %q", key, gotVal, wantVal)
				}
			}
		})
	}
}
