package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestPutCandidateIfAbsent(t *testing.T) {
	tests := []struct {
		name             string
		puzzleID         string
		sourcePartition  string
		putErr           error
		wantErr          bool
		wantAlreadyExist bool
		wantPK           string
		wantSK           string
		wantCondition    string
	}{
		{
			name:            "writes when slot empty",
			puzzleID:        "puzzle-uuid-1",
			sourcePartition: "9#standard",
			putErr:          nil,
			wantErr:         false,
			wantPK:          dailyCandidatePK,
			wantSK:          dailySingletonSK,
			wantCondition:   "attribute_not_exists(PK)",
		},
		{
			name:             "returns ErrCandidateAlreadyExists when slot filled",
			puzzleID:         "puzzle-uuid-2",
			sourcePartition:  "9#standard",
			putErr:           &types.ConditionalCheckFailedException{Message: strPtr("slot already filled")},
			wantErr:          true,
			wantAlreadyExist: true,
		},
		{
			name:            "propagates non-conditional DynamoDB error",
			puzzleID:        "puzzle-uuid-3",
			sourcePartition: "9#standard",
			putErr:          errors.New("dynamodb network error"),
			wantErr:         true,
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
			err := repo.PutCandidateIfAbsent(context.Background(), tt.puzzleID, tt.sourcePartition)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantAlreadyExist && !errors.Is(err, ErrCandidateAlreadyExists) {
					t.Fatalf("error = %v, want ErrCandidateAlreadyExists", err)
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
			puzzleID := capturedInput.Item["puzzleId"].(*types.AttributeValueMemberS).Value
			if puzzleID != tt.puzzleID {
				t.Errorf("puzzleId = %q, want %q", puzzleID, tt.puzzleID)
			}
			sourcePart := capturedInput.Item["sourcePartition"].(*types.AttributeValueMemberS).Value
			if sourcePart != tt.sourcePartition {
				t.Errorf("sourcePartition = %q, want %q", sourcePart, tt.sourcePartition)
			}
			queuedAt := capturedInput.Item["queuedAt"].(*types.AttributeValueMemberS).Value
			if queuedAt == "" {
				t.Error("queuedAt should be stamped")
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

func TestGetCandidate(t *testing.T) {
	tests := []struct {
		name                string
		item                map[string]types.AttributeValue
		getErr              error
		wantNil             bool
		wantErr             bool
		wantPuzzleID        string
		wantSourcePartition string
	}{
		{
			name:    "returns nil when absent",
			item:    nil,
			wantNil: true,
		},
		{
			name: "returns the candidate row when present",
			item: map[string]types.AttributeValue{
				"PK":              &types.AttributeValueMemberS{Value: dailyCandidatePK},
				"SK":              &types.AttributeValueMemberS{Value: dailySingletonSK},
				"puzzleId":        &types.AttributeValueMemberS{Value: "candidate-uuid-1"},
				"queuedAt":        &types.AttributeValueMemberS{Value: "2026-04-30T10:00:00Z"},
				"sourcePartition": &types.AttributeValueMemberS{Value: "9#standard"},
			},
			wantPuzzleID:        "candidate-uuid-1",
			wantSourcePartition: "9#standard",
		},
		{
			name:    "propagates DynamoDB error",
			getErr:  errors.New("dynamodb get error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.GetItemInput
			mock := &mockDynamoDBClient{
				getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					capturedInput = params
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					return &dynamodb.GetItemOutput{Item: tt.item}, nil
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			result, err := repo.GetCandidate(context.Background())

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
			if result.PuzzleID != tt.wantPuzzleID {
				t.Errorf("PuzzleID = %q, want %q", result.PuzzleID, tt.wantPuzzleID)
			}
			if result.SourcePartition != tt.wantSourcePartition {
				t.Errorf("SourcePartition = %q, want %q", result.SourcePartition, tt.wantSourcePartition)
			}
			if capturedInput == nil {
				t.Fatal("GetItem was not called")
			}
			pk := capturedInput.Key["PK"].(*types.AttributeValueMemberS).Value
			if pk != dailyCandidatePK {
				t.Errorf("PK = %q, want %q", pk, dailyCandidatePK)
			}
			sk := capturedInput.Key["SK"].(*types.AttributeValueMemberS).Value
			if sk != dailySingletonSK {
				t.Errorf("SK = %q, want %q", sk, dailySingletonSK)
			}
		})
	}
}

func TestDeleteCandidate(t *testing.T) {
	tests := []struct {
		name      string
		deleteErr error
		wantErr   bool
	}{
		{
			name: "succeeds when present",
		},
		{
			name: "succeeds when absent (idempotent no-op)",
		},
		{
			name:      "propagates DynamoDB error",
			deleteErr: errors.New("dynamodb delete error"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.DeleteItemInput
			mock := &mockDynamoDBClient{
				deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
					capturedInput = params
					return &dynamodb.DeleteItemOutput{}, tt.deleteErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			err := repo.DeleteCandidate(context.Background())

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
				t.Fatal("DeleteItem was not called")
			}
			pk := capturedInput.Key["PK"].(*types.AttributeValueMemberS).Value
			if pk != dailyCandidatePK {
				t.Errorf("PK = %q, want %q", pk, dailyCandidatePK)
			}
			sk := capturedInput.Key["SK"].(*types.AttributeValueMemberS).Value
			if sk != dailySingletonSK {
				t.Errorf("SK = %q, want %q", sk, dailySingletonSK)
			}
			if capturedInput.ConditionExpression != nil {
				t.Errorf("DeleteCandidate should be unconditional (idempotent); got ConditionExpression=%q", *capturedInput.ConditionExpression)
			}
		})
	}
}

func TestFinalizeSchedule(t *testing.T) {
	tests := []struct {
		name             string
		date             string
		puzzleID         string
		sourcePartition  string
		putErr           error
		wantErr          bool
		wantAlreadyFinal bool
		wantPK           string
		wantSK           string
		wantCondition    string
	}{
		{
			name:            "writes when no schedule for date",
			date:            "2026-04-30",
			puzzleID:        "puzzle-uuid-1",
			sourcePartition: "9#standard",
			putErr:          nil,
			wantErr:         false,
			wantPK:          "DAILY#2026-04-30",
			wantSK:          dailySingletonSK,
			wantCondition:   "attribute_not_exists(PK)",
		},
		{
			name:             "returns ErrScheduleAlreadyFinalized when date already has a row",
			date:             "2026-04-30",
			puzzleID:         "puzzle-uuid-2",
			sourcePartition:  "9#standard",
			putErr:           &types.ConditionalCheckFailedException{Message: strPtr("schedule already finalized")},
			wantErr:          true,
			wantAlreadyFinal: true,
		},
		{
			name:            "propagates non-conditional DynamoDB error",
			date:            "2026-04-30",
			puzzleID:        "puzzle-uuid-3",
			sourcePartition: "9#standard",
			putErr:          errors.New("dynamodb network error"),
			wantErr:         true,
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
			err := repo.FinalizeSchedule(context.Background(), tt.date, tt.puzzleID, tt.sourcePartition)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantAlreadyFinal && !errors.Is(err, ErrScheduleAlreadyFinalized) {
					t.Fatalf("error = %v, want ErrScheduleAlreadyFinalized", err)
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
			puzzleID := capturedInput.Item["puzzleId"].(*types.AttributeValueMemberS).Value
			if puzzleID != tt.puzzleID {
				t.Errorf("puzzleId = %q, want %q", puzzleID, tt.puzzleID)
			}
			sourcePart := capturedInput.Item["sourcePartition"].(*types.AttributeValueMemberS).Value
			if sourcePart != tt.sourcePartition {
				t.Errorf("sourcePartition = %q, want %q", sourcePart, tt.sourcePartition)
			}
			assignedAt := capturedInput.Item["assignedAt"].(*types.AttributeValueMemberS).Value
			if assignedAt == "" {
				t.Error("assignedAt should be stamped")
			}
			counters, ok := capturedInput.Item["counters"].(*types.AttributeValueMemberM)
			if !ok {
				t.Fatal("counters should be a map attribute")
			}
			started := counters.Value["started"].(*types.AttributeValueMemberN).Value
			if started != "0" {
				t.Errorf("counters.started = %q, want 0", started)
			}
			solved := counters.Value["solved"].(*types.AttributeValueMemberN).Value
			if solved != "0" {
				t.Errorf("counters.solved = %q, want 0", solved)
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

func TestGetSchedule(t *testing.T) {
	tests := []struct {
		name                string
		date                string
		item                map[string]types.AttributeValue
		getErr              error
		wantNil             bool
		wantErr             bool
		wantPuzzleID        string
		wantSourcePartition string
		wantStarted         int64
		wantSolved          int64
	}{
		{
			name:    "returns nil when no schedule for date",
			date:    "2026-04-30",
			item:    nil,
			wantNil: true,
		},
		{
			name: "returns ScheduleRecord when present",
			date: "2026-04-30",
			item: map[string]types.AttributeValue{
				"PK":              &types.AttributeValueMemberS{Value: "DAILY#2026-04-30"},
				"SK":              &types.AttributeValueMemberS{Value: dailySingletonSK},
				"puzzleId":        &types.AttributeValueMemberS{Value: "schedule-uuid-1"},
				"assignedAt":      &types.AttributeValueMemberS{Value: "2026-04-30T00:00:01Z"},
				"sourcePartition": &types.AttributeValueMemberS{Value: "9#standard"},
				"counters": &types.AttributeValueMemberM{
					Value: map[string]types.AttributeValue{
						"started": &types.AttributeValueMemberN{Value: "42"},
						"solved":  &types.AttributeValueMemberN{Value: "13"},
					},
				},
			},
			wantPuzzleID:        "schedule-uuid-1",
			wantSourcePartition: "9#standard",
			wantStarted:         42,
			wantSolved:          13,
		},
		{
			name:    "propagates DynamoDB error",
			date:    "2026-04-30",
			getErr:  errors.New("dynamodb get error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.GetItemInput
			mock := &mockDynamoDBClient{
				getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					capturedInput = params
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					return &dynamodb.GetItemOutput{Item: tt.item}, nil
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			result, err := repo.GetSchedule(context.Background(), tt.date)

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
			if result.Date != tt.date {
				t.Errorf("Date = %q, want %q", result.Date, tt.date)
			}
			if result.PuzzleID != tt.wantPuzzleID {
				t.Errorf("PuzzleID = %q, want %q", result.PuzzleID, tt.wantPuzzleID)
			}
			if result.SourcePartition != tt.wantSourcePartition {
				t.Errorf("SourcePartition = %q, want %q", result.SourcePartition, tt.wantSourcePartition)
			}
			if result.Counters.Started != tt.wantStarted {
				t.Errorf("Counters.Started = %d, want %d", result.Counters.Started, tt.wantStarted)
			}
			if result.Counters.Solved != tt.wantSolved {
				t.Errorf("Counters.Solved = %d, want %d", result.Counters.Solved, tt.wantSolved)
			}
			if capturedInput == nil {
				t.Fatal("GetItem was not called")
			}
			pk := capturedInput.Key["PK"].(*types.AttributeValueMemberS).Value
			if pk != "DAILY#"+tt.date {
				t.Errorf("PK = %q, want %q", pk, "DAILY#"+tt.date)
			}
			sk := capturedInput.Key["SK"].(*types.AttributeValueMemberS).Value
			if sk != dailySingletonSK {
				t.Errorf("SK = %q, want %q", sk, dailySingletonSK)
			}
		})
	}
}

func TestIncrementScheduleCounter(t *testing.T) {
	tests := []struct {
		name      string
		date      string
		field     string
		delta     int64
		updateErr error
		wantErr   bool
		// skipMockExpectation = true when validation rejects the call before
		// the SDK runs (the mock should not be invoked).
		skipMockExpectation bool
	}{
		{
			name:    "increments started from 0 to 1",
			date:    "2026-04-30",
			field:   ScheduleCounterStarted,
			delta:   1,
			wantErr: false,
		},
		{
			name:    "increments solved from 0 to 1",
			date:    "2026-04-30",
			field:   ScheduleCounterSolved,
			delta:   1,
			wantErr: false,
		},
		{
			name:                "rejects invalid field name",
			date:                "2026-04-30",
			field:               "bogus",
			delta:               1,
			wantErr:             true,
			skipMockExpectation: true,
		},
		{
			name:      "propagates DynamoDB error",
			date:      "2026-04-30",
			field:     ScheduleCounterStarted,
			delta:     1,
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
			err := repo.IncrementScheduleCounter(context.Background(), tt.date, tt.field, tt.delta)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.skipMockExpectation && capturedInput != nil {
					t.Errorf("UpdateItem should not have been called for invalid field; got input=%+v", capturedInput)
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
			if pk != "DAILY#"+tt.date {
				t.Errorf("PK = %q, want %q", pk, "DAILY#"+tt.date)
			}
			sk := capturedInput.Key["SK"].(*types.AttributeValueMemberS).Value
			if sk != dailySingletonSK {
				t.Errorf("SK = %q, want %q", sk, dailySingletonSK)
			}
			if capturedInput.ExpressionAttributeNames["#field"] != tt.field {
				t.Errorf("#field = %q, want %q", capturedInput.ExpressionAttributeNames["#field"], tt.field)
			}
			if capturedInput.ExpressionAttributeNames["#counters"] != "counters" {
				t.Errorf("#counters = %q, want counters", capturedInput.ExpressionAttributeNames["#counters"])
			}
			deltaVal := capturedInput.ExpressionAttributeValues[":delta"].(*types.AttributeValueMemberN).Value
			if deltaVal != "1" {
				t.Errorf(":delta = %q, want 1", deltaVal)
			}
		})
	}
}

func TestListApprovedPool(t *testing.T) {
	// poolItem builds a puzzle-pool DDB item for the 9#standard partition.
	// The repository delegates pool filtering to DynamoDB's
	// FilterExpression, so the mock returns the *post-filter* slice the
	// real DDB would emit; the test asserts both the FilterExpression
	// shape (so DDB filters correctly in prod) and the post-unmarshal
	// records the repository returns.
	poolItem := func(id string, up, down int, lastDailyDate string) map[string]types.AttributeValue {
		item := map[string]types.AttributeValue{
			"PK":     &types.AttributeValueMemberS{Value: "9#standard"},
			"SK":     &types.AttributeValueMemberS{Value: id},
			"status": &types.AttributeValueMemberS{Value: "ready"},
			"verdictSummary": &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"up":            &types.AttributeValueMemberN{Value: itoa(up)},
					"down":          &types.AttributeValueMemberN{Value: itoa(down)},
					"lastUpdatedAt": &types.AttributeValueMemberS{Value: "2026-04-26T10:00:00Z"},
				},
			},
		}
		if lastDailyDate != "" {
			item["lastDailyDate"] = &types.AttributeValueMemberS{Value: lastDailyDate}
		}
		return item
	}

	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	wantCutoff := now.AddDate(0, 0, -DailyRecycleWindowDays).Format("2006-01-02") // "2026-04-16"

	tests := []struct {
		name                   string
		excludeRecentlyDailied bool
		// preFilteredItems is what DDB would return after applying the
		// repository's FilterExpression — i.e., only the rows that should
		// survive. The fixture commentary lists *all* rows the partition
		// holds; rows that the FilterExpression would reject are simply
		// omitted from this slice (mirroring real DDB behaviour).
		preFilteredItems  []map[string]types.AttributeValue
		queryErr          error
		wantErr           bool
		wantIDs           []string
		wantCutoffPresent bool
	}{
		{
			// Partition holds:
			//   A: up=1 down=0   → FilterExpression accepts → returned
			//   B: up=2 down=0   → FilterExpression accepts → returned
			//   C: up=0 down=0   → FilterExpression rejects → omitted
			//   D: up=2 down=1   → FilterExpression rejects → omitted
			name:                   "returns approved puzzles (up >= 1 && down == 0)",
			excludeRecentlyDailied: false,
			preFilteredItems: []map[string]types.AttributeValue{
				poolItem("puzzle-A", 1, 0, ""),
				poolItem("puzzle-B", 2, 0, ""),
			},
			wantIDs: []string{"puzzle-A", "puzzle-B"},
		},
		{
			// Partition holds:
			//   E: up=1 down=0 lastDailyDate=2026-04-25 (5 days ago) → recycle filter rejects
			//   F: up=1 down=0 lastDailyDate=2026-04-10 (20 days ago) → recycle filter accepts
			//   G: up=1 down=0 (no lastDailyDate) → recycle filter accepts
			name:                   "with excludeRecentlyDailied=true filters dailied within window",
			excludeRecentlyDailied: true,
			preFilteredItems: []map[string]types.AttributeValue{
				poolItem("puzzle-F", 1, 0, "2026-04-10"),
				poolItem("puzzle-G", 1, 0, ""),
			},
			wantIDs:           []string{"puzzle-F", "puzzle-G"},
			wantCutoffPresent: true,
		},
		{
			// Same fixture E (within window) but no recycle filter:
			// FilterExpression has no lastDailyDate clause, so E survives.
			name:                   "with excludeRecentlyDailied=false includes recently-dailied",
			excludeRecentlyDailied: false,
			preFilteredItems: []map[string]types.AttributeValue{
				poolItem("puzzle-E", 1, 0, "2026-04-25"),
			},
			wantIDs: []string{"puzzle-E"},
		},
		{
			name:                   "propagates DDB Query error",
			excludeRecentlyDailied: false,
			queryErr:               errors.New("dynamodb query failed"),
			wantErr:                true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.QueryInput
			mock := &mockDynamoDBClient{
				queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
					capturedInput = params
					if tt.queryErr != nil {
						return nil, tt.queryErr
					}
					return &dynamodb.QueryOutput{Items: tt.preFilteredItems}, nil
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			got, err := repo.ListApprovedPool(context.Background(), 9, "standard", tt.excludeRecentlyDailied, now)

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
				t.Fatal("Query was not called")
			}
			pkVal := capturedInput.ExpressionAttributeValues[":pk"].(*types.AttributeValueMemberS).Value
			if pkVal != "9#standard" {
				t.Errorf(":pk = %q, want 9#standard", pkVal)
			}
			oneVal := capturedInput.ExpressionAttributeValues[":one"].(*types.AttributeValueMemberN).Value
			if oneVal != "1" {
				t.Errorf(":one = %q, want 1", oneVal)
			}
			zeroVal := capturedInput.ExpressionAttributeValues[":zero"].(*types.AttributeValueMemberN).Value
			if zeroVal != "0" {
				t.Errorf(":zero = %q, want 0", zeroVal)
			}
			filter := ""
			if capturedInput.FilterExpression != nil {
				filter = *capturedInput.FilterExpression
			}
			if !strings.Contains(filter, "verdictSummary.up >= :one") {
				t.Errorf("FilterExpression missing up >= :one clause; got %q", filter)
			}
			if !strings.Contains(filter, "verdictSummary.down = :zero") {
				t.Errorf("FilterExpression missing down = :zero clause; got %q", filter)
			}
			if tt.wantCutoffPresent {
				cutoffAttr, ok := capturedInput.ExpressionAttributeValues[":cutoff"].(*types.AttributeValueMemberS)
				if !ok {
					t.Fatalf(":cutoff missing or wrong type when excludeRecentlyDailied=true")
				}
				if cutoffAttr.Value != wantCutoff {
					t.Errorf(":cutoff = %q, want %q", cutoffAttr.Value, wantCutoff)
				}
				if !strings.Contains(filter, "attribute_not_exists(lastDailyDate)") || !strings.Contains(filter, "lastDailyDate < :cutoff") {
					t.Errorf("FilterExpression missing recycle-window clause; got %q", filter)
				}
			} else {
				if _, present := capturedInput.ExpressionAttributeValues[":cutoff"]; present {
					t.Errorf(":cutoff should not be present when excludeRecentlyDailied=false")
				}
				if strings.Contains(filter, "lastDailyDate") {
					t.Errorf("FilterExpression should not reference lastDailyDate when excludeRecentlyDailied=false; got %q", filter)
				}
			}
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("len(got) = %d, want %d", len(got), len(tt.wantIDs))
			}
			gotIDs := make(map[string]bool, len(got))
			for _, p := range got {
				gotIDs[p.ID] = true
				if p.GridSize != 9 || p.Mode != "standard" {
					t.Errorf("identity not stamped on row: got (size=%d, mode=%q); want (9, standard)", p.GridSize, p.Mode)
				}
			}
			for _, want := range tt.wantIDs {
				if !gotIDs[want] {
					t.Errorf("expected puzzle %q in result; got IDs %v", want, gotIDs)
				}
			}
		})
	}
}

func TestMarkPuzzleAsDailyOn(t *testing.T) {
	// markPuzzleItem builds a PuzzleRecord row for the disambiguation
	// GetItem the repository falls back to on a conditional failure.
	// `lastDailyDate` is omitted when empty so the absent-field branch
	// of the condition can be exercised.
	markPuzzleItem := func(id, lastDailyDate string) map[string]types.AttributeValue {
		item := map[string]types.AttributeValue{
			"PK":     &types.AttributeValueMemberS{Value: "9#standard"},
			"SK":     &types.AttributeValueMemberS{Value: id},
			"status": &types.AttributeValueMemberS{Value: "ready"},
		}
		if lastDailyDate != "" {
			item["lastDailyDate"] = &types.AttributeValueMemberS{Value: lastDailyDate}
		}
		return item
	}

	tests := []struct {
		name string
		// updateErr is what UpdateItem returns. Conditional failures
		// trigger the disambiguation GetItem leg.
		updateErr error
		// fallbackItem is what the disambiguation GetItem returns. Nil
		// item simulates the missing-row case (→ ErrPuzzleNotFound);
		// a populated item simulates the newer-date silent-no-op case.
		fallbackItem map[string]types.AttributeValue
		wantErr      bool
		wantNotFound bool
		// wantUpdateInvoked asserts UpdateItem was called (always true
		// here — every sub-case routes through UpdateItem first).
		wantUpdateInvoked bool
	}{
		{
			name:              "sets lastDailyDate when field absent",
			updateErr:         nil,
			wantErr:           false,
			wantUpdateInvoked: true,
		},
		{
			name:              "idempotent on same date",
			updateErr:         nil,
			wantErr:           false,
			wantUpdateInvoked: true,
		},
		{
			name:              "refuses to overwrite a NEWER date with an older one",
			updateErr:         &types.ConditionalCheckFailedException{Message: strPtr("newer date present")},
			fallbackItem:      markPuzzleItem("puzzle-uuid-1", "2026-05-10"),
			wantErr:           false,
			wantUpdateInvoked: true,
		},
		{
			name:              "returns ErrPuzzleNotFound when row is missing",
			updateErr:         &types.ConditionalCheckFailedException{Message: strPtr("row missing")},
			fallbackItem:      nil,
			wantErr:           true,
			wantNotFound:      true,
			wantUpdateInvoked: true,
		},
		{
			name:              "propagates non-conditional error",
			updateErr:         errors.New("dynamodb network error"),
			wantErr:           true,
			wantUpdateInvoked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedUpdate *dynamodb.UpdateItemInput
			mock := &mockDynamoDBClient{
				updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					capturedUpdate = params
					return &dynamodb.UpdateItemOutput{}, tt.updateErr
				},
				getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					return &dynamodb.GetItemOutput{Item: tt.fallbackItem}, nil
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			err := repo.MarkPuzzleAsDailyOn(context.Background(), 9, "standard", "puzzle-uuid-1", "2026-05-02")

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantNotFound && !errors.Is(err, ErrPuzzleNotFound) {
					t.Fatalf("error = %v, want ErrPuzzleNotFound", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantUpdateInvoked && capturedUpdate == nil {
				t.Fatal("UpdateItem was not called")
			}
			if capturedUpdate != nil {
				pk := capturedUpdate.Key["PK"].(*types.AttributeValueMemberS).Value
				if pk != "9#standard" {
					t.Errorf("PK = %q, want 9#standard", pk)
				}
				sk := capturedUpdate.Key["SK"].(*types.AttributeValueMemberS).Value
				if sk != "puzzle-uuid-1" {
					t.Errorf("SK = %q, want puzzle-uuid-1", sk)
				}
				cond := ""
				if capturedUpdate.ConditionExpression != nil {
					cond = *capturedUpdate.ConditionExpression
				}
				if !strings.Contains(cond, "attribute_exists(PK)") {
					t.Errorf("ConditionExpression missing attribute_exists(PK); got %q", cond)
				}
				if !strings.Contains(cond, "attribute_not_exists(lastDailyDate)") {
					t.Errorf("ConditionExpression missing attribute_not_exists(lastDailyDate); got %q", cond)
				}
				if !strings.Contains(cond, "lastDailyDate <= :date") {
					t.Errorf("ConditionExpression missing lastDailyDate <= :date; got %q", cond)
				}
				dateVal := capturedUpdate.ExpressionAttributeValues[":date"].(*types.AttributeValueMemberS).Value
				if dateVal != "2026-05-02" {
					t.Errorf(":date = %q, want 2026-05-02", dateVal)
				}
				update := ""
				if capturedUpdate.UpdateExpression != nil {
					update = *capturedUpdate.UpdateExpression
				}
				if !strings.Contains(update, "SET lastDailyDate = :date") {
					t.Errorf("UpdateExpression should set lastDailyDate; got %q", update)
				}
			}
		})
	}
}

func TestPuzzleRecordLastDailyDate(t *testing.T) {
	t.Run("with value, marshals and unmarshals", func(t *testing.T) {
		// Arrange
		original := PuzzleRecord{
			ID:            "puzzle-uuid-1",
			GridSize:      9,
			Mode:          "standard",
			Status:        "ready",
			LastDailyDate: "2026-05-02",
		}

		// Act
		marshalled, err := attributevalue.MarshalMap(original)
		if err != nil {
			t.Fatalf("MarshalMap: %v", err)
		}
		var roundTripped PuzzleRecord
		if err := attributevalue.UnmarshalMap(marshalled, &roundTripped); err != nil {
			t.Fatalf("UnmarshalMap: %v", err)
		}

		// Assert
		attr, present := marshalled["lastDailyDate"]
		if !present {
			t.Fatal("marshalled map missing lastDailyDate key")
		}
		s, ok := attr.(*types.AttributeValueMemberS)
		if !ok {
			t.Fatalf("lastDailyDate = %T, want AttributeValueMemberS", attr)
		}
		if s.Value != "2026-05-02" {
			t.Errorf("marshalled lastDailyDate = %q, want 2026-05-02", s.Value)
		}
		if roundTripped.LastDailyDate != "2026-05-02" {
			t.Errorf("round-tripped LastDailyDate = %q, want 2026-05-02", roundTripped.LastDailyDate)
		}
	})

	t.Run("absent, omitempty keeps row clean", func(t *testing.T) {
		// Arrange
		original := PuzzleRecord{
			ID:       "puzzle-uuid-2",
			GridSize: 9,
			Mode:     "standard",
			Status:   "ready",
		}

		// Act
		marshalled, err := attributevalue.MarshalMap(original)
		if err != nil {
			t.Fatalf("MarshalMap: %v", err)
		}

		// Assert
		if _, present := marshalled["lastDailyDate"]; present {
			t.Errorf("marshalled map should omit lastDailyDate when empty; got %v", marshalled["lastDailyDate"])
		}
	})
}

func TestPutPlayStartedIfAbsent(t *testing.T) {
	assignedAt := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		playerID         string
		date             string
		puzzleID         string
		putErr           error
		wantErr          bool
		wantAlreadyExist bool
		wantPK           string
		wantSK           string
		wantCondition    string
	}{
		{
			name:          "writes when (playerId, date) absent",
			playerID:      "user_abc",
			date:          "2026-04-30",
			puzzleID:      "puzzle-uuid-1",
			putErr:        nil,
			wantErr:       false,
			wantPK:        "PLAY#user_abc",
			wantSK:        "DAILY#2026-04-30",
			wantCondition: "attribute_not_exists(PK)",
		},
		{
			name:             "returns ErrPlayAlreadyExists when row present",
			playerID:         "user_abc",
			date:             "2026-04-30",
			puzzleID:         "puzzle-uuid-1",
			putErr:           &types.ConditionalCheckFailedException{Message: strPtr("play row already exists")},
			wantErr:          true,
			wantAlreadyExist: true,
		},
		{
			name:     "propagates non-conditional error",
			playerID: "user_abc",
			date:     "2026-04-30",
			puzzleID: "puzzle-uuid-1",
			putErr:   errors.New("dynamodb network error"),
			wantErr:  true,
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
			err := repo.PutPlayStartedIfAbsent(context.Background(), tt.playerID, tt.date, tt.puzzleID, assignedAt)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantAlreadyExist && !errors.Is(err, ErrPlayAlreadyExists) {
					t.Fatalf("error = %v, want ErrPlayAlreadyExists", err)
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
			outcome := capturedInput.Item["outcome"].(*types.AttributeValueMemberS).Value
			if outcome != PlayOutcomeStarted {
				t.Errorf("outcome = %q, want %q", outcome, PlayOutcomeStarted)
			}
			puzzleID := capturedInput.Item["puzzleId"].(*types.AttributeValueMemberS).Value
			if puzzleID != tt.puzzleID {
				t.Errorf("puzzleId = %q, want %q", puzzleID, tt.puzzleID)
			}
			gotAssigned := capturedInput.Item["assignedAt"].(*types.AttributeValueMemberS).Value
			wantAssigned := assignedAt.UTC().Format(time.RFC3339)
			if gotAssigned != wantAssigned {
				t.Errorf("assignedAt = %q, want %q", gotAssigned, wantAssigned)
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

func TestGetPlay(t *testing.T) {
	tests := []struct {
		name                string
		playerID            string
		date                string
		item                map[string]types.AttributeValue
		getErr              error
		wantNil             bool
		wantErr             bool
		wantPuzzleID        string
		wantOutcome         string
		wantAssignedAt      string
		wantSubmittedAt     string
		wantServerElapsedMs int64
		wantClientClaimedMs int64
	}{
		{
			name:     "returns nil when absent",
			playerID: "user_abc",
			date:     "2026-04-30",
			item:     nil,
			wantNil:  true,
		},
		{
			name:     "returns PlayRecord with all fields when present",
			playerID: "user_abc",
			date:     "2026-04-30",
			item: map[string]types.AttributeValue{
				"PK":              &types.AttributeValueMemberS{Value: "PLAY#user_abc"},
				"SK":              &types.AttributeValueMemberS{Value: "DAILY#2026-04-30"},
				"outcome":         &types.AttributeValueMemberS{Value: PlayOutcomeStarted},
				"assignedAt":      &types.AttributeValueMemberS{Value: "2026-04-30T09:00:00Z"},
				"puzzleId":        &types.AttributeValueMemberS{Value: "puzzle-uuid-1"},
				"submittedAt":     &types.AttributeValueMemberS{Value: ""},
				"serverElapsedMs": &types.AttributeValueMemberN{Value: "0"},
				"clientClaimedMs": &types.AttributeValueMemberN{Value: "0"},
			},
			wantPuzzleID:        "puzzle-uuid-1",
			wantOutcome:         PlayOutcomeStarted,
			wantAssignedAt:      "2026-04-30T09:00:00Z",
			wantSubmittedAt:     "",
			wantServerElapsedMs: 0,
			wantClientClaimedMs: 0,
		},
		{
			name:     "propagates DDB error",
			playerID: "user_abc",
			date:     "2026-04-30",
			getErr:   errors.New("dynamodb get error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.GetItemInput
			mock := &mockDynamoDBClient{
				getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					capturedInput = params
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					return &dynamodb.GetItemOutput{Item: tt.item}, nil
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			result, err := repo.GetPlay(context.Background(), tt.playerID, tt.date)

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
			if result.PlayerID != tt.playerID {
				t.Errorf("PlayerID = %q, want %q", result.PlayerID, tt.playerID)
			}
			if result.Date != tt.date {
				t.Errorf("Date = %q, want %q", result.Date, tt.date)
			}
			if result.PuzzleID != tt.wantPuzzleID {
				t.Errorf("PuzzleID = %q, want %q", result.PuzzleID, tt.wantPuzzleID)
			}
			if result.Outcome != tt.wantOutcome {
				t.Errorf("Outcome = %q, want %q", result.Outcome, tt.wantOutcome)
			}
			if result.AssignedAt != tt.wantAssignedAt {
				t.Errorf("AssignedAt = %q, want %q", result.AssignedAt, tt.wantAssignedAt)
			}
			if result.SubmittedAt != tt.wantSubmittedAt {
				t.Errorf("SubmittedAt = %q, want %q", result.SubmittedAt, tt.wantSubmittedAt)
			}
			if result.ServerElapsedMs != tt.wantServerElapsedMs {
				t.Errorf("ServerElapsedMs = %d, want %d", result.ServerElapsedMs, tt.wantServerElapsedMs)
			}
			if result.ClientClaimedMs != tt.wantClientClaimedMs {
				t.Errorf("ClientClaimedMs = %d, want %d", result.ClientClaimedMs, tt.wantClientClaimedMs)
			}
			if capturedInput == nil {
				t.Fatal("GetItem was not called")
			}
			pk := capturedInput.Key["PK"].(*types.AttributeValueMemberS).Value
			if pk != "PLAY#"+tt.playerID {
				t.Errorf("PK = %q, want %q", pk, "PLAY#"+tt.playerID)
			}
			sk := capturedInput.Key["SK"].(*types.AttributeValueMemberS).Value
			if sk != "DAILY#"+tt.date {
				t.Errorf("SK = %q, want %q", sk, "DAILY#"+tt.date)
			}
		})
	}
}

func TestSubmitPlayTransactionally(t *testing.T) {
	assignedAt := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	submittedAt := assignedAt.Add(10 * time.Second)

	tests := []struct {
		name           string
		playerID       string
		date           string
		submission     SubmitInput
		txErr          error
		wantErr        bool
		wantSentinel   error
		wantSDKCalls   int
		wantTxItems    int
		wantLeaderItem bool
	}{
		{
			name:     "signed-in player: PLAY update + counter increment + LEADERBOARD write",
			playerID: "user_2abc",
			date:     "2026-04-30",
			submission: SubmitInput{
				PuzzleID:    "puzzle-uuid-1",
				AssignedAt:  assignedAt,
				SubmittedAt: submittedAt,
				ClientMs:    10000,
				IsAnonymous: false,
				UserID:      "user_2abc",
			},
			wantSDKCalls:   1,
			wantTxItems:    3,
			wantLeaderItem: true,
		},
		{
			name:     "anonymous player: PLAY update + counter increment, no leaderboard leg",
			playerID: "device-xyz",
			date:     "2026-04-30",
			submission: SubmitInput{
				PuzzleID:    "puzzle-uuid-1",
				AssignedAt:  assignedAt,
				SubmittedAt: submittedAt,
				ClientMs:    10000,
				IsAnonymous: true,
			},
			wantSDKCalls:   1,
			wantTxItems:    2,
			wantLeaderItem: false,
		},
		{
			name:     "idempotent — surfaces ErrPlayNotInStartedState when leg 0 condition fails",
			playerID: "user_2abc",
			date:     "2026-04-30",
			submission: SubmitInput{
				PuzzleID:    "puzzle-uuid-1",
				AssignedAt:  assignedAt,
				SubmittedAt: submittedAt,
				ClientMs:    10000,
				IsAnonymous: false,
				UserID:      "user_2abc",
			},
			txErr: &types.TransactionCanceledException{
				CancellationReasons: []types.CancellationReason{
					{Code: strPtr("ConditionalCheckFailed")},
					{Code: strPtr("None")},
					{Code: strPtr("None")},
				},
			},
			wantErr:      true,
			wantSentinel: ErrPlayNotInStartedState,
			wantSDKCalls: 1,
		},
		{
			name:     "propagates non-conditional TransactWriteItems error",
			playerID: "user_2abc",
			date:     "2026-04-30",
			submission: SubmitInput{
				PuzzleID:    "puzzle-uuid-1",
				AssignedAt:  assignedAt,
				SubmittedAt: submittedAt,
				ClientMs:    10000,
				IsAnonymous: false,
				UserID:      "user_2abc",
			},
			txErr:        errors.New("dynamodb throttled"),
			wantErr:      true,
			wantSDKCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var capturedInput *dynamodb.TransactWriteItemsInput
			calls := 0
			mock := &mockDynamoDBClient{
				transactWriteItemsFunc: func(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
					calls++
					capturedInput = params
					return &dynamodb.TransactWriteItemsOutput{}, tt.txErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			err := repo.SubmitPlayTransactionally(context.Background(), tt.playerID, tt.date, tt.submission)

			// Assert
			if calls != tt.wantSDKCalls {
				t.Errorf("TransactWriteItems calls = %d, want %d", calls, tt.wantSDKCalls)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantSentinel != nil && !errors.Is(err, tt.wantSentinel) {
					t.Fatalf("error = %v, want sentinel %v", err, tt.wantSentinel)
				}
				if tt.wantSentinel == nil && !strings.Contains(err.Error(), "submitting daily play") {
					t.Errorf("error = %v, want wrapped 'submitting daily play' context", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedInput == nil {
				t.Fatal("TransactWriteItems was not called")
			}
			if got := len(capturedInput.TransactItems); got != tt.wantTxItems {
				t.Fatalf("TransactItems len = %d, want %d", got, tt.wantTxItems)
			}

			// Leg 0 — PLAY row update.
			leg0 := capturedInput.TransactItems[0]
			if leg0.Update == nil {
				t.Fatal("leg 0 missing Update")
			}
			if pk := leg0.Update.Key["PK"].(*types.AttributeValueMemberS).Value; pk != "PLAY#"+tt.playerID {
				t.Errorf("leg 0 PK = %q, want %q", pk, "PLAY#"+tt.playerID)
			}
			if sk := leg0.Update.Key["SK"].(*types.AttributeValueMemberS).Value; sk != "DAILY#"+tt.date {
				t.Errorf("leg 0 SK = %q, want %q", sk, "DAILY#"+tt.date)
			}
			if solved := leg0.Update.ExpressionAttributeValues[":solved"].(*types.AttributeValueMemberS).Value; solved != PlayOutcomeSolved {
				t.Errorf("leg 0 :solved = %q, want %q", solved, PlayOutcomeSolved)
			}
			if started := leg0.Update.ExpressionAttributeValues[":started"].(*types.AttributeValueMemberS).Value; started != PlayOutcomeStarted {
				t.Errorf("leg 0 :started = %q, want %q", started, PlayOutcomeStarted)
			}
			if serverMs := leg0.Update.ExpressionAttributeValues[":serverMs"].(*types.AttributeValueMemberN).Value; serverMs != "10000" {
				t.Errorf("leg 0 :serverMs = %q, want %q", serverMs, "10000")
			}
			if clientMs := leg0.Update.ExpressionAttributeValues[":clientMs"].(*types.AttributeValueMemberN).Value; clientMs != "10000" {
				t.Errorf("leg 0 :clientMs = %q, want %q", clientMs, "10000")
			}

			// Leg 1 — schedule counter increment.
			leg1 := capturedInput.TransactItems[1]
			if leg1.Update == nil {
				t.Fatal("leg 1 missing Update")
			}
			if pk := leg1.Update.Key["PK"].(*types.AttributeValueMemberS).Value; pk != "DAILY#"+tt.date {
				t.Errorf("leg 1 PK = %q, want %q", pk, "DAILY#"+tt.date)
			}
			if sk := leg1.Update.Key["SK"].(*types.AttributeValueMemberS).Value; sk != dailySingletonSK {
				t.Errorf("leg 1 SK = %q, want %q", sk, dailySingletonSK)
			}
			if expr := *leg1.Update.UpdateExpression; !strings.Contains(expr, "ADD") {
				t.Errorf("leg 1 UpdateExpression = %q, want it to ADD counter", expr)
			}
			if name := leg1.Update.ExpressionAttributeNames["#solved"]; name != ScheduleCounterSolved {
				t.Errorf("leg 1 #solved name = %q, want %q", name, ScheduleCounterSolved)
			}
			if one := leg1.Update.ExpressionAttributeValues[":one"].(*types.AttributeValueMemberN).Value; one != "1" {
				t.Errorf("leg 1 :one = %q, want %q", one, "1")
			}

			// Leg 2 (signed-in only) — leaderboard Put.
			if tt.wantLeaderItem {
				leg2 := capturedInput.TransactItems[2]
				if leg2.Put == nil {
					t.Fatal("leg 2 missing Put")
				}
				if pk := leg2.Put.Item["PK"].(*types.AttributeValueMemberS).Value; pk != "DAILY-LEADERBOARD#"+tt.date {
					t.Errorf("leg 2 PK = %q, want %q", pk, "DAILY-LEADERBOARD#"+tt.date)
				}
				wantSK := buildLeaderboardSK(10000, tt.submission.UserID)
				if sk := leg2.Put.Item["SK"].(*types.AttributeValueMemberS).Value; sk != wantSK {
					t.Errorf("leg 2 SK = %q, want %q", sk, wantSK)
				}
				if uid := leg2.Put.Item["userId"].(*types.AttributeValueMemberS).Value; uid != tt.submission.UserID {
					t.Errorf("leg 2 userId = %q, want %q", uid, tt.submission.UserID)
				}
				if elapsed := leg2.Put.Item["serverElapsedMs"].(*types.AttributeValueMemberN).Value; elapsed != "10000" {
					t.Errorf("leg 2 serverElapsedMs = %q, want %q", elapsed, "10000")
				}
				if pid := leg2.Put.Item["puzzleId"].(*types.AttributeValueMemberS).Value; pid != tt.submission.PuzzleID {
					t.Errorf("leg 2 puzzleId = %q, want %q", pid, tt.submission.PuzzleID)
				}
			} else if len(capturedInput.TransactItems) > 2 {
				t.Errorf("anonymous submission produced %d transact items, want 2 (no leaderboard leg)", len(capturedInput.TransactItems))
			}
		})
	}
}

func TestFinalizeDailyTransaction(t *testing.T) {
	const (
		date            = "2026-04-30"
		puzzleID        = "puzzle-uuid-1"
		sourcePartition = "9#standard"
	)

	t.Run("Confirm_Success — 3 legs in correct order", func(t *testing.T) {
		// Arrange
		var capturedInput *dynamodb.TransactWriteItemsInput
		mock := &mockDynamoDBClient{
			transactWriteItemsFunc: func(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
				capturedInput = params
				return &dynamodb.TransactWriteItemsOutput{}, nil
			},
		}
		repo := NewPuzzleRepository(mock, "puzzle-pool")

		// Act
		err := repo.FinalizeDailyTransaction(context.Background(), date, puzzleID, sourcePartition, FinalizeModeConfirm)

		// Assert
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedInput == nil {
			t.Fatal("TransactWriteItems was not called")
		}
		if got := len(capturedInput.TransactItems); got != 3 {
			t.Fatalf("TransactItems len = %d, want 3", got)
		}

		// Leg 0 — Put schedule row, conditional on absence.
		leg0 := capturedInput.TransactItems[0]
		if leg0.Put == nil {
			t.Fatal("leg 0 missing Put")
		}
		if pk := leg0.Put.Item["PK"].(*types.AttributeValueMemberS).Value; pk != "DAILY#"+date {
			t.Errorf("leg 0 PK = %q, want %q", pk, "DAILY#"+date)
		}
		if sk := leg0.Put.Item["SK"].(*types.AttributeValueMemberS).Value; sk != dailySingletonSK {
			t.Errorf("leg 0 SK = %q, want %q", sk, dailySingletonSK)
		}
		if pid := leg0.Put.Item["puzzleId"].(*types.AttributeValueMemberS).Value; pid != puzzleID {
			t.Errorf("leg 0 puzzleId = %q, want %q", pid, puzzleID)
		}
		if sp := leg0.Put.Item["sourcePartition"].(*types.AttributeValueMemberS).Value; sp != sourcePartition {
			t.Errorf("leg 0 sourcePartition = %q, want %q", sp, sourcePartition)
		}
		if assigned := leg0.Put.Item["assignedAt"].(*types.AttributeValueMemberS).Value; assigned == "" {
			t.Error("leg 0 assignedAt should be stamped")
		}
		counters, ok := leg0.Put.Item["counters"].(*types.AttributeValueMemberM)
		if !ok {
			t.Fatal("leg 0 counters should be a map attribute")
		}
		if started := counters.Value["started"].(*types.AttributeValueMemberN).Value; started != "0" {
			t.Errorf("leg 0 counters.started = %q, want 0", started)
		}
		if solved := counters.Value["solved"].(*types.AttributeValueMemberN).Value; solved != "0" {
			t.Errorf("leg 0 counters.solved = %q, want 0", solved)
		}
		if leg0.Put.ConditionExpression == nil || *leg0.Put.ConditionExpression != "attribute_not_exists(PK)" {
			got := "<nil>"
			if leg0.Put.ConditionExpression != nil {
				got = *leg0.Put.ConditionExpression
			}
			t.Errorf("leg 0 ConditionExpression = %q, want attribute_not_exists(PK)", got)
		}

		// Leg 1 — Update PuzzleRecord lastDailyDate (unconditional).
		leg1 := capturedInput.TransactItems[1]
		if leg1.Update == nil {
			t.Fatal("leg 1 missing Update")
		}
		if pk := leg1.Update.Key["PK"].(*types.AttributeValueMemberS).Value; pk != sourcePartition {
			t.Errorf("leg 1 PK = %q, want %q", pk, sourcePartition)
		}
		if sk := leg1.Update.Key["SK"].(*types.AttributeValueMemberS).Value; sk != puzzleID {
			t.Errorf("leg 1 SK = %q, want %q", sk, puzzleID)
		}
		if expr := *leg1.Update.UpdateExpression; !strings.Contains(expr, "SET lastDailyDate = :date") {
			t.Errorf("leg 1 UpdateExpression = %q, want SET lastDailyDate = :date", expr)
		}
		if leg1.Update.ConditionExpression != nil {
			t.Errorf("leg 1 ConditionExpression should be nil (recycle must succeed even with stale lastDailyDate); got %q", *leg1.Update.ConditionExpression)
		}
		if d := leg1.Update.ExpressionAttributeValues[":date"].(*types.AttributeValueMemberS).Value; d != date {
			t.Errorf("leg 1 :date = %q, want %q", d, date)
		}

		// Leg 2 — Delete candidate, conditional on puzzleId match.
		leg2 := capturedInput.TransactItems[2]
		if leg2.Delete == nil {
			t.Fatal("leg 2 missing Delete")
		}
		if pk := leg2.Delete.Key["PK"].(*types.AttributeValueMemberS).Value; pk != dailyCandidatePK {
			t.Errorf("leg 2 PK = %q, want %q", pk, dailyCandidatePK)
		}
		if sk := leg2.Delete.Key["SK"].(*types.AttributeValueMemberS).Value; sk != dailySingletonSK {
			t.Errorf("leg 2 SK = %q, want %q", sk, dailySingletonSK)
		}
		if leg2.Delete.ConditionExpression == nil {
			t.Fatal("leg 2 ConditionExpression must guard against candidate-swap race")
		}
		if !strings.Contains(*leg2.Delete.ConditionExpression, "puzzleId") {
			t.Errorf("leg 2 ConditionExpression = %q, want it to reference puzzleId", *leg2.Delete.ConditionExpression)
		}
		if pid := leg2.Delete.ExpressionAttributeValues[":pid"].(*types.AttributeValueMemberS).Value; pid != puzzleID {
			t.Errorf("leg 2 :pid = %q, want %q", pid, puzzleID)
		}
	})

	t.Run("Recycle_Success — 2 legs (no candidate delete)", func(t *testing.T) {
		// Arrange
		var capturedInput *dynamodb.TransactWriteItemsInput
		mock := &mockDynamoDBClient{
			transactWriteItemsFunc: func(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
				capturedInput = params
				return &dynamodb.TransactWriteItemsOutput{}, nil
			},
		}
		repo := NewPuzzleRepository(mock, "puzzle-pool")

		// Act
		err := repo.FinalizeDailyTransaction(context.Background(), date, puzzleID, sourcePartition, FinalizeModeRecycle)

		// Assert
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedInput == nil {
			t.Fatal("TransactWriteItems was not called")
		}
		if got := len(capturedInput.TransactItems); got != 2 {
			t.Fatalf("TransactItems len = %d, want 2 (recycle has no candidate-delete leg)", got)
		}
		if capturedInput.TransactItems[0].Put == nil {
			t.Error("recycle leg 0 should be Put (schedule row)")
		}
		if capturedInput.TransactItems[1].Update == nil {
			t.Error("recycle leg 1 should be Update (PuzzleRecord lastDailyDate)")
		}
	})

	t.Run("AlreadyFinalized — leg-0 ConditionalCheckFailed maps to ErrScheduleAlreadyFinalized", func(t *testing.T) {
		// Arrange
		mock := &mockDynamoDBClient{
			transactWriteItemsFunc: func(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
				return nil, &types.TransactionCanceledException{
					CancellationReasons: []types.CancellationReason{
						{Code: strPtr("ConditionalCheckFailed")},
						{Code: strPtr("None")},
						{Code: strPtr("None")},
					},
				}
			},
		}
		repo := NewPuzzleRepository(mock, "puzzle-pool")

		// Act
		err := repo.FinalizeDailyTransaction(context.Background(), date, puzzleID, sourcePartition, FinalizeModeConfirm)

		// Assert
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrScheduleAlreadyFinalized) {
			t.Fatalf("error = %v, want ErrScheduleAlreadyFinalized", err)
		}
	})

	t.Run("CandidateMismatch — leg-2 ConditionalCheckFailed wraps non-sentinel", func(t *testing.T) {
		// Arrange
		mock := &mockDynamoDBClient{
			transactWriteItemsFunc: func(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
				return nil, &types.TransactionCanceledException{
					CancellationReasons: []types.CancellationReason{
						{Code: strPtr("None")},
						{Code: strPtr("None")},
						{Code: strPtr("ConditionalCheckFailed")},
					},
				}
			},
		}
		repo := NewPuzzleRepository(mock, "puzzle-pool")

		// Act
		err := repo.FinalizeDailyTransaction(context.Background(), date, puzzleID, sourcePartition, FinalizeModeConfirm)

		// Assert
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, ErrScheduleAlreadyFinalized) {
			t.Fatalf("candidate-mismatch must NOT map to ErrScheduleAlreadyFinalized; got %v", err)
		}
	})

	t.Run("InvalidMode — synchronous reject, no DDB call", func(t *testing.T) {
		// Arrange
		calls := 0
		mock := &mockDynamoDBClient{
			transactWriteItemsFunc: func(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
				calls++
				return &dynamodb.TransactWriteItemsOutput{}, nil
			},
		}
		repo := NewPuzzleRepository(mock, "puzzle-pool")

		// Act
		err := repo.FinalizeDailyTransaction(context.Background(), date, puzzleID, sourcePartition, FinalizeMode("bogus"))

		// Assert
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if calls != 0 {
			t.Errorf("TransactWriteItems should not be called for invalid mode; got %d calls", calls)
		}
	})

	t.Run("GenericFailure — non-tx error wrapped", func(t *testing.T) {
		// Arrange
		mock := &mockDynamoDBClient{
			transactWriteItemsFunc: func(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
				return nil, errors.New("some ddb error")
			},
		}
		repo := NewPuzzleRepository(mock, "puzzle-pool")

		// Act
		err := repo.FinalizeDailyTransaction(context.Background(), date, puzzleID, sourcePartition, FinalizeModeConfirm)

		// Assert
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, ErrScheduleAlreadyFinalized) {
			t.Fatalf("generic failure must NOT map to ErrScheduleAlreadyFinalized; got %v", err)
		}
		if !strings.Contains(err.Error(), "some ddb error") {
			t.Errorf("error should wrap underlying cause; got %v", err)
		}
	})
}

// itoa is a tiny shim so the table fixtures stay readable.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func TestLeaderboardRank_FirstPlace(t *testing.T) {
	// Arrange — mock returns Count=1 (only the player's own row counts as
	// at-or-faster, i.e. the player is alone at the top of the leaderboard).
	mock := &mockDynamoDBClient{
		queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			return &dynamodb.QueryOutput{Count: 1}, nil
		},
	}
	repo := NewPuzzleRepository(mock, "puzzle-pool")

	// Act
	rank, err := repo.LeaderboardRank(context.Background(), "2026-05-02", 12345, "user_abc")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rank != 1 {
		t.Errorf("rank = %d, want 1", rank)
	}
}

func TestLeaderboardRank_NormalRanking(t *testing.T) {
	// Arrange — mock returns Count=5 (four faster rows ahead of the player
	// plus the player's own row → rank 5).
	mock := &mockDynamoDBClient{
		queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			return &dynamodb.QueryOutput{Count: 5}, nil
		},
	}
	repo := NewPuzzleRepository(mock, "puzzle-pool")

	// Act
	rank, err := repo.LeaderboardRank(context.Background(), "2026-05-02", 67890, "user_xyz")

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rank != 5 {
		t.Errorf("rank = %d, want 5", rank)
	}
}
