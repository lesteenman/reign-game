package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/eriksteenman/reign-game/backend/internal/message"
	puzzlestore "github.com/eriksteenman/reign-game/backend/internal/service/puzzlestore"
)

// mockPuzzleStore implements PuzzleStore for testing.
type mockPuzzleStore struct {
	storePuzzleFunc func(ctx context.Context, in *puzzlestore.PuzzleInput) error
}

func (m *mockPuzzleStore) StorePuzzle(ctx context.Context, in *puzzlestore.PuzzleInput) error {
	return m.storePuzzleFunc(ctx, in)
}

func TestHandleSQSEvent(t *testing.T) {
	tests := []struct {
		name     string
		req      message.GenerationRequest
		putErr   error
		wantErr  bool
		wantSize int
		wantMode string
	}{
		{
			name: "generates and stores 7x7 standard puzzle",
			req: message.GenerationRequest{
				Size: 7,
				Mode: "standard",
			},
			wantErr:  false,
			wantSize: 7,
			wantMode: "standard",
		},
		{
			name:    "invalid JSON returns error",
			req:     message.GenerationRequest{}, // we override body below
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "invalid JSON returns error" {
				// Arrange
				var capturedInput *puzzlestore.PuzzleInput
				store := &mockPuzzleStore{
					storePuzzleFunc: func(_ context.Context, in *puzzlestore.PuzzleInput) error {
						capturedInput = in
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
				if capturedInput != nil {
					t.Error("StorePuzzle should not have been called for invalid JSON")
				}
				return
			}

			// Arrange
			var capturedInput *puzzlestore.PuzzleInput
			store := &mockPuzzleStore{
				storePuzzleFunc: func(_ context.Context, in *puzzlestore.PuzzleInput) error {
					capturedInput = in
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
			if capturedInput == nil {
				t.Fatal("StorePuzzle was not called")
			}
			if capturedInput.GridSize != tt.wantSize {
				t.Errorf("GridSize = %d, want %d", capturedInput.GridSize, tt.wantSize)
			}
			if capturedInput.Mode != tt.wantMode {
				t.Errorf("Mode = %q, want %q", capturedInput.Mode, tt.wantMode)
			}
			if capturedInput.ID != "test-uuid-001" {
				t.Errorf("ID = %q, want %q", capturedInput.ID, "test-uuid-001")
			}
			if capturedInput.Status != "ready" {
				t.Errorf("Status = %q, want %q", capturedInput.Status, "ready")
			}
			if capturedInput.RegionMap == nil {
				t.Error("RegionMap should not be nil")
			}
			if capturedInput.Solution == nil {
				t.Error("Solution should not be nil")
			}
			if capturedInput.GenerationDurationMs < 0 {
				t.Errorf("GenerationDurationMs = %d, want >= 0", capturedInput.GenerationDurationMs)
			}
			if capturedInput.CreatedAt == "" {
				t.Error("CreatedAt should not be empty")
			}
			// Classification data must be populated.
			if capturedInput.Difficulty == 0 {
				t.Error("Difficulty = 0 (unknown), want a valid tier")
			}
			if capturedInput.MaxTier == 0 {
				t.Error("MaxTier = 0, want >= 1")
			}
			if len(capturedInput.TierCounts) != 5 {
				t.Errorf("TierCounts length = %d, want 5", len(capturedInput.TierCounts))
			}
		})
	}
}
