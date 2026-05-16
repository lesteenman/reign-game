package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/queue"
	configsvc "github.com/eriksteenman/reign-game/backend/internal/service/config"
)

// mockConfigReader implements handler.ConfigReader for testing.
type mockConfigReader struct {
	configs []configsvc.ConfigView
	err     error
}

func (m *mockConfigReader) GetAllConfigs(_ context.Context) ([]configsvc.ConfigView, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.configs, nil
}

// mockPoolCounter implements handler.PoolCounter for testing.
type mockPoolCounter struct {
	counts map[string]int
	err    error
}

func (m *mockPoolCounter) CountReady(_ context.Context, size int, mode string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	key := keyFor(size, mode)
	return m.counts[key], nil
}

func keyFor(size int, mode string) string {
	return fmt.Sprintf("%s%d", mode, size)
}

// mockMessagePublisher implements handler.MessagePublisher for testing.
type mockMessagePublisher struct {
	published []queue.GenerationRequest
	err       error
}

func (m *mockMessagePublisher) PublishGenerationRequest(_ context.Context, req *queue.GenerationRequest) error {
	if m.err != nil {
		return m.err
	}
	m.published = append(m.published, *req)
	return nil
}

type replenishResp struct {
	Triggered []struct {
		Size  int    `json:"size"`
		Mode  string `json:"mode"`
		Count int    `json:"count"`
	} `json:"triggered"`
	Skipped []struct {
		Size  int    `json:"size"`
		Mode  string `json:"mode"`
		Ready int    `json:"ready"`
	} `json:"skipped"`
}

func defaultConfig(size int, mode string, threshold int) configsvc.ConfigView {
	return configsvc.ConfigView{
		Size:      size,
		Mode:      mode,
		Threshold: threshold,
		Enabled:   true,
	}
}

