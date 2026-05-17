package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
	poolsvc "github.com/eriksteenman/reign-game/backend/internal/service/pool"
)

// stubPoolService implements handler.PoolService for testing the HTTP
// boundary. The service-layer behaviors (DDB calls, timing logs) are
// covered in internal/service/pool.
type stubPoolService struct {
	entries []poolsvc.ComboEntry
	err     error
}

func (s *stubPoolService) LoadPool(_ context.Context) ([]poolsvc.ComboEntry, error) {
	return s.entries, s.err
}

func TestAdminPoolHandler(t *testing.T) {
	tests := []struct {
		name        string
		entries     []poolsvc.ComboEntry
		svcErr      error
		wantStatus  int
		wantError   string
		wantCombos  int
		checkCombos func(t *testing.T, combos []comboStatusJSON)
	}{
		{
			name: "mixed enabled and disabled combos",
			entries: []poolsvc.ComboEntry{
				{Config: repository.ConfigRecord{Size: 5, Mode: "standard", Threshold: 3, Enabled: true}, ReadyCount: 3},
				{Config: repository.ConfigRecord{Size: 7, Mode: "standard", Threshold: 5, Enabled: false}, ReadyCount: 0},
			},
			wantStatus: http.StatusOK,
			wantCombos: 2,
			checkCombos: func(t *testing.T, combos []comboStatusJSON) {
				t.Helper()
				if combos[0].Size != 5 || combos[0].Mode != "standard" {
					t.Errorf("combo[0] size/mode = %d/%s, want 5/standard", combos[0].Size, combos[0].Mode)
				}
				if combos[0].ReadyCount != 3 {
					t.Errorf("combo[0] readyCount = %d, want 3", combos[0].ReadyCount)
				}
				if !combos[0].Config.Enabled {
					t.Error("combo[0] config.enabled = false, want true")
				}
				if combos[1].Size != 7 || combos[1].Mode != "standard" {
					t.Errorf("combo[1] size/mode = %d/%s, want 7/standard", combos[1].Size, combos[1].Mode)
				}
				if combos[1].ReadyCount != 0 {
					t.Errorf("combo[1] readyCount = %d, want 0", combos[1].ReadyCount)
				}
				if combos[1].Config.Enabled {
					t.Error("combo[1] config.enabled = true, want false")
				}
			},
		},
		{
			name:       "empty pool returns empty combos",
			entries:    []poolsvc.ComboEntry{},
			wantStatus: http.StatusOK,
			wantCombos: 0,
		},
		{
			name:       "service error returns 500",
			svcErr:     errors.New("service failure"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			svc := &stubPoolService{entries: tt.entries, err: tt.svcErr}
			h := handler.AdminPoolHandler(svc)

			req := httptest.NewRequest(http.MethodGet, "/admin/pool", http.NoBody)
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

			var resp adminPoolResponseJSON
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if len(resp.Combos) != tt.wantCombos {
				t.Errorf("combos length = %d, want %d", len(resp.Combos), tt.wantCombos)
			}

			if tt.checkCombos != nil {
				tt.checkCombos(t, resp.Combos)
			}
		})
	}
}

// Response types for JSON unmarshaling in tests.
type configResponseJSON struct {
	Threshold   int  `json:"threshold"`
	Enabled     bool `json:"enabled"`
	MaxAttempts int  `json:"maxAttempts,omitempty"`
}

type comboStatusJSON struct {
	Size       int                `json:"size"`
	Mode       string             `json:"mode"`
	Config     configResponseJSON `json:"config"`
	ReadyCount int                `json:"readyCount"`
}

type adminPoolResponseJSON struct {
	Combos []comboStatusJSON `json:"combos"`
}

// TestAdminPoolHandler_AuthMatrix proves the route returns 401 for
// an anonymous request, 403 for a signed-in non-admin, and 200 for
// an admin — the three states RequireAuth + RequireAdmin enforce on
// every admin route.
func TestAdminPoolHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			svc := &stubPoolService{} // no data needed — admin case returns 200 with empty combos
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Get("/pool", handler.AdminPoolHandler(svc))
			}, roleForState(tc.state))

			req := newAdminRequest(tc.state, http.MethodGet, "/api/admin/pool", nil)
			rec := httptest.NewRecorder()

			// Act
			router.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tc.wantCode, rec.Body.String())
			}
		})
	}
}
