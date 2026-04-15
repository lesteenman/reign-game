package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// mockDynamoDBClient implements DynamoDBAPI for testing.
type mockDynamoDBClient struct {
	putItemFunc    func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	queryFunc      func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	updateItemFunc func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

func (m *mockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	return m.putItemFunc(ctx, params, optFns...)
}

func (m *mockDynamoDBClient) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	return m.queryFunc(ctx, params, optFns...)
}

func (m *mockDynamoDBClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return m.updateItemFunc(ctx, params, optFns...)
}

func TestPutPuzzle(t *testing.T) {
	tests := []struct {
		name      string
		puzzle    PuzzleRecord
		putErr    error
		wantErr   bool
		wantPK    string
		wantSK    string
		wantTable string
	}{
		{
			name: "writes puzzle correctly",
			puzzle: PuzzleRecord{
				GridSize:  7,
				Mode:      "standard",
				ID:        "test-uuid-123",
				Status:    "ready",
				Verdict:   "none",
				RegionMap: [][]int{{0, 0, 1}, {0, 1, 1}, {2, 2, 1}},
				Solution:  [][]bool{{true, false, false}, {false, false, true}, {false, true, false}},
				Pipeline:  "iterative",
				Solver:    "propagation",
				Regions:   "bfs",
			},
			putErr:    nil,
			wantErr:   false,
			wantPK:    "7#standard",
			wantSK:    "test-uuid-123",
			wantTable: "puzzle-pool",
		},
		{
			name: "propagates DynamoDB error",
			puzzle: PuzzleRecord{
				GridSize: 5,
				Mode:     "standard",
				ID:       "test-uuid-456",
			},
			putErr:  errors.New("dynamodb connection error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.PutItemInput
			mock := &mockDynamoDBClient{
				putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
					capturedInput = params
					return &dynamodb.PutItemOutput{}, tt.putErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			err := repo.PutPuzzle(context.Background(), &tt.puzzle)

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
				t.Fatal("PutItem was not called")
			}
			if *capturedInput.TableName != tt.wantTable {
				t.Errorf("table name = %q, want %q", *capturedInput.TableName, tt.wantTable)
			}
			pk := capturedInput.Item["PK"].(*types.AttributeValueMemberS).Value
			if pk != tt.wantPK {
				t.Errorf("PK = %q, want %q", pk, tt.wantPK)
			}
			sk := capturedInput.Item["SK"].(*types.AttributeValueMemberS).Value
			if sk != tt.wantSK {
				t.Errorf("SK = %q, want %q", sk, tt.wantSK)
			}
			status := capturedInput.Item["status"].(*types.AttributeValueMemberS).Value
			if status != "ready" {
				t.Errorf("status = %q, want %q", status, "ready")
			}
		})
	}
}

func TestNextReady(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		mode       string
		queryItems []map[string]types.AttributeValue
		queryErr   error
		wantNil    bool
		wantErr    bool
		wantID     string
	}{
		{
			name: "returns puzzle when one is ready",
			size: 7,
			mode: "standard",
			queryItems: []map[string]types.AttributeValue{
				{
					"PK":                   &types.AttributeValueMemberS{Value: "7#standard"},
					"SK":                   &types.AttributeValueMemberS{Value: "puzzle-uuid-1"},
					"status":               &types.AttributeValueMemberS{Value: "ready"},
					"verdict":              &types.AttributeValueMemberS{Value: "none"},
					"regionMap":            &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberN{Value: "0"}}}}},
					"solution":             &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberBOOL{Value: true}}}}},
					"pipeline":             &types.AttributeValueMemberS{Value: "iterative"},
					"solver":               &types.AttributeValueMemberS{Value: "propagation"},
					"regions":              &types.AttributeValueMemberS{Value: "bfs"},
					"regionVariance":       &types.AttributeValueMemberN{Value: "0"},
					"deducible":            &types.AttributeValueMemberBOOL{Value: true},
					"concurrency":          &types.AttributeValueMemberN{Value: "1"},
					"generationDurationMs": &types.AttributeValueMemberN{Value: "4200"},
					"createdAt":            &types.AttributeValueMemberS{Value: "2026-04-15T10:30:00Z"},
					"servedAt":             &types.AttributeValueMemberS{Value: ""},
				},
			},
			wantNil: false,
			wantErr: false,
			wantID:  "puzzle-uuid-1",
		},
		{
			name:       "returns nil when no puzzles are ready",
			size:       5,
			mode:       "standard",
			queryItems: []map[string]types.AttributeValue{},
			wantNil:    true,
			wantErr:    false,
		},
		{
			name:     "propagates DynamoDB error",
			size:     7,
			mode:     "standard",
			queryErr: errors.New("dynamodb query error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mock := &mockDynamoDBClient{
				queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
					if tt.queryErr != nil {
						return nil, tt.queryErr
					}
					return &dynamodb.QueryOutput{
						Items: tt.queryItems,
						Count: int32(len(tt.queryItems)),
					}, nil
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			result, err := repo.NextReady(context.Background(), tt.size, tt.mode)

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
			if tt.wantNil {
				if result != nil {
					t.Fatalf("expected nil result, got %+v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result, got nil")
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
		})
	}
}

