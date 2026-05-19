// Package repository — daily puzzle data layer.
//
// This file implements the contracts in
// openspec/changes/phase-8-daily-puzzle/specs/daily-puzzle-backend.md
// (DP-01..DP-06, DP-15..DP-18) on top of the existing puzzle-pool table.
//
// Five row shapes are touched:
//   - DAILY#YYYY-MM-DD          — schedule row (one per UTC day)
//   - DAILY-CANDIDATE           — singleton fresh-puzzle slot
//   - 9#standard / SK={puzzleId} — PuzzleRecord (existing) gains lastDailyDate
//   - PLAY#{playerId} / SK=DAILY#YYYY-MM-DD — per-player play history
//   - DAILY-LEADERBOARD#YYYY-MM-DD — sorted leaderboard (signed-in only)
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DailyRecycleWindowDays is the rolling window inside which a puzzle that
// was previously the daily is excluded from candidate selection. 14 days
// is the user-set default; revisit when the approved pool grows or shrinks
// by an order of magnitude.
const DailyRecycleWindowDays = 14

// DailyCandidatePK / DailySingletonSK are the keys used for fixed-shape
// rows (singleton candidate slot, single-row schedule partitions).
// Exported so the service layer can reference them when assembling
// multi-leg transactions without importing private implementation detail.
const (
	DailyCandidatePK = "DAILY-CANDIDATE"
	DailySingletonSK = "<single>"
)

// Sentinel errors surfaced by the daily repository so callers (handlers,
// crons) can distinguish race-loser conditional failures from genuine
// DDB problems. All four follow the pattern of ErrPuzzleNotFound above.
var (
	// ErrCandidateAlreadyExists is returned by PutCandidateIfAbsent when
	// the singleton DAILY-CANDIDATE slot is already occupied. Expected on
	// duplicate T-6h cron firings — caller logs and exits.
	ErrCandidateAlreadyExists = errors.New("daily candidate already exists")
	// ErrScheduleAlreadyFinalized is returned by Service.FinalizeDaily in
	// internal/service/daily when today's DAILY#date row already exists.
	// Expected on duplicate T=0 cron firings or when the sync fallback
	// races a cron — caller falls back to GetSchedule and reads the
	// winner's row.
	ErrScheduleAlreadyFinalized = errors.New("daily schedule already finalized")
	// ErrPlayNotInStartedState is returned by Service.SubmitPlay in
	// internal/service/daily when the PLAY row is missing or its outcome
	// is not "started" (e.g. a duplicate submission of an already-solved
	// row). Maps to HTTP 409.
	ErrPlayNotInStartedState = errors.New("daily play row not in started state")
)

// Schedule counter field constants — the only legal values for the
// counter field updated via the daily service's WriteTransaction legs.
// Defined as constants so callers can't fat-finger an arbitrary
// attribute name and drift the schema.
const (
	ScheduleCounterStarted = "started"
	ScheduleCounterSolved  = "solved"
)

// PlayOutcomeStarted / PlayOutcomeSolved are the only legal values for
// PlayRecord.Outcome. No "skipped" on dailies — the daily/packs UI does
// not surface a Skip action.
const (
	PlayOutcomeStarted = "started"
	PlayOutcomeSolved  = "solved"
)

// ScheduleRecord is the schedule row shape (PK=DAILY#YYYY-MM-DD).
// One row per UTC day. Counters are atomically updated via the daily
// service's WriteTransaction; assignedAt is the cron / sync-fallback's
// stamp and is never overwritten.
type ScheduleRecord struct {
	// Date is the UTC date (YYYY-MM-DD), embedded in the PK.
	Date string `dynamodbav:"-"`
	// PuzzleID is the puzzle UUID this date resolves to.
	PuzzleID string `dynamodbav:"puzzleId"`
	// AssignedAt is the RFC 3339 timestamp the schedule row was created.
	AssignedAt string `dynamodbav:"assignedAt"`
	// SourcePartition is the puzzle-pool partition the puzzle was drawn
	// from (e.g. "9#standard"). Future-proofs combo rotation without
	// locking it in.
	SourcePartition string `dynamodbav:"sourcePartition"`
	// Counters tracks per-day plays. Updated atomically by the daily
	// service. Powers recycle decisions and telemetry.
	Counters ScheduleCounters `dynamodbav:"counters"`
}

// ScheduleCounters is the {started, solved} pair embedded in the
// schedule row. Both default to 0; updated atomically by the daily
// service via UpdateItem `ADD counters.<field> :one`.
type ScheduleCounters struct {
	Started int64 `dynamodbav:"started" json:"started"`
	Solved  int64 `dynamodbav:"solved" json:"solved"`
}

