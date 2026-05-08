package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestTryClaimAutoReplenish(t *testing.T) {
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	window := 60 * time.Second

	tests := []struct {
		name        string
		updateErr   error
		wantClaimed bool
		wantErrIs   error
	}{
		{
			name:        "first-fire: no attribute / fresh CONFIG",
			updateErr:   nil,
			wantClaimed: true,
		},
		{
			name:        "stale attribute: update succeeds",
			updateErr:   nil,
			wantClaimed: true,
		},
		{
			name:        "fresh attribute: conditional check fails -> debounced",
			updateErr:   &types.ConditionalCheckFailedException{Message: strPtr("condition")},
			wantClaimed: false,
		},
		{
			name:        "missing CONFIG record: conditional check fails -> debounced",
			updateErr:   &types.ConditionalCheckFailedException{Message: strPtr("condition")},
			wantClaimed: false,
		},
		{
			name:        "transient DDB error propagates",
			updateErr:   errors.New("dynamodb unavailable"),
			wantClaimed: false,
			wantErrIs:   errors.New("dynamodb unavailable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.UpdateItemInput
			mock := &mockDynamoDBClient{
				updateItemFunc: func(_ context.Context, params *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					capturedInput = params
					return &dynamodb.UpdateItemOutput{}, tt.updateErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			claimed, err := repo.TryClaimAutoReplenish(context.Background(), 9, "standard", now, window)

			// Assert
			if tt.wantErrIs != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrIs.Error()) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErrIs.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if claimed != tt.wantClaimed {
				t.Errorf("claimed = %v, want %v", claimed, tt.wantClaimed)
			}
			if capturedInput == nil {
				t.Fatal("UpdateItem was not called")
			}

			// PK / SK address the right CONFIG record.
			pk := capturedInput.Key["PK"].(*types.AttributeValueMemberS).Value
			sk := capturedInput.Key["SK"].(*types.AttributeValueMemberS).Value
			if pk != "CONFIG" {
				t.Errorf("PK = %q, want CONFIG", pk)
			}
			if sk != "9#standard" {
				t.Errorf("SK = %q, want 9#standard", sk)
			}

			// Update + Condition expressions match the spec.
			if got := *capturedInput.UpdateExpression; got != "SET last_auto_replenish_ts = :now" {
				t.Errorf("UpdateExpression = %q", got)
			}
			wantCond := "attribute_exists(PK) AND (attribute_not_exists(last_auto_replenish_ts) OR last_auto_replenish_ts < :cutoff)"
			if got := *capturedInput.ConditionExpression; got != wantCond {
				t.Errorf("ConditionExpression = %q, want %q", got, wantCond)
			}

			// Values are RFC3339Nano-formatted UTC timestamps.
			nowVal := capturedInput.ExpressionAttributeValues[":now"].(*types.AttributeValueMemberS).Value
			cutoffVal := capturedInput.ExpressionAttributeValues[":cutoff"].(*types.AttributeValueMemberS).Value
			if nowVal != now.UTC().Format(time.RFC3339Nano) {
				t.Errorf(":now = %q, want %q", nowVal, now.UTC().Format(time.RFC3339Nano))
			}
			if cutoffVal != now.Add(-window).UTC().Format(time.RFC3339Nano) {
				t.Errorf(":cutoff = %q, want %q", cutoffVal, now.Add(-window).UTC().Format(time.RFC3339Nano))
			}
		})
	}
}