func TestReplenishHandler(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		configs       []configsvc.ConfigView
		configErr     error
		counts        map[string]int
		countErr      error
		publishErr    error
		wantStatus    int
		wantTriggered int
		wantSkipped   int
		wantMessages  int
	}{
		{
			name: "all configs enabled and below threshold — triggers all",
			configs: []configsvc.ConfigView{
				defaultConfig(5, "standard", 3),
				defaultConfig(7, "standard", 3),
				defaultConfig(9, "standard", 3),
			},
			counts: map[string]int{
				keyFor(5, "standard"): 0,
				keyFor(7, "standard"): 0,
				keyFor(9, "standard"): 0,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 3,
			wantSkipped:   0,
			wantMessages:  9, // 3 combos * 3 each
		},
		{
			name: "all pools full — nothing triggered",
			configs: []configsvc.ConfigView{
				defaultConfig(5, "standard", 3),
				defaultConfig(7, "standard", 3),
				defaultConfig(9, "standard", 3),
			},
			counts: map[string]int{
				keyFor(5, "standard"): 3,
				keyFor(7, "standard"): 5,
				keyFor(9, "standard"): 3,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 0,
			wantSkipped:   3,
			wantMessages:  0,
		},
		{
			name: "some pools below threshold",
			configs: []configsvc.ConfigView{
				defaultConfig(5, "standard", 3),
				defaultConfig(7, "standard", 3),
				defaultConfig(9, "standard", 3),
			},
			counts: map[string]int{
				keyFor(5, "standard"): 3,
				keyFor(7, "standard"): 1,
				keyFor(9, "standard"): 0,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 2,
			wantSkipped:   1,
			wantMessages:  2 + 3, // 7std needs 2, 9std needs 3
		},
		{
			name: "disabled configs are skipped",
			configs: []configsvc.ConfigView{
				defaultConfig(5, "standard", 3),
				func() configsvc.ConfigView {
					c := defaultConfig(7, "standard", 3)
					c.Enabled = false
					return c
				}(),
				defaultConfig(9, "standard", 3),
			},
			counts: map[string]int{
				keyFor(5, "standard"): 0,
				keyFor(9, "standard"): 0,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 2,
			wantSkipped:   0,
			wantMessages:  6, // 2 combos * 3 each
		},
		{
			name: "different thresholds per combo",
			configs: []configsvc.ConfigView{
				defaultConfig(5, "standard", 5),
				defaultConfig(7, "standard", 2),
			},
			counts: map[string]int{
				keyFor(5, "standard"): 3, // below 5 → needs 2
				keyFor(7, "standard"): 2, // at 2 → skipped
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 1,
			wantSkipped:   1,
			wantMessages:  2,
		},
		{
			name:  "filter by size=7 only",
			query: "?size=7",
			configs: []configsvc.ConfigView{
				defaultConfig(5, "standard", 3),
				defaultConfig(7, "standard", 3),
				defaultConfig(9, "standard", 3),
			},
			counts: map[string]int{
				keyFor(7, "standard"): 1,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 1,
			wantSkipped:   0,
			wantMessages:  2, // 7std needs 2
		},
		{
			name:  "filter by size=9 and mode=standard",
			query: "?size=9&mode=standard",
			configs: []configsvc.ConfigView{
				defaultConfig(5, "standard", 3),
				defaultConfig(7, "standard", 3),
				defaultConfig(9, "standard", 3),
			},
			counts: map[string]int{
				keyFor(9, "standard"): 1,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 1,
			wantSkipped:   0,
			wantMessages:  2,
		},
		{
			name:       "GetAllConfigs error returns 500",
			configErr:  errors.New("dynamo error"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:          "empty config list returns empty response",
			configs:       []configsvc.ConfigView{},
			counts:        map[string]int{},
			wantStatus:    http.StatusOK,
			wantTriggered: 0,
			wantSkipped:   0,
			wantMessages:  0,
		},
		{
			name:       "invalid size param returns 400",
			query:      "?size=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid mode param returns 400",
			query:      "?mode=triple",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "DynamoDB CountReady error returns 500",
			configs: []configsvc.ConfigView{
				defaultConfig(5, "standard", 3),
			},
			countErr:   errors.New("dynamodb error"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "SQS publish error returns 500",
			configs: []configsvc.ConfigView{
				defaultConfig(5, "standard", 3),
			},
			counts: map[string]int{
				keyFor(5, "standard"): 0,
			},
			publishErr: errors.New("sqs error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			configReader := &mockConfigReader{configs: tt.configs, err: tt.configErr}
			counter := &mockPoolCounter{counts: tt.counts, err: tt.countErr}
			pub := &mockMessagePublisher{err: tt.publishErr}
			h := handler.ReplenishHandler(configReader, counter, pub)

			req := httptest.NewRequest(http.MethodPost, "/admin/replenish"+tt.query, http.NoBody)
			rec := httptest.NewRecorder()

			// Act
			h.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
				return
			}

			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp replenishResp
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if len(resp.Triggered) != tt.wantTriggered {
				t.Errorf("triggered count = %d, want %d", len(resp.Triggered), tt.wantTriggered)
			}
			if len(resp.Skipped) != tt.wantSkipped {
				t.Errorf("skipped count = %d, want %d", len(resp.Skipped), tt.wantSkipped)
			}
			if len(pub.published) != tt.wantMessages {
				t.Errorf("published messages = %d, want %d", len(pub.published), tt.wantMessages)
			}
		})
	}
}

func TestReplenishHandler_GenerationParams(t *testing.T) {
	// Arrange
	configs := []configsvc.ConfigView{
		{
			Size:        7,
			Mode:        "standard",
			Threshold:   2,
			Enabled:     true,
			MaxAttempts: 15,
		},
	}
	configReader := &mockConfigReader{configs: configs}
	counter := &mockPoolCounter{counts: map[string]int{keyFor(7, "standard"): 0}}
	pub := &mockMessagePublisher{}
	h := handler.ReplenishHandler(configReader, counter, pub)

	req := httptest.NewRequest(http.MethodPost, "/admin/replenish", http.NoBody)
	rec := httptest.NewRecorder()

	// Act
	h.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(pub.published) != 2 {
		t.Fatalf("published messages = %d, want 2", len(pub.published))
	}

	msg := pub.published[0]
	if msg.Size != 7 {
		t.Errorf("Size = %d, want 7", msg.Size)
	}
	if msg.Mode != "standard" {
		t.Errorf("Mode = %q, want %q", msg.Mode, "standard")
	}
	if msg.MaxAttempts != 15 {
		t.Errorf("MaxAttempts = %d, want 15", msg.MaxAttempts)
	}
}

// TestReplenishHandler_AuthMatrix asserts 401 for anonymous, 403 for
// role=user, 200 for role=admin on POST /api/admin/replenish.
// Empty configs guarantee the admin case returns 200 with empty
// triggered/skipped lists rather than 500 from a missing dependency.
func TestReplenishHandler_AuthMatrix(t *testing.T) {
	for _, tc := range adminAuthMatrix {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			configReader := &mockConfigReader{configs: []configsvc.ConfigView{}}
			counter := &mockPoolCounter{counts: map[string]int{}}
			pub := &mockMessagePublisher{}
			router := mountAdminWithAuth(func(r chi.Router) {
				r.Post("/replenish", handler.ReplenishHandler(configReader, counter, pub))
			}, roleForState(tc.state))

			req := newAdminRequest(tc.state, http.MethodPost, "/api/admin/replenish", nil)
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