func TestMarkServed(t *testing.T) {
	tests := []struct {
		name      string
		pk        string
		sk        string
		updateErr error
		wantErr   bool
	}{
		{
			name:    "updates status and servedAt",
			pk:      "7#standard",
			sk:      "puzzle-uuid-1",
			wantErr: false,
		},
		{
			name:      "propagates DynamoDB error",
			pk:        "7#standard",
			sk:        "puzzle-uuid-1",
			updateErr: errors.New("dynamodb update error"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.UpdateItemInput
			mock := &mockDynamoDBClient{
				updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					capturedInput = params
					return &dynamodb.UpdateItemOutput{}, tt.updateErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			err := repo.MarkServed(context.Background(), tt.pk, tt.sk)

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
				t.Fatal("UpdateItem was not called")
			}
			pk := capturedInput.Key["PK"].(*types.AttributeValueMemberS).Value
			if pk != tt.pk {
				t.Errorf("PK = %q, want %q", pk, tt.pk)
			}
			sk := capturedInput.Key["SK"].(*types.AttributeValueMemberS).Value
			if sk != tt.sk {
				t.Errorf("SK = %q, want %q", sk, tt.sk)
			}
			// Verify the update expression sets both status and servedAt.
			if capturedInput.UpdateExpression == nil {
				t.Fatal("UpdateExpression is nil")
			}
			statusVal := capturedInput.ExpressionAttributeValues[":status"].(*types.AttributeValueMemberS).Value
			if statusVal != "served" {
				t.Errorf("status value = %q, want %q", statusVal, "served")
			}
			servedAtVal := capturedInput.ExpressionAttributeValues[":servedAt"].(*types.AttributeValueMemberS).Value
			if servedAtVal == "" {
				t.Error("servedAt value should not be empty")
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	tests := []struct {
		name      string
		pk        string
		sk        string
		status    string
		updateErr error
		wantErr   bool
	}{
		{
			name:    "updates status to solved",
			pk:      "7#standard",
			sk:      "puzzle-uuid-1",
			status:  "solved",
			wantErr: false,
		},
		{
			name:    "updates status to skipped",
			pk:      "9#double",
			sk:      "puzzle-uuid-2",
			status:  "skipped",
			wantErr: false,
		},
		{
			name:      "propagates DynamoDB error",
			pk:        "7#standard",
			sk:        "puzzle-uuid-1",
			status:    "solved",
			updateErr: errors.New("dynamodb update error"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.UpdateItemInput
			mock := &mockDynamoDBClient{
				updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					capturedInput = params
					return &dynamodb.UpdateItemOutput{}, tt.updateErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			err := repo.UpdateStatus(context.Background(), tt.pk, tt.sk, tt.status)

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
				t.Fatal("UpdateItem was not called")
			}
			statusVal := capturedInput.ExpressionAttributeValues[":status"].(*types.AttributeValueMemberS).Value
			if statusVal != tt.status {
				t.Errorf("status value = %q, want %q", statusVal, tt.status)
			}
		})
	}
}

func TestCountReady(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		mode     string
		count    int32
		queryErr error
		wantErr  bool
		want     int
	}{
		{
			name:    "returns correct count",
			size:    7,
			mode:    "standard",
			count:   3,
			wantErr: false,
			want:    3,
		},
		{
			name:    "returns zero when no ready puzzles",
			size:    5,
			mode:    "standard",
			count:   0,
			wantErr: false,
			want:    0,
		},
		{
			name:     "propagates DynamoDB error",
			size:     7,
			mode:     "standard",
			queryErr: errors.New("dynamodb query error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mock := &mockDynamoDBClient{
				queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
					if tt.queryErr != nil {
						return nil, tt.queryErr
					}
					return &dynamodb.QueryOutput{
						Count: tt.count,
					}, nil
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			count, err := repo.CountReady(context.Background(), tt.size, tt.mode)

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
			if count != tt.want {
				t.Errorf("count = %d, want %d", count, tt.want)
			}
		})
	}
}
