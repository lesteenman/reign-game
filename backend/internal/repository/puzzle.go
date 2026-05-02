// Package repository provides data access for puzzle storage in DynamoDB.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
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

// ErrPuzzleNotFound is returned by status-mutating calls (MarkServed,
// UpdateStatus) when the target puzzle row does not exist. Callers map
// this to a 404 instead of letting DynamoDB silently upsert a new row.
var ErrPuzzleNotFound = errors.New("puzzle not found")

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

// VerdictSummary is the denormalized projection of admin verdict
// counts on a puzzle. The row family at PK="VERDICT#{size}#{mode}#{id}"
// is canonical; this summary is recomputed from that row family on
// every write so it stays consistent with no transactional cost. See
// VR-05 / VR-06 in specs/repository.md.
type VerdictSummary struct {
	// Up is the number of admins who voted "up" on this puzzle.
	Up int `dynamodbav:"up" json:"up"`
	// Down is the number of admins who voted "down" on this puzzle.
	Down int `dynamodbav:"down" json:"down"`
	// LastUpdatedAt is the RFC 3339 timestamp of the most recent
	// recompute. Empty on never-voted puzzles.
	LastUpdatedAt string `dynamodbav:"lastUpdatedAt" json:"lastUpdatedAt"`
}

// VerdictRecord is one admin's verdict on one puzzle. Stored under
// PK="VERDICT#{size}#{mode}#{puzzleId}", SK="{raterRole}#{raterId}".
// The PK/SK component fields (PuzzleID, GridSize, Mode, RaterID,
// RaterRole) are derived from the keys at write/read time and not
// stored as separate attributes — matching the PuzzleRecord and
// ConfigRecord conventions in this file. See VR-01 / VR-02.
type VerdictRecord struct {
	// PuzzleID is the puzzle UUID, embedded in the PK.
	PuzzleID string `dynamodbav:"-"`
	// GridSize is the puzzle's N dimension, embedded in the PK.
	GridSize int `dynamodbav:"-"`
	// Mode is the puzzle's mode ("standard" / "double"), embedded in the PK.
	Mode string `dynamodbav:"-"`
	// RaterID is the Clerk user ID of the admin who submitted the verdict.
	RaterID string `dynamodbav:"-"`
	// RaterRole is the rater's role at submission time. Always
	// "admin" in this phase; the SK shape is multi-rater-ready.
	RaterRole string `dynamodbav:"-"`
	// Value is the verdict — "up" or "down".
	Value string `dynamodbav:"value"`
	// PlayTimeMs is the wall-clock time in milliseconds the rater
	// spent on the producing attempt. Used by R-084 calibration.
	PlayTimeMs int64 `dynamodbav:"playTimeMs"`
	// Outcome distinguishes a verdict cast on a solved puzzle from
	// one cast on a skipped puzzle ("solved" or "skipped").
	Outcome string `dynamodbav:"outcome"`
	// ClientVersion is the frontend build SHA at vote time. Free-form;
	// frontend defaults to "dev" in local dev.
	ClientVersion string `dynamodbav:"clientVersion"`
	// SubmittedAt is the RFC 3339 timestamp of the write (server clock).
	SubmittedAt string `dynamodbav:"submittedAt"`
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
	// VerdictSummary is the denormalized projection of admin verdict
	// counts. Recomputed by RecomputeVerdictSummary on every PUT. The
	// row family at PK="VERDICT#..." is canonical; this is a cache.
	VerdictSummary VerdictSummary `dynamodbav:"verdictSummary"`
	// LastDailyDate is the most recent UTC date (YYYY-MM-DD) on which
	// this puzzle was assigned as the daily puzzle. Set atomically by
	// the T=0 daily-puzzle finalize transaction (DP-17, DP-18). Empty
	// or absent on puzzles that have never been a daily. Single date,
	// not an array (D11) — sufficient for cross-feature exclusion.
	LastDailyDate string `dynamodbav:"lastDailyDate,omitempty" json:"lastDailyDate,omitempty"`
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
	// Seed is the RNG seed the generator ran with. Lets cmd/reproduce
	// regenerate the exact same puzzle deterministically (R-06C). Zero
	// is the "unrecorded — generated pre-R-06C" sentinel; new puzzles
	// always carry a non-zero seed.
	Seed int64 `dynamodbav:"seed,omitempty"`
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
// timestamp to the current time in ISO 8601 format. The attribute_exists
// guard ensures the update fails with ErrPuzzleNotFound rather than
// silently upserting an orphan row when the PK/SK pair is attacker-
// supplied or stale.
func (r *PuzzleRepository) MarkServed(ctx context.Context, pk, sk string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression:    aws.String("SET #status = :status, servedAt = :servedAt"),
		ConditionExpression: aws.String("attribute_exists(PK)"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":   &types.AttributeValueMemberS{Value: "served"},
			":servedAt": &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return ErrPuzzleNotFound
		}
		return fmt.Errorf("marking puzzle %s/%s as served: %w", pk, sk, err)
	}

	return nil
}

// UpdateStatus updates a puzzle's status to the given value. See
// MarkServed for the attribute_exists rationale.
func (r *PuzzleRepository) UpdateStatus(ctx context.Context, pk, sk, status string) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression:    aws.String("SET #status = :status"),
		ConditionExpression: aws.String("attribute_exists(PK)"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: status},
		},
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return ErrPuzzleNotFound
		}
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

// buildVerdictPK constructs the verdict row partition key for a
// (size, mode, puzzleID) tuple. See VR-01.
func buildVerdictPK(size int, mode, puzzleID string) string {
	return fmt.Sprintf("VERDICT#%d#%s#%s", size, mode, puzzleID)
}

