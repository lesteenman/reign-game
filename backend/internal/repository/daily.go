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
	"strconv"
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

// dailyCandidatePK / dailyCandidateSK / dailySingletonSK are the keys
// used for fixed-shape rows (singleton candidate slot, single-row schedule
// partitions). Centralising them avoids string-literal drift across
// methods.
const (
	dailyCandidatePK = "DAILY-CANDIDATE"
	dailySingletonSK = "<single>"
)

// Sentinel errors surfaced by the daily repository so callers (handlers,
// crons) can distinguish race-loser conditional failures from genuine
// DDB problems. All four follow the pattern of ErrPuzzleNotFound above.
var (
	// ErrCandidateAlreadyExists is returned by PutCandidateIfAbsent when
	// the singleton DAILY-CANDIDATE slot is already occupied. Expected on
	// duplicate T-6h cron firings — caller logs and exits.
	ErrCandidateAlreadyExists = errors.New("daily candidate already exists")
	// ErrScheduleAlreadyFinalized is returned by FinalizeSchedule when
	// today's DAILY#date row already exists. Expected on duplicate T=0
	// cron firings or when the sync fallback races a cron — caller falls
	// back to GetSchedule and reads the winner's row.
	ErrScheduleAlreadyFinalized = errors.New("daily schedule already finalized")
	// ErrPlayAlreadyExists is returned by PutPlayStartedIfAbsent when a
	// PLAY row exists for (playerId, date). Caller follows up with GetPlay
	// to read the existing assignedAt — never overwrite (DP-19).
	ErrPlayAlreadyExists = errors.New("daily play row already exists")
	// ErrPlayNotInStartedState is returned by SubmitPlayTransactionally
	// when the PLAY row is missing or its outcome is not "started" (e.g.
	// a duplicate submission of an already-solved row). Maps to HTTP 409
	// per DP-12.
	ErrPlayNotInStartedState = errors.New("daily play row not in started state")
)

// Schedule counter field constants — the only legal values for the
// `field` argument to IncrementScheduleCounter (DP-06). Defined as
// constants so callers can't fat-finger an arbitrary attribute name and
// drift the schema.
const (
	ScheduleCounterStarted = "started"
	ScheduleCounterSolved  = "solved"
)

// PlayOutcomeStarted / PlayOutcomeSolved are the only legal values for
// PlayRecord.Outcome (D9). No "skipped" on dailies — the daily/packs UI
// does not surface a Skip action.
const (
	PlayOutcomeStarted = "started"
	PlayOutcomeSolved  = "solved"
)

// ScheduleRecord is the schedule row shape (PK=DAILY#YYYY-MM-DD).
// One row per UTC day. Counters are atomically updated via
// IncrementScheduleCounter; assignedAt is the cron / sync-fallback's
// stamp and is never overwritten (DP-01).
type ScheduleRecord struct {
	// Date is the UTC date (YYYY-MM-DD), embedded in the PK.
	Date string `dynamodbav:"-"`
	// PuzzleID is the puzzle UUID this date resolves to.
	PuzzleID string `dynamodbav:"puzzleId"`
	// AssignedAt is the RFC 3339 timestamp the schedule row was created.
	AssignedAt string `dynamodbav:"assignedAt"`
	// SourcePartition is the puzzle-pool partition the puzzle was drawn
	// from (e.g. "9#standard"). Future-proofs combo rotation without
	// locking it in (D12).
	SourcePartition string `dynamodbav:"sourcePartition"`
	// Counters tracks per-day plays. Updated atomically by
	// IncrementScheduleCounter (DP-06). Powers recycle decisions and
	// telemetry (D10).
	Counters ScheduleCounters `dynamodbav:"counters"`
}

// ScheduleCounters is the {started, solved} pair embedded in the
// schedule row. Both default to 0; updated atomically via UpdateItem
// `ADD counters.<field> :one`.
type ScheduleCounters struct {
	Started int64 `dynamodbav:"started" json:"started"`
	Solved  int64 `dynamodbav:"solved" json:"solved"`
}

