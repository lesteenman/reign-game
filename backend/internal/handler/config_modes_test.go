package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// fakeConfigModesRepo is a minimal stub covering GetAllConfigs for the
// ConfigModesHandler tests. The landing page's config/modes endpoint
// does not need CountReady, so no other calls are stubbed.
type fakeConfigModesRepo struct {
	configs []repository.ConfigRecord
	err     error
}

func (f *fakeConfigModesRepo) GetAllConfigs(ctx context.Context) ([]repository.ConfigRecord, error) {
	return f.configs, f.err
}

func TestConfigModesHandler_EnabledOnly(t *testing.T) {
	// Arrange
	repo := &fakeConfigModesRepo{
		configs: []repository.ConfigRecord{
			{Size: 5, Mode: "standard", Enabled: true, Threshold: 3},
			{Size: 7, Mode: "standard", Enabled: true, Threshold: 3},
			{Size: 9, Mode: "standard", Enabled: false, Threshold: 3},
			{Size: 9, Mode: "double", Enabled: true, Threshold: 3},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/config/modes", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	ConfigModesHandler(repo)(rec, req)

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
	repo := &fakeConfigModesRepo{configs: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/config/modes", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	ConfigModesHandler(repo)(rec, req)

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

func TestConfigModesHandler_AllDisabled(t *testing.T) {
	// Arrange
	repo := &fakeConfigModesRepo{
		configs: []repository.ConfigRecord{
			{Size: 9, Mode: "standard", Enabled: false, Threshold: 3},
			{Size: 9, Mode: "double", Enabled: false, Threshold: 3},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/config/modes", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	ConfigModesHandler(repo)(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var resp ConfigModesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Modes) != 0 {
		t.Errorf("expected zero modes when all disabled, got %+v", resp.Modes)
	}
}

func TestConfigModesHandler_RepoError(t *testing.T) {
	// Arrange
	repo := &fakeConfigModesRepo{err: errors.New("boom")}
	req := httptest.NewRequest(http.MethodGet, "/api/config/modes", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	ConfigModesHandler(repo)(rec, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