// buildVerdictSK constructs the verdict row sort key for a given
// rater. The shape is "{raterRole}#{raterId}" so the row family scales
// to multiple rater roles without a re-key. See VR-01.
func buildVerdictSK(raterRole, raterID string) string {
	return fmt.Sprintf("%s#%s", raterRole, raterID)
}

// GetPuzzle returns the puzzle row at PK="{size}#{mode}", SK="{id}".
// Returns (nil, nil) if the row is absent — used by VerdictHandler
// for the 404 check before writing a verdict. See VR-10 / VH-05.
func (r *PuzzleRepository) GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*PuzzleRecord, error) {
	pk := buildPK(size, mode)

	output, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: puzzleID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting puzzle %s/%s: %w", pk, puzzleID, err)
	}

	if output.Item == nil {
		return nil, nil
	}

	var record PuzzleRecord
	if err := attributevalue.UnmarshalMap(output.Item, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling puzzle record: %w", err)
	}
	record.GridSize = size
	record.Mode = mode
	record.ID = puzzleID

	return &record, nil
}

// PutVerdict writes a verdict row to the puzzle-pool table. The PK is
// derived from (GridSize, Mode, PuzzleID); the SK from (RaterRole,
// RaterID). The write is unconditional — re-submission by the same
// rater overwrites the prior row, matching PUT semantics (VR-03).
// SubmittedAt is set on the record before marshaling so callers don't
// have to remember to stamp it.
func (r *PuzzleRepository) PutVerdict(ctx context.Context, v *VerdictRecord) error {
	if v.SubmittedAt == "" {
		v.SubmittedAt = time.Now().UTC().Format(time.RFC3339)
	}

	item, err := attributevalue.MarshalMap(v)
	if err != nil {
		return fmt.Errorf("marshaling verdict record: %w", err)
	}

	pk := buildVerdictPK(v.GridSize, v.Mode, v.PuzzleID)
	sk := buildVerdictSK(v.RaterRole, v.RaterID)
	item["PK"] = &types.AttributeValueMemberS{Value: pk}
	item["SK"] = &types.AttributeValueMemberS{Value: sk}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("putting verdict %s/%s: %w", pk, sk, err)
	}

	return nil
}

// ListVerdictsForPuzzle returns every verdict row for the given
// puzzle. Issues a single Query against the verdict partition. Returns
// an empty slice when no rows exist. See VR-04.
func (r *PuzzleRepository) ListVerdictsForPuzzle(ctx context.Context, size int, mode, puzzleID string) ([]VerdictRecord, error) {
	pk := buildVerdictPK(size, mode, puzzleID)

	output, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("querying verdicts for %s: %w", pk, err)
	}

	verdicts := make([]VerdictRecord, 0, len(output.Items))
	for _, item := range output.Items {
		var record VerdictRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, fmt.Errorf("unmarshaling verdict record: %w", err)
		}

		record.GridSize = size
		record.Mode = mode
		record.PuzzleID = puzzleID

		// Parse role/id from SK (format: "{raterRole}#{raterId}").
		// Use SplitN with n=2 so a Clerk user ID containing '#' wouldn't
		// confuse the split (Clerk IDs don't, but defensively stays
		// correct under future ID schemes).
		if skAttr, ok := item["SK"].(*types.AttributeValueMemberS); ok {
			parts := strings.SplitN(skAttr.Value, "#", 2)
			if len(parts) == 2 {
				record.RaterRole = parts[0]
				record.RaterID = parts[1]
			}
		}

		verdicts = append(verdicts, record)
	}

	return verdicts, nil
}

// RecomputeVerdictSummary reads the verdict row family for one puzzle,
// counts up/down values, and writes the summary to
// PuzzleRecord.verdictSummary via UpdateItem with attribute_exists(PK)
// (so a missing puzzle row produces ErrPuzzleNotFound, not a silent
// upsert). Returns the recomputed summary regardless of whether the
// projection write succeeded — the row family is canonical, the
// summary is a cached projection (VR-05 / VH-09).
func (r *PuzzleRepository) RecomputeVerdictSummary(ctx context.Context, size int, mode, puzzleID string) (VerdictSummary, error) {
	verdicts, err := r.ListVerdictsForPuzzle(ctx, size, mode, puzzleID)
	if err != nil {
		return VerdictSummary{}, fmt.Errorf("listing verdicts for recompute: %w", err)
	}

	summary := VerdictSummary{LastUpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	// Index-based iteration rather than `for _, v := range verdicts` —
	// VerdictRecord is large enough (~144 bytes) that a per-iteration
	// value copy trips gocritic's rangeValCopy linter, and indexing is
	// also marginally faster on hot paths.
	for i := range verdicts {
		switch verdicts[i].Value {
		case "up":
			summary.Up++
		case "down":
			summary.Down++
		}
	}

	pk := buildPK(size, mode)
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: puzzleID},
		},
		UpdateExpression:    aws.String("SET verdictSummary = :summary"),
		ConditionExpression: aws.String("attribute_exists(PK)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":summary": &types.AttributeValueMemberM{
				Value: map[string]types.AttributeValue{
					"up":            &types.AttributeValueMemberN{Value: strconv.Itoa(summary.Up)},
					"down":          &types.AttributeValueMemberN{Value: strconv.Itoa(summary.Down)},
					"lastUpdatedAt": &types.AttributeValueMemberS{Value: summary.LastUpdatedAt},
				},
			},
		},
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return summary, ErrPuzzleNotFound
		}
		return summary, fmt.Errorf("updating verdict summary for %s/%s: %w", pk, puzzleID, err)
	}

	return summary, nil
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