// CandidateRecord is the singleton candidate-slot row
// (PK=DAILY-CANDIDATE). Persists across days when a recycle leaves it
// unconsumed (D7, DP-02).
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
// signed-in players, `deviceId` for anonymous (DP-10).
type PlayRecord struct {
	// PlayerID is `userId` for signed-in or `deviceId` for anonymous.
	// Embedded in the PK; not stored as a separate attribute.
	PlayerID string `dynamodbav:"-"`
	// Date is the UTC date (YYYY-MM-DD), embedded in the SK.
	Date string `dynamodbav:"-"`
	// Outcome is "started" on first GET and "solved" on submission.
	// "skipped" is intentionally not used on dailies (D9).
	Outcome string `dynamodbav:"outcome"`
	// AssignedAt is the server-stamped first-GET timestamp. Never
	// overwritten — refresh, second-device, second-GET all return this
	// (DP-19, anti-cheat).
	AssignedAt string `dynamodbav:"assignedAt"`
	// SubmittedAt is the RFC 3339 timestamp of the solve submission.
	// Empty until outcome=solved.
	SubmittedAt string `dynamodbav:"submittedAt,omitempty"`
	// PuzzleID is the puzzle UUID this row plays. Carried for handler
	// convenience so we don't have to re-resolve the schedule row on
	// every GET.
	PuzzleID string `dynamodbav:"puzzleId"`
	// ServerElapsedMs is `submittedAt - assignedAt` in milliseconds.
	// Source of truth for any future ranking surface (DP-20).
	ServerElapsedMs int64 `dynamodbav:"serverElapsedMs,omitempty"`
	// ClientClaimedMs is the player-claimed playTimeMs. Captured for
	// telemetry only — never authoritative (DP-20).
	ClientClaimedMs int64 `dynamodbav:"clientClaimedMs,omitempty"`
}

// SubmitInput bundles the fields the handler captures from a valid
// POST /api/daily/{date}/result request and forwards to
// SubmitPlayTransactionally. Solution validation is the handler's job
// (DP-11) — by the time we get here the submission is structurally and
// semantically valid.
type SubmitInput struct {
	// PuzzleID is the schedule row's puzzle (used for the leaderboard
	// row's correlation/debug — not strictly required for ranking).
	PuzzleID string
	// AssignedAt is the PLAY row's authoritative start timestamp
	// (RFC 3339). Used both to compute `serverElapsedMs` and to derive
	// the play-origin date for cross-midnight submissions (DP-13).
	AssignedAt time.Time
	// SubmittedAt is when the submission landed on the server.
	SubmittedAt time.Time
	// ClientMs is the player-claimed `playTimeMs` from the body.
	// Telemetry only.
	ClientMs int64
	// IsAnonymous is true for `deviceId`-keyed players. When true,
	// SubmitPlayTransactionally skips the leaderboard leg (D13).
	IsAnonymous bool
	// UserID is the Clerk user ID, used as the leaderboard SK suffix.
	// Ignored when IsAnonymous=true.
	UserID string
}

// buildDailySchedulePK constructs DAILY#YYYY-MM-DD.
func buildDailySchedulePK(date string) string {
	return "DAILY#" + date
}

// buildDailyLeaderboardPK constructs DAILY-LEADERBOARD#YYYY-MM-DD.
func buildDailyLeaderboardPK(date string) string {
	return "DAILY-LEADERBOARD#" + date
}

// buildPlayPK constructs PLAY#{playerId}. `playerId` is opaque — it can
// be a Clerk userID or a deviceId; the prefix scopes both into the same
// row family without collision risk.
func buildPlayPK(playerID string) string {
	return "PLAY#" + playerID
}

// buildPlaySK constructs DAILY#YYYY-MM-DD as the PLAY row sort key.
// The shape mirrors the schedule PK on purpose so future per-player
// surfaces (packs, etc.) can extend with sibling SK prefixes.
func buildPlaySK(date string) string {
	return "DAILY#" + date
}

// buildLeaderboardSK constructs the leaderboard SK as
// {paddedMs:8d}#{userId}. Eight digits → max ~27.7 hours, ample
// headroom for any legitimate solve time. Ascending lexicographic Query
// returns fastest first.
func buildLeaderboardSK(elapsedMs int64, userID string) string {
	return fmt.Sprintf("%08d#%s", elapsedMs, userID)
}