// CandidateRecord is the singleton candidate-slot row
// (PK=DAILY-CANDIDATE). Persists across days when a recycle leaves it
// unconsumed.
type CandidateRecord struct {
	// PuzzleID is the candidate puzzle UUID.
	PuzzleID string `dynamodbav:"puzzleId"`
	// QueuedAt is the RFC 3339 timestamp the candidate was queued.
	QueuedAt string `dynamodbav:"queuedAt"`
	// SourcePartition is the puzzle-pool partition the candidate was
	// drawn from. Carried through to the schedule row on confirm.
	SourcePartition string `dynamodbav:"sourcePartition"`
}

// PlayRecord is the per-player daily play row
// (PK=PLAY#{playerId}, SK=DAILY#YYYY-MM-DD). `playerId` is `userId` for
// signed-in players, `deviceId` for anonymous.
type PlayRecord struct {
	// PlayerID is `userId` for signed-in or `deviceId` for anonymous.
	// Embedded in the PK; not stored as a separate attribute.
	PlayerID string `dynamodbav:"-"`
	// Date is the UTC date (YYYY-MM-DD), embedded in the SK.
	Date string `dynamodbav:"-"`
	// Outcome is "started" on first GET and "solved" on submission.
	// "skipped" is intentionally not used on dailies.
	Outcome string `dynamodbav:"outcome"`
	// AssignedAt is the server-stamped first-GET timestamp. Never
	// overwritten — refresh, second-device, second-GET all return this
	// (anti-cheat: assignedAt is set once, never overwritten).
	AssignedAt string `dynamodbav:"assignedAt"`
	// SubmittedAt is the RFC 3339 timestamp of the solve submission.
	// Empty until outcome=solved.
	SubmittedAt string `dynamodbav:"submittedAt,omitempty"`
	// PuzzleID is the puzzle UUID this row plays. Carried for handler
	// convenience so we don't have to re-resolve the schedule row on
	// every GET.
	PuzzleID string `dynamodbav:"puzzleId"`
	// ServerElapsedMs is `submittedAt - assignedAt` in milliseconds.
	// Source of truth for any future ranking surface.
	ServerElapsedMs int64 `dynamodbav:"serverElapsedMs,omitempty"`
	// ClientClaimedMs is the player-claimed playTimeMs. Captured for
	// telemetry only — never authoritative.
	ClientClaimedMs int64 `dynamodbav:"clientClaimedMs,omitempty"`
}

// SubmitInput bundles the fields the handler captures from a valid
// POST /api/daily/{date}/result request and forwards to
// Service.SubmitPlay in internal/service/daily. Solution validation
// is the handler's job — by the time we get here the
// submission is structurally and semantically valid.
type SubmitInput struct {
	// PuzzleID is the schedule row's puzzle (used for the leaderboard
	// row's correlation/debug — not strictly required for ranking).
	PuzzleID string
	// AssignedAt is the PLAY row's authoritative start timestamp
	// (RFC 3339). Used both to compute `serverElapsedMs` and to derive
	// the play-origin date for cross-midnight submissions.
	AssignedAt time.Time
	// SubmittedAt is when the submission landed on the server.
	SubmittedAt time.Time
	// ClientMs is the player-claimed `playTimeMs` from the body.
	// Telemetry only.
	ClientMs int64
	// IsAnonymous is true for `deviceId`-keyed players. When true,
	// Service.SubmitPlay skips the leaderboard leg for anonymous players.
	IsAnonymous bool
	// UserID is the Clerk user ID, used as the leaderboard SK suffix.
	// Ignored when IsAnonymous=true.
	UserID string
}

// BuildDailySchedulePK constructs DAILY#YYYY-MM-DD. Exported so the
// service layer can build keys for transaction legs without re-encoding
// the prefix.
func BuildDailySchedulePK(date string) string {
	return "DAILY#" + date
}

// BuildDailyLeaderboardPK constructs DAILY-LEADERBOARD#YYYY-MM-DD.
// Exported so the service layer can build keys for transaction legs
// without re-encoding the prefix.
func BuildDailyLeaderboardPK(date string) string {
	return "DAILY-LEADERBOARD#" + date
}

// BuildPlayPK constructs PLAY#{playerId}. `playerId` is opaque — it can
// be a Clerk userID or a deviceId; the prefix scopes both into the same
// row family without collision risk. Exported so the service layer can
// assemble transaction legs.
func BuildPlayPK(playerID string) string {
	return "PLAY#" + playerID
}

// BuildPlaySK constructs DAILY#YYYY-MM-DD as the PLAY row sort key.
// The shape mirrors the schedule PK on purpose so future per-player
// surfaces (packs, etc.) can extend with sibling SK prefixes. Exported
// so the service layer can assemble transaction legs.
func BuildPlaySK(date string) string {
	return "DAILY#" + date
}

