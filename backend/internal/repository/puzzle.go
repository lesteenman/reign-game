// Package repository provides data access for puzzle storage in DynamoDB.
package repository

import (
	"context"
	"errors"
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
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

// ConfigRecord represents a generation config stored in the puzzle-pool DynamoDB table.
// Config items share the table with puzzles, using PK="CONFIG" and SK="{size}#{mode}".
type ConfigRecord struct {
	Size        int    `dynamodbav:"-"`
	Mode        string `dynamodbav:"-"`
	Threshold   int    `dynamodbav:"threshold"`
	Enabled     bool   `dynamodbav:"enabled"`
	MaxAttempts int    `dynamodbav:"maxAttempts,omitempty"`
}

// ConfigAlreadyExistsError is returned when CreateConfig is called for a config
// that already exists in the table.
type ConfigAlreadyExistsError struct {
	Size int
	Mode string
}

// Error implements the error interface for ConfigAlreadyExistsError.
func (e *ConfigAlreadyExistsError) Error() string {
	return fmt.Sprintf("config already exists for %d#%s", e.Size, e.Mode)
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
	// Difficulty is the generator-assigned tier (0 unknown, 1 Easy, 2 Medium,
	// 3 Hard, 4 Expert).
	Difficulty int `dynamodbav:"difficulty"`
	// MaxTier is the highest rule tier that fired during the deductive solve.
	MaxTier int `dynamodbav:"maxTier"`
	// TierCounts is the per-tier rule firing count (length 5; index 0 unused).
	TierCounts []int `dynamodbav:"tierCounts"`
	// TraceLen is the total number of rule firings in the deductive trace.
	TraceLen int `dynamodbav:"traceLen"`
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

	// Note: DynamoDB Limit applies before FilterExpression, so we cannot
	// use Limit=1 here — it would read one item and discard it if the
	// status doesn't match. Instead we scan the full partition (small,
	// typically <60 items) and filter server-side.
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

// GetAllConfigs returns all config records from the puzzle-pool table.
// Config items are stored with PK="CONFIG" and SK="{size}#{mode}".
func (r *PuzzleRepository) GetAllConfigs(ctx context.Context) ([]ConfigRecord, error) {
	output, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "CONFIG"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("querying configs: %w", err)
	}

	configs := make([]ConfigRecord, 0, len(output.Items))
	for _, item := range output.Items {
		var record ConfigRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, fmt.Errorf("unmarshaling config record: %w", err)
		}

		// Parse Size and Mode from SK since they are tagged with "-".
		if skAttr, ok := item["SK"].(*types.AttributeValueMemberS); ok {
			var parsedSize int
			var parsedMode string
			if _, err := fmt.Sscanf(skAttr.Value, "%d#%s", &parsedSize, &parsedMode); err == nil {
				record.Size = parsedSize
				record.Mode = parsedMode
			}
		}

		configs = append(configs, record)
	}

	return configs, nil
}

// GetConfig returns a single config record for the given size and mode.
// Returns nil, nil if no config is found.
func (r *PuzzleRepository) GetConfig(ctx context.Context, size int, mode string) (*ConfigRecord, error) {
	sk := buildPK(size, mode)

	output, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "CONFIG"},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting config for %s: %w", sk, err)
	}

	if output.Item == nil {
		return nil, nil
	}

	var record ConfigRecord
	if err := attributevalue.UnmarshalMap(output.Item, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling config record: %w", err)
	}

	record.Size = size
	record.Mode = mode

	return &record, nil
}

// PutConfig writes a config record to DynamoDB, unconditionally overwriting
// any existing config for the same size and mode.
func (r *PuzzleRepository) PutConfig(ctx context.Context, config *ConfigRecord) error {
	item, err := attributevalue.MarshalMap(config)
	if err != nil {
		return fmt.Errorf("marshaling config record: %w", err)
	}

	sk := buildPK(config.Size, config.Mode)
	item["PK"] = &types.AttributeValueMemberS{Value: "CONFIG"}
	item["SK"] = &types.AttributeValueMemberS{Value: sk}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("putting config %s: %w", sk, err)
	}

	return nil
}

// CreateConfig writes a config record to DynamoDB only if one does not already
// exist for the same size and mode. Returns ConfigAlreadyExistsError if a
// config already exists.
func (r *PuzzleRepository) CreateConfig(ctx context.Context, config *ConfigRecord) error {
	item, err := attributevalue.MarshalMap(config)
	if err != nil {
		return fmt.Errorf("marshaling config record: %w", err)
	}

	sk := buildPK(config.Size, config.Mode)
	item["PK"] = &types.AttributeValueMemberS{Value: "CONFIG"}
	item["SK"] = &types.AttributeValueMemberS{Value: sk}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return &ConfigAlreadyExistsError{Size: config.Size, Mode: config.Mode}
		}
		return fmt.Errorf("creating config %s: %w", sk, err)
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