// GetSchedule reads the schedule row for date. Returns (nil, nil) when
// absent — caller decides whether to engage the sync fallback (DP-05)
// or 404.
func (r *PuzzleRepository) GetSchedule(ctx context.Context, date string) (*ScheduleRecord, error) {
	output, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: buildDailySchedulePK(date)},
			"SK": &types.AttributeValueMemberS{Value: dailySingletonSK},
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
// recycle trigger (DP-04).
func (r *PuzzleRepository) GetCandidate(ctx context.Context) (*CandidateRecord, error) {
	output, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: dailyCandidatePK},
			"SK": &types.AttributeValueMemberS{Value: dailySingletonSK},
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
// singleton slot. Conditional on the slot being empty (DP-03's race
// guard) — duplicate T-6h cron firings see ErrCandidateAlreadyExists
// and exit cleanly. The same conditional handles the case where a
// recycle left an older candidate in place: T-6h would log+exit, and
// the candidate ages naturally until T=0 consumes or recycles it.
func (r *PuzzleRepository) PutCandidateIfAbsent(ctx context.Context, puzzleID, sourcePartition string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	item := map[string]types.AttributeValue{
		"PK":              &types.AttributeValueMemberS{Value: dailyCandidatePK},
		"SK":              &types.AttributeValueMemberS{Value: dailySingletonSK},
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

// DeleteCandidate clears the singleton slot. Used by the T=0 confirm
// path after the schedule row is finalized (DP-04). Unconditional —
// idempotent against duplicate calls because a missing row deletes
// cleanly.
func (r *PuzzleRepository) DeleteCandidate(ctx context.Context) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: dailyCandidatePK},
			"SK": &types.AttributeValueMemberS{Value: dailySingletonSK},
		},
	})
	if err != nil {
		return fmt.Errorf("deleting daily candidate: %w", err)
	}
	return nil
}

// FinalizeSchedule writes today's schedule row. Conditional on the
// row not yet existing — duplicate T=0 cron firings AND sync-fallback
// races resolve via ErrScheduleAlreadyFinalized (DP-04, DP-05).
// Counters are initialized to zero. Caller is responsible for the
// PuzzleRecord lastDailyDate write (separate transactional concern;
// crons compose the two).
func (r *PuzzleRepository) FinalizeSchedule(ctx context.Context, date, puzzleID, sourcePartition string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	item := map[string]types.AttributeValue{
		"PK":              &types.AttributeValueMemberS{Value: buildDailySchedulePK(date)},
		"SK":              &types.AttributeValueMemberS{Value: dailySingletonSK},
		"puzzleId":        &types.AttributeValueMemberS{Value: puzzleID},
		"assignedAt":      &types.AttributeValueMemberS{Value: now},
		"sourcePartition": &types.AttributeValueMemberS{Value: sourcePartition},
		"counters": &types.AttributeValueMemberM{
			Value: map[string]types.AttributeValue{
				"started": &types.AttributeValueMemberN{Value: "0"},
				"solved":  &types.AttributeValueMemberN{Value: "0"},
			},
		},
	}

	_, err := r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return ErrScheduleAlreadyFinalized
		}
		return fmt.Errorf("finalizing daily schedule for %s: %w", date, err)
	}
	return nil
}

// IncrementScheduleCounter atomically increments counters.{started|solved}
// on the schedule row by `delta` (DP-06). `field` must be one of
// ScheduleCounterStarted / ScheduleCounterSolved — any other value is
// rejected to keep the schema honest.
func (r *PuzzleRepository) IncrementScheduleCounter(ctx context.Context, date, field string, delta int64) error {
	if field != ScheduleCounterStarted && field != ScheduleCounterSolved {
		return fmt.Errorf("invalid schedule counter field %q", field)
	}

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: buildDailySchedulePK(date)},
			"SK": &types.AttributeValueMemberS{Value: dailySingletonSK},
		},
		UpdateExpression: aws.String("ADD #counters.#field :delta"),
		ExpressionAttributeNames: map[string]string{
			"#counters": "counters",
			"#field":    field,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":delta": &types.AttributeValueMemberN{Value: strconv.FormatInt(delta, 10)},
		},
	})
	if err != nil {
		return fmt.Errorf("incrementing schedule counter %s.%s: %w", date, field, err)
	}
	return nil
}

