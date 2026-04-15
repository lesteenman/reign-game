// Package repository provides data access for puzzle storage in DynamoDB.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBAPI defines the DynamoDB operations used by PuzzleRepository.
// Keeping this minimal makes testing straightforward via mock implementations.
type DynamoDBAPI interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// PuzzleRecord represents a puzzle stored in the puzzle-pool DynamoDB table.
// Fields map directly to the DynamoDB attributes defined in the design document.
type PuzzleRecord struct {
	// GridSize is the N dimension of the NxN grid (e.g., 5, 7, 9).
	GridSize int `dynamodbav:"-"`
	// Mode is the game mode: "standard" or "double".
	Mode string `dynamodbav:"-"`
	// ID is the puzzle UUID, stored as the sort key (SK).
	ID string `dynamodbav:"-"`
	// Status tracks the puzzle lifecycle: ready, served, solved, skipped.
	Status string `dynamodbav:"status"`
	// Verdict is the curation verdict: none, upvote, downvote, skip.
	Verdict string `dynamodbav:"verdict"`
	// RegionMap is a 2D array of region IDs defining which region each cell belongs to.
	RegionMap [][]int `dynamodbav:"regionMap"`
	// Solution is a 2D boolean array indicating correct marker placements.
	Solution [][]bool `dynamodbav:"solution"`
	// Pipeline is the generation pipeline strategy used (e.g., "iterative").
	Pipeline string `dynamodbav:"pipeline"`
	// Solver is the solver strategy used (e.g., "propagation").
	Solver string `dynamodbav:"solver"`
	// Regions is the region generation strategy used (e.g., "bfs").
	Regions string `dynamodbav:"regions"`
	// RegionVariance controls region shape irregularity (0.0 to 1.0).
	RegionVariance float64 `dynamodbav:"regionVariance"`
	// Deducible indicates whether the puzzle is solvable without guessing.
	Deducible bool `dynamodbav:"deducible"`
	// Concurrency is the number of goroutines used during generation.
	Concurrency int `dynamodbav:"concurrency"`
	// GenerationDurationMs is the wall-clock generation time in milliseconds.
	GenerationDurationMs int64 `dynamodbav:"generationDurationMs"`
	// CreatedAt is the ISO 8601 timestamp when the puzzle was generated.
	CreatedAt string `dynamodbav:"createdAt"`
	// ServedAt is the ISO 8601 timestamp when the puzzle was served (empty until served).
	ServedAt string `dynamodbav:"servedAt"`
}

// PuzzleRepository provides data access methods for puzzles in DynamoDB.
type PuzzleRepository struct {
	client    DynamoDBAPI
	tableName string
}

// NewPuzzleRepository creates a PuzzleRepository with the given DynamoDB client
// and table name.
func NewPuzzleRepository(client DynamoDBAPI, tableName string) *PuzzleRepository {
	return &PuzzleRepository{
		client:    client,
		tableName: tableName,
	}
}

// buildPK constructs the partition key from grid size and mode.
func buildPK(size int, mode string) string {
	return fmt.Sprintf("%d#%s", size, mode)
}

// PutPuzzle writes a puzzle record to DynamoDB with status set to "ready".
// The partition key is constructed from the puzzle's grid size and mode,
// and the sort key is the puzzle ID.
func (r *PuzzleRepository) PutPuzzle(ctx context.Context, puzzle *PuzzleRecord) error {
	puzzle.Status = "ready"

	item, err := attributevalue.MarshalMap(puzzle)
	if err != nil {
		return fmt.Errorf("marshaling puzzle record: %w", err)
	}

	// Set PK and SK explicitly since they are derived from GridSize/Mode/ID.
	pk := buildPK(puzzle.GridSize, puzzle.Mode)
	item["PK"] = &types.AttributeValueMemberS{Value: pk}
	item["SK"] = &types.AttributeValueMemberS{Value: puzzle.ID}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("putting puzzle %s: %w", puzzle.ID, err)
	}

	return nil
}

// NextReady queries for one ready puzzle matching the given size and mode.
// Returns nil, nil if no ready puzzles are found.
func (r *PuzzleRepository) NextReady(ctx context.Context, size int, mode string) (*PuzzleRecord, error) {
	pk := buildPK(size, mode)

	output, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		FilterExpression:       aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: pk},
			":status": &types.AttributeValueMemberS{Value: "ready"},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("querying next ready puzzle for %s: %w", pk, err)
	}

	if len(output.Items) == 0 {
		return nil, nil
	}

	var record PuzzleRecord
	if err := attributevalue.UnmarshalMap(output.Items[0], &record); err != nil {
		return nil, fmt.Errorf("unmarshaling puzzle record: %w", err)
	}

	// Extract PK components and SK since they are tagged with "-".
	if pkAttr, ok := output.Items[0]["PK"].(*types.AttributeValueMemberS); ok {
		// Parse grid size from PK (format: "{size}#{mode}").
		var parsedSize int
		var parsedMode string
		if _, err := fmt.Sscanf(pkAttr.Value, "%d#%s", &parsedSize, &parsedMode); err == nil {
			record.GridSize = parsedSize
			record.Mode = parsedMode
		}
	}
	if skAttr, ok := output.Items[0]["SK"].(*types.AttributeValueMemberS); ok {
		record.ID = skAttr.Value
	}

	return &record, nil
}

// MarkServed updates a puzzle's status to "served" and sets the servedAt
// timestamp to the current time in ISO 8601 format.
func (r *PuzzleRepository) MarkServed(ctx context.Context, pk, sk string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET #status = :status, servedAt = :servedAt"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":   &types.AttributeValueMemberS{Value: "served"},
			":servedAt": &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		return fmt.Errorf("marking puzzle %s/%s as served: %w", pk, sk, err)
	}

	return nil
}

// UpdateStatus updates a puzzle's status to the given value.
func (r *PuzzleRepository) UpdateStatus(ctx context.Context, pk, sk, status string) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET #status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: status},
		},
	})
	if err != nil {
		return fmt.Errorf("updating puzzle %s/%s status to %s: %w", pk, sk, status, err)
	}

	return nil
}

// CountReady returns the number of puzzles with status "ready" for the given
// size and mode combination.
func (r *PuzzleRepository) CountReady(ctx context.Context, size int, mode string) (int, error) {
	pk := buildPK(size, mode)

	output, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		FilterExpression:       aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: pk},
			":status": &types.AttributeValueMemberS{Value: "ready"},
		},
		Select: types.SelectCount,
	})
	if err != nil {
		return 0, fmt.Errorf("counting ready puzzles for %s: %w", pk, err)
	}

	return int(output.Count), nil
}
