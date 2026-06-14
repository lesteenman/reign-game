package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestPutPack(t *testing.T) {
	tests := []struct {
		name          string
		pack          PackRecord
		putErr        error
		wantErr       bool
		wantExists    bool
		wantPK        string
		wantSK        string
		wantCondition string
	}{
		{
			name: "creates pack successfully",
			pack: PackRecord{
				Slug:      "starter",
				Name:      "Starter Pack",
				Size:      7,
				Mode:      "standard",
				Published: false,
				PuzzleIDs: []string{"id-1", "id-2"},
				CreatedAt: "2026-06-14T00:00:00Z",
				UpdatedAt: "2026-06-14T00:00:00Z",
			},
			wantErr:       false,
			wantPK:        "PACK",
			wantSK:        "starter",
			wantCondition: "attribute_not_exists(PK)",
		},
		{
			name: "returns ErrPackAlreadyExists on conflict",
			pack: PackRecord{
				Slug: "starter",
				Size: 7,
				Mode: "standard",
			},
			putErr:     &types.ConditionalCheckFailedException{Message: strPtr("conditional check failed")},
			wantErr:    true,
			wantExists: true,
		},
		{
			name: "propagates other DynamoDB errors",
			pack: PackRecord{
				Slug: "starter",
				Size: 7,
				Mode: "standard",
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
				putItemFunc: func(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
					capturedInput = params
					return &dynamodb.PutItemOutput{}, tt.putErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			err := repo.PutPack(context.Background(), &tt.pack)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantExists && !errors.Is(err, ErrPackAlreadyExists) {
					t.Fatalf("expected ErrPackAlreadyExists, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedInput == nil {
				t.Fatal("PutItem was not called")
			}
			pk, ok := capturedInput.Item["PK"].(*types.AttributeValueMemberS)
			if !ok || pk.Value != tt.wantPK {
				t.Errorf("PK = %v, want %q", capturedInput.Item["PK"], tt.wantPK)
			}
			sk, ok := capturedInput.Item["SK"].(*types.AttributeValueMemberS)
			if !ok || sk.Value != tt.wantSK {
				t.Errorf("SK = %v, want %q", capturedInput.Item["SK"], tt.wantSK)
			}
			if capturedInput.ConditionExpression == nil || *capturedInput.ConditionExpression != tt.wantCondition {
				t.Errorf("ConditionExpression = %v, want %q", capturedInput.ConditionExpression, tt.wantCondition)
			}
		})
	}
}

func TestGetPack(t *testing.T) {
	tests := []struct {
		name     string
		item     map[string]types.AttributeValue
		getErr   error
		wantErr  bool
		wantNil  bool
		wantSlug string
		wantName string
	}{
		{
			name: "returns pack",
			item: map[string]types.AttributeValue{
				"PK":        &types.AttributeValueMemberS{Value: "PACK"},
				"SK":        &types.AttributeValueMemberS{Value: "starter"},
				"name":      &types.AttributeValueMemberS{Value: "Starter Pack"},
				"size":      &types.AttributeValueMemberN{Value: "7"},
				"mode":      &types.AttributeValueMemberS{Value: "standard"},
				"published": &types.AttributeValueMemberBOOL{Value: true},
				"puzzleIds": &types.AttributeValueMemberL{Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "id-1"},
				}},
				"createdAt": &types.AttributeValueMemberS{Value: "2026-06-14T00:00:00Z"},
				"updatedAt": &types.AttributeValueMemberS{Value: "2026-06-14T00:00:00Z"},
			},
			wantSlug: "starter",
			wantName: "Starter Pack",
		},
		{
			name:    "returns nil when absent",
			item:    nil,
			wantNil: true,
		},
		{
			name:    "propagates DynamoDB error",
			getErr:  errors.New("boom"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mock := &mockDynamoDBClient{
				getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					return &dynamodb.GetItemOutput{Item: tt.item}, tt.getErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			got, err := repo.GetPack(context.Background(), "starter")

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
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got.Slug != tt.wantSlug {
				t.Errorf("Slug = %q, want %q", got.Slug, tt.wantSlug)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Size != 7 || got.Mode != "standard" || !got.Published {
				t.Errorf("unexpected fields: %+v", got)
			}
			if len(got.PuzzleIDs) != 1 || got.PuzzleIDs[0] != "id-1" {
				t.Errorf("PuzzleIDs = %v", got.PuzzleIDs)
			}
		})
	}
}

func TestListPacks(t *testing.T) {
	// Arrange
	mock := &mockDynamoDBClient{
		queryFunc: func(_ context.Context, params *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			if params.IndexName != nil {
				t.Errorf("ListPacks must query the base table, not a GSI")
			}
			return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{
				{
					"PK":        &types.AttributeValueMemberS{Value: "PACK"},
					"SK":        &types.AttributeValueMemberS{Value: "a"},
					"name":      &types.AttributeValueMemberS{Value: "A"},
					"size":      &types.AttributeValueMemberN{Value: "7"},
					"mode":      &types.AttributeValueMemberS{Value: "standard"},
					"published": &types.AttributeValueMemberBOOL{Value: false},
				},
				{
					"PK":        &types.AttributeValueMemberS{Value: "PACK"},
					"SK":        &types.AttributeValueMemberS{Value: "b"},
					"name":      &types.AttributeValueMemberS{Value: "B"},
					"size":      &types.AttributeValueMemberN{Value: "9"},
					"mode":      &types.AttributeValueMemberS{Value: "double"},
					"published": &types.AttributeValueMemberBOOL{Value: true},
				},
			}}, nil
		},
	}
	repo := NewPuzzleRepository(mock, "puzzle-pool")

	// Act
	got, err := repo.ListPacks(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Slug != "a" || got[1].Slug != "b" {
		t.Errorf("slugs = %q, %q", got[0].Slug, got[1].Slug)
	}
}

func TestUpdatePack(t *testing.T) {
	tests := []struct {
		name      string
		updateErr error
		wantErr   bool
		wantNF    bool
	}{
		{name: "updates successfully"},
		{
			name:      "maps ConditionalCheckFailed to ErrPackNotFound",
			updateErr: &types.ConditionalCheckFailedException{},
			wantErr:   true,
			wantNF:    true,
		},
		{
			name:      "propagates other errors",
			updateErr: errors.New("boom"),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var captured *dynamodb.UpdateItemInput
			mock := &mockDynamoDBClient{
				updateItemFunc: func(_ context.Context, params *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
					captured = params
					return &dynamodb.UpdateItemOutput{}, tt.updateErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")
			pack := &PackRecord{Slug: "starter", Name: "New", PuzzleIDs: []string{"id-1"}, Published: true, UpdatedAt: "2026-06-14T00:00:00Z"}

			// Act
			err := repo.UpdatePack(context.Background(), pack)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantNF && !errors.Is(err, ErrPackNotFound) {
					t.Fatalf("expected ErrPackNotFound, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if captured.ConditionExpression == nil || *captured.ConditionExpression != "attribute_exists(PK)" {
				t.Errorf("ConditionExpression = %v, want attribute_exists(PK)", captured.ConditionExpression)
			}
		})
	}
}

func TestDeletePack(t *testing.T) {
	tests := []struct {
		name      string
		deleteErr error
		wantErr   bool
		wantNF    bool
	}{
		{name: "deletes successfully"},
		{
			name:      "maps ConditionalCheckFailed to ErrPackNotFound",
			deleteErr: &types.ConditionalCheckFailedException{},
			wantErr:   true,
			wantNF:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mock := &mockDynamoDBClient{
				deleteItemFunc: func(_ context.Context, _ *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
					return &dynamodb.DeleteItemOutput{}, tt.deleteErr
				},
			}
			repo := NewPuzzleRepository(mock, "puzzle-pool")

			// Act
			err := repo.DeletePack(context.Background(), "starter")

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantNF && !errors.Is(err, ErrPackNotFound) {
					t.Fatalf("expected ErrPackNotFound, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