// ListApprovedPool returns approved puzzles eligible for daily
// assignment, scoped to (size, mode). Approval gate is
// `verdictSummary.up >= 1 AND verdictSummary.down == 0` (DP-15).
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

// MarkPuzzleAsDailyOn sets PuzzleRecord.lastDailyDate to `date`. Idempotent
// against same-date repeats and refuses to overwrite a newer date —
// guards against late-arriving cron writes / retries clobbering a
// fresher value (DP-18). Returns ErrPuzzleNotFound when the underlying
// puzzle row is absent so a stale puzzleID does not silently upsert.
func (r *PuzzleRepository) MarkPuzzleAsDailyOn(ctx context.Context, size int, mode, puzzleID, date string) error {
	pk := buildPK(size, mode)

	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: puzzleID},
		},
		UpdateExpression:    aws.String("SET lastDailyDate = :date"),
		ConditionExpression: aws.String("attribute_exists(PK) AND (attribute_not_exists(lastDailyDate) OR lastDailyDate <= :date)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":date": &types.AttributeValueMemberS{Value: date},
		},
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			// Two reasons: (a) row missing → 404; (b) existing
			// lastDailyDate is newer than ours → silent no-op (idempotent
			// from the caller's view).
			//
			// We cannot distinguish without an extra GetItem, so we
			// optimistically check existence: if it exists, treat as
			// silent no-op (newer-date case); else surface ErrPuzzleNotFound.
			existing, getErr := r.GetPuzzle(ctx, size, mode, puzzleID)
			if getErr != nil {
				return fmt.Errorf("verifying puzzle existence after conditional fail %s/%s: %w", pk, puzzleID, getErr)
			}
			if existing == nil {
				return ErrPuzzleNotFound
			}
			return nil
		}
		return fmt.Errorf("marking puzzle %s/%s as daily on %s: %w", pk, puzzleID, date, err)
	}
	return nil
}

// GetPlay reads the per-player play row for (playerId, date). Returns
// (nil, nil) when absent — caller branches to PutPlayStartedIfAbsent on
// first GET (DP-08).
func (r *PuzzleRepository) GetPlay(ctx context.Context, playerID, date string) (*PlayRecord, error) {
	output, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: buildPlayPK(playerID)},
			"SK": &types.AttributeValueMemberS{Value: buildPlaySK(date)},
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

// PutPlayStartedIfAbsent creates a fresh PLAY row with outcome=started
// and the server-stamped assignedAt. Conditional on absence — DP-19's
// anti-cheat invariant says assignedAt is set once and never overwritten,
// so a duplicate first-GET race must NOT update the existing row.
// Caller responds to ErrPlayAlreadyExists by calling GetPlay and using
// the winner's assignedAt.
func (r *PuzzleRepository) PutPlayStartedIfAbsent(ctx context.Context, playerID, date, puzzleID string, assignedAt time.Time) error {
	item := map[string]types.AttributeValue{
		"PK":         &types.AttributeValueMemberS{Value: buildPlayPK(playerID)},
		"SK":         &types.AttributeValueMemberS{Value: buildPlaySK(date)},
		"outcome":    &types.AttributeValueMemberS{Value: PlayOutcomeStarted},
		"assignedAt": &types.AttributeValueMemberS{Value: assignedAt.UTC().Format(time.RFC3339)},
		"puzzleId":   &types.AttributeValueMemberS{Value: puzzleID},
	}

	_, err := r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			return ErrPlayAlreadyExists
		}
		return fmt.Errorf("putting daily play %s/%s: %w", playerID, date, err)
	}
	return nil
}

