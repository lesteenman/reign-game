package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eriksteenman/reign-game/backend/internal/handler"
	"github.com/eriksteenman/reign-game/backend/internal/queue"
)

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
	return mode + string(rune('0'+size))
}

// mockMessagePublisher implements handler.MessagePublisher for testing.
type mockMessagePublisher struct {
	published []queue.GenerationRequest
	err       error
}

func (m *mockMessagePublisher) PublishGenerationRequest(_ context.Context, req queue.GenerationRequest) error {
	if m.err != nil {
		return m.err
	}
	m.published = append(m.published, req)
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

func TestReplenishHandler(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		counts        map[string]int
		countErr      error
		publishErr    error
		wantStatus    int
		wantTriggered int
		wantSkipped   int
		wantMessages  int
	}{
		{
			name: "all pools full — nothing triggered",
			counts: map[string]int{
				keyFor(5, "standard"): 3,
				keyFor(7, "standard"): 5,
				keyFor(9, "standard"): 3,
				keyFor(7, "double"):   4,
				keyFor(9, "double"):   3,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 0,
			wantSkipped:   5,
			wantMessages:  0,
		},
		{
			name: "some pools below threshold",
			counts: map[string]int{
				keyFor(5, "standard"): 3,
				keyFor(7, "standard"): 1,
				keyFor(9, "standard"): 0,
				keyFor(7, "double"):   3,
				keyFor(9, "double"):   2,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 3,
			wantSkipped:   2,
			wantMessages:  2 + 3 + 1, // 7std needs 2, 9std needs 3, 9dbl needs 1
		},
		{
			name: "all pools empty",
			counts: map[string]int{
				keyFor(5, "standard"): 0,
				keyFor(7, "standard"): 0,
				keyFor(9, "standard"): 0,
				keyFor(7, "double"):   0,
				keyFor(9, "double"):   0,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 5,
			wantSkipped:   0,
			wantMessages:  15, // 5 combos * 3 each
		},
		{
			name:  "filter by size=7 only",
			query: "?size=7",
			counts: map[string]int{
				keyFor(7, "standard"): 1,
				keyFor(7, "double"):   0,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 2,
			wantSkipped:   0,
			wantMessages:  2 + 3, // 7std needs 2, 7dbl needs 3
		},
		{
			name:  "filter by size=9 and mode=standard",
			query: "?size=9&mode=standard",
			counts: map[string]int{
				keyFor(9, "standard"): 1,
			},
			wantStatus:    http.StatusOK,
			wantTriggered: 1,
			wantSkipped:   0,
			wantMessages:  2,
		},
		{
			name:       "DynamoDB error returns 500",
			countErr:   errors.New("dynamodb error"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "SQS publish error returns 500",
			counts: map[string]int{
				keyFor(5, "standard"): 0,
			},
			publishErr: errors.New("sqs error"),
			wantStatus: http.StatusInternalServerError,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			counter := &mockPoolCounter{counts: tt.counts, err: tt.countErr}
			pub := &mockMessagePublisher{err: tt.publishErr}
			h := handler.ReplenishHandler(counter, pub)

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

			// Verify all published messages have correct defaults.
			for i, msg := range pub.published {
				if msg.Pipeline != "iterative" {
					t.Errorf("message[%d].Pipeline = %q, want %q", i, msg.Pipeline, "iterative")
				}
				if msg.Solver != "propagation" {
					t.Errorf("message[%d].Solver = %q, want %q", i, msg.Solver, "propagation")
				}
				if msg.Regions != "bfs" {
					t.Errorf("message[%d].Regions = %q, want %q", i, msg.Regions, "bfs")
				}
				if !msg.Deducible {
					t.Errorf("message[%d].Deducible = false, want true", i)
				}
			}
		})
	}
}