// BuildLeaderboardSK constructs the leaderboard SK as
// {paddedMs:8d}#{userId}. Eight digits → max ~27.7 hours, ample
// headroom for any legitimate solve time. Ascending lexicographic Query
// returns fastest first. Exported so the service layer can assemble
// transaction legs.
func BuildLeaderboardSK(elapsedMs int64, userID string) string {
	return fmt.Sprintf("%08d#%s", elapsedMs, userID)
}

// GetSchedule reads the schedule row for date. Returns (nil, nil) when
// absent — caller decides whether to engage the sync fallback or 404.
func (r *PuzzleRepository) GetSchedule(ctx context.Context, date string) (*ScheduleRecord, error) {
	output, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: BuildDailySchedulePK(date)},
			"SK": &types.AttributeValueMemberS{Value: DailySingletonSK},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting daily schedule for %s: %w", date, err)
	}
	if output.Item == nil {
		return nil, nil
	}

	var record ScheduleRecord
	if err := attributevalue.UnmarshalMap(output.Item, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling schedule record: %w", err)
	}
	record.Date = date
	return &record, nil
}

// GetCandidate reads the singleton candidate slot. Returns (nil, nil)
// when empty — caller (T=0 cron, sync fallback) treats empty as a
// recycle trigger.
func (r *PuzzleRepository) GetCandidate(ctx context.Context) (*CandidateRecord, error) {
	output, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: DailyCandidatePK},
			"SK": &types.AttributeValueMemberS{Value: DailySingletonSK},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting daily candidate: %w", err)
	}
	if output.Item == nil {
		return nil, nil
	}

	var record CandidateRecord
	if err := attributevalue.UnmarshalMap(output.Item, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling candidate record: %w", err)
	}
	return &record, nil
}

// PutCandidateIfAbsent writes a fresh candidate puzzle into the
// singleton slot. Conditional on the slot being empty (race
// guard) — duplicate T-6h cron firings see ErrCandidateAlreadyExists
// and exit cleanly. The same conditional handles the case where a
// recycle left an older candidate in place: T-6h would log+exit, and
// the candidate ages naturally until T=0 consumes or recycles it.
func (r *PuzzleRepository) PutCandidateIfAbsent(ctx context.Context, puzzleID, sourcePartition string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	item := map[string]types.AttributeValue{
		"PK":              &types.AttributeValueMemberS{Value: DailyCandidatePK},
		"SK":              &types.AttributeValueMemberS{Value: DailySingletonSK},
		"puzzleId":        &types.AttributeValueMemberS{Value: puzzleID},
		"queuedAt":        &types.AttributeValueMemberS{Value: now},
		"sourcePartition": &types.AttributeValueMemberS{Value: sourcePartition},
	}

	_, err := r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return ErrCandidateAlreadyExists
		}
		return fmt.Errorf("putting daily candidate %s: %w", puzzleID, err)
	}
	return nil
}

// ListApprovedPool returns approved puzzles eligible for daily
// assignment, scoped to (size, mode). Approval gate is
// `verdictSummary.up >= 1 AND verdictSummary.down == 0`.
// When excludeRecentlyDailied=true, also rejects puzzles whose
// `lastDailyDate` falls within the DailyRecycleWindowDays-day
// rolling window relative to `now`.
//
// Per CLAUDE.md lesson 10, this method does NOT pass DDB Limit
// alongside FilterExpression — DDB's Limit applies pre-filter and
// can return zero results from a non-empty pool. The approved-pool
// partitions are small (<60 items in the steady state), so a full
// partition scan is safe.
func (r *PuzzleRepository) ListApprovedPool(ctx context.Context, size int, mode string, excludeRecentlyDailied bool, now time.Time) ([]PuzzleRecord, error) {
	pk := buildPK(size, mode)

	filter := "verdictSummary.up >= :one AND verdictSummary.down = :zero"
	values := map[string]types.AttributeValue{
		":pk":   &types.AttributeValueMemberS{Value: pk},
		":one":  &types.AttributeValueMemberN{Value: "1"},
		":zero": &types.AttributeValueMemberN{Value: "0"},
	}
	if excludeRecentlyDailied {
		cutoff := now.UTC().AddDate(0, 0, -DailyRecycleWindowDays).Format("2006-01-02")
		filter += " AND (attribute_not_exists(lastDailyDate) OR lastDailyDate < :cutoff)"
		values[":cutoff"] = &types.AttributeValueMemberS{Value: cutoff}
	}

	output, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    aws.String("PK = :pk"),
		FilterExpression:          aws.String(filter),
		ExpressionAttributeValues: values,
	})
	if err != nil {
		return nil, fmt.Errorf("querying approved pool for %s: %w", pk, err)
	}

	records := make([]PuzzleRecord, 0, len(output.Items))
	for _, item := range output.Items {
		var record PuzzleRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, fmt.Errorf("unmarshaling approved-pool record: %w", err)
		}
		record.GridSize = size
		record.Mode = mode
		if skAttr, ok := item["SK"].(*types.AttributeValueMemberS); ok {
			record.ID = skAttr.Value
		}
		records = append(records, record)
	}
	return records, nil
}