// SubmitPlayTransactionally commits a daily-puzzle solve via a single
// TransactWriteItems with up to three legs (DP-12, D14):
//
//  1. UpdateItem PLAY → outcome=solved, submittedAt, serverElapsedMs,
//     clientClaimedMs. Conditional on outcome=started for idempotency —
//     a duplicate submission produces ErrPlayNotInStartedState (caller
//     maps to HTTP 409, no double-count).
//  2. UpdateItem schedule row → ADD counters.solved 1. Date keys off
//     submission.AssignedAt (Finding 7 / DP-13: cross-midnight
//     submissions credit the prior date's counter).
//  3. PutItem leaderboard row at DAILY-LEADERBOARD#{playOriginDate} —
//     signed-in only. Anonymous (deviceId-keyed) submissions skip this
//     leg (D13).
//
// All legs commit or none do. DDB's transaction guarantees rule out
// partial writes (DP-22).
func (r *PuzzleRepository) SubmitPlayTransactionally(ctx context.Context, playerID, date string, submission SubmitInput) error {
	playOriginDate := submission.AssignedAt.UTC().Format("2006-01-02")
	submittedAt := submission.SubmittedAt.UTC().Format(time.RFC3339)
	elapsedMs := submission.SubmittedAt.Sub(submission.AssignedAt).Milliseconds()
	if elapsedMs < 0 {
		// Defensive: clock skew or bad input would otherwise produce a
		// negative leaderboard SK, which sorts before legitimate plays
		// and corrupts ranking. Refuse the transaction.
		return fmt.Errorf("invalid submission: submittedAt before assignedAt (delta=%dms)", elapsedMs)
	}

	items := []types.TransactWriteItem{
		{
			Update: &types.Update{
				TableName: aws.String(r.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: buildPlayPK(playerID)},
					"SK": &types.AttributeValueMemberS{Value: buildPlaySK(date)},
				},
				UpdateExpression: aws.String(
					"SET #outcome = :solved, submittedAt = :submittedAt, " +
						"serverElapsedMs = :serverMs, clientClaimedMs = :clientMs",
				),
				ConditionExpression: aws.String("#outcome = :started"),
				ExpressionAttributeNames: map[string]string{
					"#outcome": "outcome",
				},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":solved":      &types.AttributeValueMemberS{Value: PlayOutcomeSolved},
					":started":     &types.AttributeValueMemberS{Value: PlayOutcomeStarted},
					":submittedAt": &types.AttributeValueMemberS{Value: submittedAt},
					":serverMs":    &types.AttributeValueMemberN{Value: strconv.FormatInt(elapsedMs, 10)},
					":clientMs":    &types.AttributeValueMemberN{Value: strconv.FormatInt(submission.ClientMs, 10)},
				},
			},
		},
		{
			Update: &types.Update{
				TableName: aws.String(r.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: buildDailySchedulePK(playOriginDate)},
					"SK": &types.AttributeValueMemberS{Value: dailySingletonSK},
				},
				UpdateExpression: aws.String("ADD #counters.#solved :one"),
				ExpressionAttributeNames: map[string]string{
					"#counters": "counters",
					"#solved":   ScheduleCounterSolved,
				},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":one": &types.AttributeValueMemberN{Value: "1"},
				},
			},
		},
	}

	if !submission.IsAnonymous {
		items = append(items, types.TransactWriteItem{
			Put: &types.Put{
				TableName: aws.String(r.tableName),
				Item: map[string]types.AttributeValue{
					"PK":              &types.AttributeValueMemberS{Value: buildDailyLeaderboardPK(playOriginDate)},
					"SK":              &types.AttributeValueMemberS{Value: buildLeaderboardSK(elapsedMs, submission.UserID)},
					"userId":          &types.AttributeValueMemberS{Value: submission.UserID},
					"serverElapsedMs": &types.AttributeValueMemberN{Value: strconv.FormatInt(elapsedMs, 10)},
					"submittedAt":     &types.AttributeValueMemberS{Value: submittedAt},
					"puzzleId":        &types.AttributeValueMemberS{Value: submission.PuzzleID},
				},
			},
		})
	}

	_, err := r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: items,
	})
	if err != nil {
		// TransactionCanceledException with a ConditionalCheckFailed
		// reason on leg 1 means the PLAY row was already solved (or
		// missing). Surface ErrPlayNotInStartedState so the handler
		// returns 409 (DP-12 idempotency).
		if isConditionalCheckFailureOnLeg(err, 0) {
			return ErrPlayNotInStartedState
		}
		return fmt.Errorf("submitting daily play %s/%s: %w", playerID, date, err)
	}
	return nil
}

// isConditionalCheckFailureOnLeg returns true when err is a
// TransactionCanceledException whose CancellationReasons indicate the
// leg at `legIndex` failed its ConditionExpression. DDB's Go SDK v2
// returns this as a typed error with a CancellationReasons slice whose
// indices align 1:1 with the input TransactItems.
func isConditionalCheckFailureOnLeg(err error, legIndex int) bool {
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
