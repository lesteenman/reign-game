package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	configsvc "github.com/eriksteenman/reign-game/backend/internal/service/config"
)

// stubConfigModesService is a minimal stub covering ListEnabledModes
// for the ConfigModesHandler HTTP-boundary tests.
type stubConfigModesService struct {
	configs []configsvc.ConfigView
	err     error
}

func (s *stubConfigModesService) ListEnabledModes(_ context.Context) ([]configsvc.ConfigView, error) {
	return s.configs, s.err
}

func TestConfigModesHandler_EnabledOnly(t *testing.T) {
	// Arrange — service already filters disabled out; stub returns the
	// filtered list as the service would.
	svc := &stubConfigModesService{
		configs: []configsvc.ConfigView{
			{Size: 5, Mode: "standard", Enabled: true, Threshold: 3},
			{Size: 7, Mode: "standard", Enabled: true, Threshold: 3},
			{Size: 9, Mode: "double", Enabled: true, Threshold: 3},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/config/modes", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	ConfigModesHandler(svc)(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp ConfigModesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := []ModeEntry{
		{Size: 5, Mode: "standard"},
		{Size: 7, Mode: "standard"},
		{Size: 9, Mode: "double"},
	}
	if len(resp.Modes) != len(want) {
		t.Fatalf("got %d modes, want %d: %+v", len(resp.Modes), len(want), resp.Modes)
	}
	for i, m := range resp.Modes {
		if m != want[i] {
			t.Errorf("modes[%d] = %+v, want %+v", i, m, want[i])
		}
	}
}

func TestConfigModesHandler_EmptyList(t *testing.T) {
	// Arrange
	svc := &stubConfigModesService{configs: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/config/modes", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	ConfigModesHandler(svc)(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var resp ConfigModesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Modes) != 0 {
		t.Errorf("expected empty modes, got %+v", resp.Modes)
	}
	// The zero-length slice must still serialize as [], not null — the
	// frontend distinguishes "pool fetch failed" from "no enabled combos"
	// by the array's presence.
	if !strings.Contains(rec.Body.String(), `"modes":[]`) {
		t.Errorf("expected modes:[] in body, got %s", rec.Body.String())
	}
}

func TestConfigModesHandler_ServiceError(t *testing.T) {
	// Arrange
	svc := &stubConfigModesService{err: errors.New("boom")}
	req := httptest.NewRequest(http.MethodGet, "/api/config/modes", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	ConfigModesHandler(svc)(rec, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