// GetPlay reads the per-player play row for (playerId, date). Returns
// (nil, nil) when absent.
func (r *PuzzleRepository) GetPlay(ctx context.Context, playerID, date string) (*PlayRecord, error) {
	output, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: BuildPlayPK(playerID)},
			"SK": &types.AttributeValueMemberS{Value: BuildPlaySK(date)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting daily play %s/%s: %w", playerID, date, err)
	}
	if output.Item == nil {
		return nil, nil
	}

	var record PlayRecord
	if err := attributevalue.UnmarshalMap(output.Item, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling play record: %w", err)
	}
	record.PlayerID = playerID
	record.Date = date
	return &record, nil
}

// FinalizeMode discriminates the two T=0 finalize paths (design §4):
// confirm consumes the singleton candidate slot; recycle reuses
// yesterday's puzzle and skips the candidate delete entirely.
type FinalizeMode string

// FinalizeModeConfirm / FinalizeModeRecycle are the only legal values
// for FinalizeMode. Any other value is rejected by Service.FinalizeDaily
// in internal/service/daily before any DDB call so a typo can't
// silently flow through.
const (
	FinalizeModeConfirm FinalizeMode = "confirm"
	FinalizeModeRecycle FinalizeMode = "recycle"
)

// IsConditionalCheckFailureOnLeg returns true when err is a
// TransactionCanceledException whose CancellationReasons indicate the
// leg at legIndex failed its ConditionExpression. DDB's Go SDK v2
// returns this as a typed error with a CancellationReasons slice whose
// indices align 1:1 with the input TransactItems. Exported so the
// service layer can inspect transaction errors without re-implementing
// the detection logic.
func IsConditionalCheckFailureOnLeg(err error, legIndex int) bool {
	var tce *types.TransactionCanceledException
	if !errors.As(err, &tce) {
		return false
	}
	if legIndex < 0 || legIndex >= len(tce.CancellationReasons) {
		return false
	}
	reason := tce.CancellationReasons[legIndex]
	return reason.Code != nil && strings.EqualFold(*reason.Code, "ConditionalCheckFailed")
}

// WriteTransaction executes the given transact items as a single
// DynamoDB TransactWriteItems call. Used by the daily application
// service to assemble multi-row atomic writes whose orchestration
// (which legs to include, when) lives in service/ per the
// architecture rule. No DDB error translation here — callers use
// IsConditionalCheckFailureOnLeg on the returned error.
func (r *PuzzleRepository) WriteTransaction(ctx context.Context, items []types.TransactWriteItem) error {
	_, err := r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: items,
	})
	return err
}

// LeaderboardRank returns the player's 1-based rank on the daily
// leaderboard for `date`. Rank N means N-1 strictly faster times are
// ahead of the player.
//
// Implementation: single DDB Query against PK = DAILY-LEADERBOARD#date
// with KeyConditionExpression "PK = :pk AND SK <= :playerSK",
// Select=COUNT. The leaderboard SK is padded so lex order matches
// numeric order — see buildLeaderboardSK.
//
// Pagination: NOT handled. The Phase 8 leaderboard is bounded well
// under DDB's 1MB Query result cap (worst case ~10K entries × ~30
// bytes of metadata per entry). If Phase 9 adds growth, swap in a
// paginator here.
//
// Returns (1, nil) when the player's row is the fastest. On any DDB
// error returns (0, wrapped-error).
func (r *PuzzleRepository) LeaderboardRank(
	ctx context.Context,
	date string,
	elapsedMs int64,
	userID string,
) (int, error) {
	playerSK := BuildLeaderboardSK(elapsedMs, userID)
	output, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("#pk = :pk AND #sk <= :playerSK"),
		ExpressionAttributeNames: map[string]string{
			"#pk": "PK",
			"#sk": "SK",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":       &types.AttributeValueMemberS{Value: BuildDailyLeaderboardPK(date)},
			":playerSK": &types.AttributeValueMemberS{Value: playerSK},
		},
		Select: types.SelectCount,
	})
	if err != nil {
		return 0, fmt.Errorf("leaderboard rank for %s player=%s: %w", date, userID, err)
	}
	return int(output.Count), nil
}
