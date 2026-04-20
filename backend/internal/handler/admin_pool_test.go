package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// mockConfigAndCountRepo implements handler.ConfigAndCountRepo for testing.
type mockConfigAndCountRepo struct {
	configs       []repository.ConfigRecord
	configsErr    error
	readyCounts   map[string]int
	countReadyErr error
}

func (m *mockConfigAndCountRepo) GetAllConfigs(_ context.Context) ([]repository.ConfigRecord, error) {
	return m.configs, m.configsErr
}

func (m *mockConfigAndCountRepo) CountReady(_ context.Context, size int, mode string) (int, error) {
	if m.countReadyErr != nil {
		return 0, m.countReadyErr
	}
	key := fmt.Sprintf("%d#%s", size, mode)
	return m.readyCounts[key], nil
}

func TestAdminPoolHandler(t *testing.T) {
	tests := []struct {
		name          string
		configs       []repository.ConfigRecord
		configsErr    error
		readyCounts   map[string]int
		countReadyErr error
		wantStatus    int
		wantError     string
		wantCombos    int
		checkCombos   func(t *testing.T, combos []comboStatusJSON)
	}{
		{
			name: "mixed enabled and disabled combos",
			configs: []repository.ConfigRecord{
				{Size: 5, Mode: "standard", Threshold: 3, Enabled: true},
				{Size: 7, Mode: "standard", Threshold: 5, Enabled: false},
			},
			readyCounts: map[string]int{"5#standard": 3},
			wantStatus:  http.StatusOK,
			wantCombos:  2,
			checkCombos: func(t *testing.T, combos []comboStatusJSON) {
				t.Helper()
				// First combo: enabled, should have real count
				if combos[0].Size != 5 || combos[0].Mode != "standard" {
					t.Errorf("combo[0] size/mode = %d/%s, want 5/standard", combos[0].Size, combos[0].Mode)
				}
				if combos[0].ReadyCount != 3 {
					t.Errorf("combo[0] readyCount = %d, want 3", combos[0].ReadyCount)
				}
				if !combos[0].Config.Enabled {
					t.Error("combo[0] config.enabled = false, want true")
				}
				// Second combo: disabled, should have count 0
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
			name: "all configs disabled returns zero counts",
			configs: []repository.ConfigRecord{
				{Size: 5, Mode: "standard", Enabled: false},
				{Size: 7, Mode: "double", Enabled: false},
			},
			readyCounts: map[string]int{},
			wantStatus:  http.StatusOK,
			wantCombos:  2,
			checkCombos: func(t *testing.T, combos []comboStatusJSON) {
				t.Helper()
				for i, c := range combos {
					if c.ReadyCount != 0 {
						t.Errorf("combo[%d] readyCount = %d, want 0", i, c.ReadyCount)
					}
				}
			},
		},
		{
			name:       "empty config list returns empty combos",
			configs:    []repository.ConfigRecord{},
			wantStatus: http.StatusOK,
			wantCombos: 0,
		},
		{
			name:       "GetAllConfigs error returns 500",
			configsErr: errors.New("dynamo error"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
		{
			name: "CountReady error returns 500",
			configs: []repository.ConfigRecord{
				{Size: 5, Mode: "standard", Enabled: true},
			},
			countReadyErr: errors.New("dynamo error"),
			wantStatus:    http.StatusInternalServerError,
			wantError:     "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := &mockConfigAndCountRepo{
				configs:       tt.configs,
				configsErr:    tt.configsErr,
				readyCounts:   tt.readyCounts,
				countReadyErr: tt.countReadyErr,
			}
			h := handler.AdminPoolHandler(repo)

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
