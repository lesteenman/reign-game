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
			wantPK:          DailyCandidatePK,
			wantSK:          DailySingletonSK,
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
				"PK":              &types.AttributeValueMemberS{Value: DailyCandidatePK},
				"SK":              &types.AttributeValueMemberS{Value: DailySingletonSK},
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
			if pk != DailyCandidatePK {
				t.Errorf("PK = %q, want %q", pk, DailyCandidatePK)
			}
			sk := capturedInput.Key["SK"].(*types.AttributeValueMemberS).Value
			if sk != DailySingletonSK {
				t.Errorf("SK = %q, want %q", sk, DailySingletonSK)
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
				"SK":              &types.AttributeValueMemberS{Value: DailySingletonSK},
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
			if sk != DailySingletonSK {
				t.Errorf("SK = %q, want %q", sk, DailySingletonSK)
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
