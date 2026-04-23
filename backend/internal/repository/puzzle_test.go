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
	getItemFunc    func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
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

func (m *mockDynamoDBClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return m.getItemFunc(ctx, params, optFns...)
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
				GridSize:   7,
				Mode:       "standard",
				ID:         "test-uuid-123",
				Status:     "ready",
				Verdict:    "none",
				RegionMap:  [][]int{{0, 0, 1}, {0, 1, 1}, {2, 2, 1}},
				Solution:   [][]bool{{true, false, false}, {false, false, true}, {false, true, false}},
				Difficulty: 2,
				MaxTier:    2,
				TierCounts: []int{0, 3, 1, 0, 0},
				TraceLen:   4,
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
					"difficulty":           &types.AttributeValueMemberN{Value: "2"},
					"maxTier":              &types.AttributeValueMemberN{Value: "2"},
					"tierCounts":           &types.AttributeValueMemberL{Value: []types.AttributeValue{&types.AttributeValueMemberN{Value: "0"}, &types.AttributeValueMemberN{Value: "3"}, &types.AttributeValueMemberN{Value: "1"}, &types.AttributeValueMemberN{Value: "0"}, &types.AttributeValueMemberN{Value: "0"}}},
					"traceLen":             &types.AttributeValueMemberN{Value: "4"},
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
		name        string
		pk          string
		sk          string
		updateErr   error
		wantErr     bool
		wantNotFoun bool
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
		{
			name:        "maps ConditionalCheckFailed to ErrPuzzleNotFound",
			pk:          "7#standard",
			sk:          "puzzle-uuid-missing",
			updateErr:   &types.ConditionalCheckFailedException{Message: strPtr("row not found")},
			wantErr:     true,
			wantNotFoun: true,
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
				if tt.wantNotFoun && !errors.Is(err, ErrPuzzleNotFound) {
					t.Fatalf("error = %v, want ErrPuzzleNotFound", err)
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
			if capturedInput.ConditionExpression == nil || *capturedInput.ConditionExpression != "attribute_exists(PK)" {
				t.Errorf("ConditionExpression = %v, want attribute_exists(PK)", capturedInput.ConditionExpression)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	tests := []struct {
		name        string
		pk          string
		sk          string
		status      string
		updateErr   error
		wantErr     bool
		wantNotFoun bool
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
		{
			name:        "maps ConditionalCheckFailed to ErrPuzzleNotFound",
			pk:          "7#standard",
			sk:          "puzzle-uuid-missing",
			status:      "solved",
			updateErr:   &types.ConditionalCheckFailedException{Message: strPtr("row not found")},
			wantErr:     true,
			wantNotFoun: true,
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
				if tt.wantNotFoun && !errors.Is(err, ErrPuzzleNotFound) {
					t.Fatalf("error = %v, want ErrPuzzleNotFound", err)
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
			if capturedInput.ConditionExpression == nil || *capturedInput.ConditionExpression != "attribute_exists(PK)" {
				t.Errorf("ConditionExpression = %v, want attribute_exists(PK)", capturedInput.ConditionExpression)
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

// helper to build a DynamoDB item map for a config record.
func buildConfigItem(sk string, enabled bool) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK":        &types.AttributeValueMemberS{Value: "CONFIG"},
		"SK":        &types.AttributeValueMemberS{Value: sk},
		"threshold": &types.AttributeValueMemberN{Value: "10"},
		"enabled":   &types.AttributeValueMemberBOOL{Value: enabled},
	}
}

func TestGetAllConfigs(t *testing.T) {
	tests := []struct {
		name       string
		queryItems []map[string]types.AttributeValue
		queryErr   error
		wantErr    bool
		wantCount  int
		wantSizes  []int
		wantModes  []string
	}{
		{
			name:       "returns empty slice when no configs",
			queryItems: []map[string]types.AttributeValue{},
			wantErr:    false,
			wantCount:  0,
		},
		{
			name: "returns multiple configs with parsed Size and Mode",
			queryItems: []map[string]types.AttributeValue{
				buildConfigItem("5#standard", true),
				buildConfigItem("7#double", false),
			},
			wantErr:   false,
			wantCount: 2,
			wantSizes: []int{5, 7},
			wantModes: []string{"standard", "double"},
		},
		{
			name:     "propagates DynamoDB error",
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
			configs, err := repo.GetAllConfigs(context.Background())

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
			if len(configs) != tt.wantCount {
				t.Fatalf("config count = %d, want %d", len(configs), tt.wantCount)
			}
			for i, cfg := range configs {
				if i < len(tt.wantSizes) && cfg.Size != tt.wantSizes[i] {
					t.Errorf("configs[%d].Size = %d, want %d", i, cfg.Size, tt.wantSizes[i])
				}
				if i < len(tt.wantModes) && cfg.Mode != tt.wantModes[i] {
					t.Errorf("configs[%d].Mode = %q, want %q", i, cfg.Mode, tt.wantModes[i])
				}
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name          string
		size          int
		mode          string
		item          map[string]types.AttributeValue
		getErr        error
		wantNil       bool
		wantErr       bool
		wantEnabled   bool
		wantThreshold int
	}{
		{
			name:          "returns config when found",
			size:          5,
			mode:          "standard",
			item:          buildConfigItem("5#standard", true),
			wantNil:       false,
			wantErr:       false,
			wantEnabled:   true,
			wantThreshold: 10,
		},
		{
			name:    "returns nil when not found",
			size:    9,
			mode:    "standard",
			item:    nil,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "propagates DynamoDB error",
			size:    5,
			mode:    "standard",
			getErr:  errors.New("dynamodb get error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mock := &mockDynamoDBClient{
				getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					return &dynamodb.GetItemOutput{Item: tt.item}, nil
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			result, err := repo.GetConfig(context.Background(), tt.size, tt.mode)

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
			if result.Size != tt.size {
				t.Errorf("Size = %d, want %d", result.Size, tt.size)
			}
			if result.Mode != tt.mode {
				t.Errorf("Mode = %q, want %q", result.Mode, tt.mode)
			}
			if result.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", result.Enabled, tt.wantEnabled)
			}
			if result.Threshold != tt.wantThreshold {
				t.Errorf("Threshold = %d, want %d", result.Threshold, tt.wantThreshold)
			}
		})
	}
}

func TestPutConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  ConfigRecord
		putErr  error
		wantErr bool
		wantPK  string
		wantSK  string
	}{
		{
			name: "writes config correctly",
			config: ConfigRecord{
				Size:      5,
				Mode:      "standard",
				Threshold: 10,
				Enabled:   true,
			},
			wantErr: false,
			wantPK:  "CONFIG",
			wantSK:  "5#standard",
		},
		{
			name: "propagates DynamoDB error",
			config: ConfigRecord{
				Size: 7,
				Mode: "double",
			},
			putErr:  errors.New("dynamodb put error"),
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
			err := repo.PutConfig(context.Background(), &tt.config)

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
			pk := capturedInput.Item["PK"].(*types.AttributeValueMemberS).Value
			if pk != tt.wantPK {
				t.Errorf("PK = %q, want %q", pk, tt.wantPK)
			}
			sk := capturedInput.Item["SK"].(*types.AttributeValueMemberS).Value
			if sk != tt.wantSK {
				t.Errorf("SK = %q, want %q", sk, tt.wantSK)
			}
			if capturedInput.ConditionExpression != nil {
				t.Error("PutConfig should not have a condition expression")
			}
		})
	}
}

func TestCreateConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        ConfigRecord
		putErr        error
		wantErr       bool
		wantConflict  bool
		wantCondition string
	}{
		{
			name: "creates config successfully",
			config: ConfigRecord{
				Size:      5,
				Mode:      "standard",
				Enabled:   true,
				Threshold: 3,
			},
			wantErr:       false,
			wantCondition: "attribute_not_exists(PK)",
		},
		{
			name: "returns ConfigAlreadyExistsError on conflict",
			config: ConfigRecord{
				Size: 5,
				Mode: "standard",
			},
			putErr:       &types.ConditionalCheckFailedException{Message: strPtr("conditional check failed")},
			wantErr:      true,
			wantConflict: true,
		},
		{
			name: "propagates other DynamoDB errors",
			config: ConfigRecord{
				Size: 7,
				Mode: "double",
			},
			putErr:       errors.New("dynamodb connection error"),
			wantErr:      true,
			wantConflict: false,
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
			err := repo.CreateConfig(context.Background(), &tt.config)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantConflict {
					var conflictErr *ConfigAlreadyExistsError
					if !errors.As(err, &conflictErr) {
						t.Fatalf("expected ConfigAlreadyExistsError, got %T: %v", err, err)
					}
					if conflictErr.Size != tt.config.Size {
						t.Errorf("conflict Size = %d, want %d", conflictErr.Size, tt.config.Size)
					}
					if conflictErr.Mode != tt.config.Mode {
						t.Errorf("conflict Mode = %q, want %q", conflictErr.Mode, tt.config.Mode)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedInput == nil {
				t.Fatal("PutItem was not called")
			}
			if capturedInput.ConditionExpression == nil || *capturedInput.ConditionExpression != tt.wantCondition {
				got := "<nil>"
				if capturedInput.ConditionExpression != nil {
					got = *capturedInput.ConditionExpression
				}
				t.Errorf("ConditionExpression = %q, want %q", got, tt.wantCondition)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
