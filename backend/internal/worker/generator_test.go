package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/eriksteenman/reign-game/backend/internal/queue"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// mockPuzzleStore implements PuzzleStore for testing.
type mockPuzzleStore struct {
	putPuzzleFunc func(ctx context.Context, puzzle *repository.PuzzleRecord) error
}

func (m *mockPuzzleStore) PutPuzzle(ctx context.Context, puzzle *repository.PuzzleRecord) error {
	return m.putPuzzleFunc(ctx, puzzle)
}

func TestHandleSQSEvent(t *testing.T) {
	tests := []struct {
		name     string
		req      queue.GenerationRequest
		putErr   error
		wantErr  bool
		wantSize int
		wantMode string
	}{
		{
			name: "generates and stores 7x7 standard puzzle",
			req: queue.GenerationRequest{
				Size: 7,
				Mode: "standard",
			},
			wantErr:  false,
			wantSize: 7,
			wantMode: "standard",
		},
		{
			name:    "invalid JSON returns error",
			req:     queue.GenerationRequest{}, // we override body below
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "invalid JSON returns error" {
				// Arrange
				var capturedRecord *repository.PuzzleRecord
				store := &mockPuzzleStore{
					putPuzzleFunc: func(_ context.Context, puzzle *repository.PuzzleRecord) error {
						capturedRecord = puzzle
						return nil
					},
				}
				worker := NewGeneratorWorker(store, func() (string, error) {
					return "test-uuid-001", nil
				})

				event := events.SQSEvent{
					Records: []events.SQSMessage{
						{
							MessageId: "msg-1",
							Body:      "not valid json{{{",
						},
					},
				}

				// Act
				err := worker.HandleSQSEvent(context.Background(), event)

				// Assert
				if err == nil {
					t.Fatal("expected error for invalid JSON, got nil")
				}
				if capturedRecord != nil {
					t.Error("PutPuzzle should not have been called for invalid JSON")
				}
				return
			}

			// Arrange
			var capturedRecord *repository.PuzzleRecord
			store := &mockPuzzleStore{
				putPuzzleFunc: func(_ context.Context, puzzle *repository.PuzzleRecord) error {
					capturedRecord = puzzle
					return tt.putErr
				},
			}
			worker := NewGeneratorWorker(store, func() (string, error) {
				return "test-uuid-001", nil
			})

			body, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			event := events.SQSEvent{
				Records: []events.SQSMessage{
					{
						MessageId: "msg-1",
						Body:      string(body),
					},
				},
			}

			// Act
			err = worker.HandleSQSEvent(context.Background(), event)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedRecord == nil {
				t.Fatal("PutPuzzle was not called")
			}
			if capturedRecord.GridSize != tt.wantSize {
				t.Errorf("GridSize = %d, want %d", capturedRecord.GridSize, tt.wantSize)
			}
			if capturedRecord.Mode != tt.wantMode {
				t.Errorf("Mode = %q, want %q", capturedRecord.Mode, tt.wantMode)
			}
			if capturedRecord.ID != "test-uuid-001" {
				t.Errorf("ID = %q, want %q", capturedRecord.ID, "test-uuid-001")
			}
			if capturedRecord.Status != "ready" {
				t.Errorf("Status = %q, want %q", capturedRecord.Status, "ready")
			}
			if capturedRecord.Verdict != "none" {
				t.Errorf("Verdict = %q, want %q", capturedRecord.Verdict, "none")
			}
			if capturedRecord.RegionMap == nil {
				t.Error("RegionMap should not be nil")
			}
			if capturedRecord.Solution == nil {
				t.Error("Solution should not be nil")
			}
			if capturedRecord.GenerationDurationMs < 0 {
				t.Errorf("GenerationDurationMs = %d, want >= 0", capturedRecord.GenerationDurationMs)
			}
			if capturedRecord.CreatedAt == "" {
				t.Error("CreatedAt should not be empty")
			}
			// Classification data must be populated.
			if capturedRecord.Difficulty == 0 {
				t.Error("Difficulty = 0 (unknown), want a valid tier")
			}
			if capturedRecord.MaxTier == 0 {
				t.Error("MaxTier = 0, want >= 1")
			}
			if len(capturedRecord.TierCounts) != 5 {
				t.Errorf("TierCounts length = %d, want 5", len(capturedRecord.TierCounts))
			}
		})
	}
}
