package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// packPK is the shared partition key for every pack row. Packs live in
// one partition (PK="PACK", SK="{slug}"), mirroring the CONFIG family,
// so ListPacks is a single Query and slug uniqueness is enforced by an
// attribute_not_exists(PK) guard on the composite key.
const packPK = "PACK"

// ErrPackAlreadyExists is returned by PutPack when a pack with the same
// slug already exists. Callers map it to a 409.
var ErrPackAlreadyExists = errors.New("pack already exists")

// ErrPackNotFound is returned by UpdatePack and DeletePack when the
// addressed slug has no existing row. Callers map it to a 404.
var ErrPackNotFound = errors.New("pack not found")

// PackRecord represents a curated puzzle pack stored in the puzzle-pool
// DynamoDB table. Pack rows use PK="PACK" and SK="{slug}". The Slug is
// the canonical identity and is immutable after creation.
type PackRecord struct {
	// Slug is the canonical pack identity, stored as the sort key.
	Slug string `dynamodbav:"-"`
	// Name is the human-readable pack title.
	Name string `dynamodbav:"name"`
	// Size is the N dimension shared by every puzzle in the pack.
	Size int `dynamodbav:"size"`
	// Mode is the game mode ("standard"/"double") shared by every puzzle.
	Mode string `dynamodbav:"mode"`
	// Published toggles public visibility. Drafts are admin-only.
	Published bool `dynamodbav:"published"`
	// PuzzleIDs is the ordered list of bare puzzle ids within the pack's
	// size+mode partition.
	PuzzleIDs []string `dynamodbav:"puzzleIds"`
	// CreatedAt is the RFC 3339 timestamp of creation.
	CreatedAt string `dynamodbav:"createdAt"`
	// UpdatedAt is the RFC 3339 timestamp of the most recent write.
	UpdatedAt string `dynamodbav:"updatedAt"`
}

// PutPack creates a new pack row only if no pack with the same slug
// already exists. Returns ErrPackAlreadyExists when the slug is taken.
func (r *PuzzleRepository) PutPack(ctx context.Context, pack *PackRecord) error {
	item, err := attributevalue.MarshalMap(pack)
	if err != nil {
		return fmt.Errorf("marshaling pack record: %w", err)
	}
	item["PK"] = &types.AttributeValueMemberS{Value: packPK}
	item["SK"] = &types.AttributeValueMemberS{Value: pack.Slug}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return ErrPackAlreadyExists
		}
		return fmt.Errorf("creating pack %s: %w", pack.Slug, err)
	}
	return nil
}

// GetPack returns the pack with the given slug. Returns (nil, nil) when
// no matching row exists.
func (r *PuzzleRepository) GetPack(ctx context.Context, slug string) (*PackRecord, error) {
	output, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: packPK},
			"SK": &types.AttributeValueMemberS{Value: slug},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting pack %s: %w", slug, err)
	}
	if output.Item == nil {
		return nil, nil
	}

	record, err := unmarshalPack(output.Item)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// ListPacks returns every pack (drafts included), querying the shared
// PACK partition in a single Query.
func (r *PuzzleRepository) ListPacks(ctx context.Context) ([]PackRecord, error) {
	output, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: packPK},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("querying packs: %w", err)
	}

	packs := make([]PackRecord, 0, len(output.Items))
	for _, item := range output.Items {
		record, err := unmarshalPack(item)
		if err != nil {
			return nil, err
		}
		packs = append(packs, record)
	}
	return packs, nil
}

// UpdatePack updates the mutable fields (name, puzzleIds, published,
// updatedAt) of an existing pack. The attribute_exists(PK) guard makes
// the update fail with ErrPackNotFound rather than silently upserting a
// new row. Slug, size, and mode are immutable and not touched.
func (r *PuzzleRepository) UpdatePack(ctx context.Context, pack *PackRecord) error {
	puzzleIDs, err := attributevalue.MarshalList(pack.PuzzleIDs)
	if err != nil {
		return fmt.Errorf("marshaling pack puzzleIds: %w", err)
	}

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: packPK},
			"SK": &types.AttributeValueMemberS{Value: pack.Slug},
		},
		UpdateExpression:    aws.String("SET #name = :name, puzzleIds = :puzzleIds, published = :published, updatedAt = :updatedAt"),
		ConditionExpression: aws.String("attribute_exists(PK)"),
		ExpressionAttributeNames: map[string]string{
			"#name": "name",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name":      &types.AttributeValueMemberS{Value: pack.Name},
			":puzzleIds": &types.AttributeValueMemberL{Value: puzzleIDs},
			":published": &types.AttributeValueMemberBOOL{Value: pack.Published},
			":updatedAt": &types.AttributeValueMemberS{Value: pack.UpdatedAt},
		},
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return ErrPackNotFound
		}
		return fmt.Errorf("updating pack %s: %w", pack.Slug, err)
	}
	return nil
}

// DeletePack removes the pack with the given slug. The attribute_exists
// guard makes the delete fail with ErrPackNotFound when the slug is
// absent rather than reporting a no-op success.
func (r *PuzzleRepository) DeletePack(ctx context.Context, slug string) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: packPK},
			"SK": &types.AttributeValueMemberS{Value: slug},
		},
		ConditionExpression: aws.String("attribute_exists(PK)"),
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return ErrPackNotFound
		}
		return fmt.Errorf("deleting pack %s: %w", slug, err)
	}
	return nil
}

// unmarshalPack decodes a pack item and lifts the slug out of the SK.
func unmarshalPack(item map[string]types.AttributeValue) (PackRecord, error) {
	var record PackRecord
	if err := attributevalue.UnmarshalMap(item, &record); err != nil {
		return PackRecord{}, fmt.Errorf("unmarshaling pack record: %w", err)
	}
	if skAttr, ok := item["SK"].(*types.AttributeValueMemberS); ok {
		record.Slug = skAttr.Value
	}
	return record, nil
}
