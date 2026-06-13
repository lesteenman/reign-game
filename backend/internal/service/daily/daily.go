// Package daily is the application service for the daily-puzzle HTTP
// endpoints (GET /api/daily/{date} and POST /api/daily/{date}/result).
// It owns the multi-step orchestration that GET and POST used to do
// inline in the handler: schedule reads, sync-fallback finalize,
// puzzle reads, play-row materialization, submission validation,
// transaction execution, and leaderboard rank lookup.
//
// Method signatures use plain-field DTOs (GetInput, DailyView, SubmitInput,
// SubmitResult) so the HTTP handler can talk to this package without
// importing `internal/repository`.
package daily

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)

// Sentinel errors. The HTTP handler translates these to status codes:
//
//	ErrInvalidDate          -> 400
//	ErrOutOfWindow          -> 404
//	ErrScheduleNotFinalized -> 404 (yesterday's schedule absent)
//	ErrPoolExhausted        -> 500 ("pool exhausted") — declared in sync.go
//	ErrPlayNotStarted       -> 400 ("play not started")
//	ErrAlreadySolved        -> 409
//	ErrInvalidSolution      -> 400
//	ErrInvalidPlayTime      -> 400 ("invalid playTimeMs")
//	ErrNegativeClockSkew    -> 500
//
// Wrapping is fine (handler uses errors.Is). Other errors (DDB I/O,
// etc.) map to 500.
var (
	ErrInvalidDate          = errors.New("invalid date")
	ErrOutOfWindow          = errors.New("out of window")
	ErrScheduleNotFinalized = errors.New("schedule not finalized")
	// ErrPoolExhausted is declared in sync.go (more descriptive message).
	ErrPlayNotStarted    = errors.New("play not started")
	ErrAlreadySolved     = errors.New("already solved")
	ErrInvalidSolution   = errors.New("invalid solution")
	ErrNegativeClockSkew = errors.New("negative clock skew")
	ErrInvalidPlayTime   = errors.New("invalid play time")
)

// Store is the persistence surface used by Service. Subset of
// *repository.PuzzleRepository's methods — production wiring passes
// the repo straight in; tests substitute a fake.
type Store interface {
	GetSchedule(ctx context.Context, date string) (*repository.ScheduleRecord, error)
	GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error)
	GetPlay(ctx context.Context, playerID, date string) (*repository.PlayRecord, error)
	GetCandidate(ctx context.Context) (*repository.CandidateRecord, error)
	WriteTransaction(ctx context.Context, items []types.TransactWriteItem) error
	ListApprovedPool(ctx context.Context, size int, mode string, excludeRecentlyDailied bool, now time.Time) ([]repository.PuzzleRecord, error)
	ListReadyPoolNoDownvotes(ctx context.Context, size int, mode string, excludeRecentlyDailied bool, now time.Time) ([]repository.PuzzleRecord, error)
	PutCandidateIfAbsent(ctx context.Context, puzzleID, sourcePartition string) error
	LeaderboardRank(ctx context.Context, date string, elapsedMs int64, userID string) (int, error)
}

// GetInput is the plain-field input to Service.GetDaily.
type GetInput struct {
	PlayerID    string
	IsAnonymous bool
	Date        string // YYYY-MM-DD UTC
}

// DailyView is the result of GetDaily — enough data for the handler
// to render its 200 JSON body.
//
// ServerElapsedMs and SubmittedAt are pointer-typed: they are set ONLY
// when Outcome == "solved" (the canonical PlayRecord outcome string).
// Handlers use `omitempty` to drop them otherwise.
type DailyView struct {
	PuzzleID        string
	Grid            int
	Regions         [][]int
	AssignedAt      string
	Outcome         string
	ServerElapsedMs *int64
	SubmittedAt     *string
	// IsRecycle is true when this day's schedule resolved to a recycle of
	// a recent day's puzzle (schedule Mode == FinalizeModeRecycle).
	IsRecycle bool
}

// SubmitInput is the plain-field input to Service.SubmitDaily.
type SubmitInput struct {
	PlayerID    string
	IsAnonymous bool
	Date        string // YYYY-MM-DD UTC
	PlayTimeMs  int64
	Solution    [][]int
}

// SubmitResult is the result of SubmitDaily.
// LeaderboardRank is nil for anonymous submits and for rank-fetch
// failures (best-effort post-commit).
type SubmitResult struct {
	ServerElapsedMs int64
	LeaderboardRank *int
}

// Service holds the application logic for the daily-puzzle endpoints.
type Service struct {
	store         Store
	tableName     string
	clock         func() time.Time
	replenishHook func(size int, mode string)
}

// New constructs a Service. tableName is the DynamoDB table name used
// when assembling transaction legs in FinalizeDaily. clock is used in
// tests to pin "now"; pass nil to default to time.Now. replenishHook
// fires on sync-fallback cold-start when the candidate pool drains; nil
// is acceptable in tests and in local-dev wiring without SQS.
func New(store Store, tableName string, clock func() time.Time, replenishHook func(size int, mode string)) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{store: store, tableName: tableName, clock: clock, replenishHook: replenishHook}
}

// FinalizeDaily writes the T=0 finalize transaction: puts the schedule
// row, updates the puzzle's lastDailyDate, and (confirm-mode only)
// deletes the candidate slot. Delegates the actual TransactWriteItems
// call to s.store.WriteTransaction, keeping orchestration in service/
// per the architecture rule.
//
// Returns repository.ErrScheduleAlreadyFinalized when leg 0 fails its
// condition (race-loser / duplicate cron firing). Callers should read the
// winner's schedule row and continue.
func (s *Service) FinalizeDaily(
	ctx context.Context,
	date, puzzleID, sourcePartition string,
	mode repository.FinalizeMode,
) error {
	if mode != repository.FinalizeModeConfirm && mode != repository.FinalizeModeRecycle {
		return fmt.Errorf("invalid finalize mode %q", mode)
	}

	now := s.clock().UTC().Format(time.RFC3339)

	items := []types.TransactWriteItem{
		{
			Put: &types.Put{
				TableName: aws.String(s.tableName),
				Item: map[string]types.AttributeValue{
					"PK":              &types.AttributeValueMemberS{Value: repository.BuildDailySchedulePK(date)},
					"SK":              &types.AttributeValueMemberS{Value: repository.DailySingletonSK},
					"puzzleId":        &types.AttributeValueMemberS{Value: puzzleID},
					"assignedAt":      &types.AttributeValueMemberS{Value: now},
					"sourcePartition": &types.AttributeValueMemberS{Value: sourcePartition},
					"mode":            &types.AttributeValueMemberS{Value: string(mode)},
					"counters": &types.AttributeValueMemberM{
						Value: map[string]types.AttributeValue{
							"started": &types.AttributeValueMemberN{Value: "0"},
							"solved":  &types.AttributeValueMemberN{Value: "0"},
						},
					},
				},
				ConditionExpression: aws.String("attribute_not_exists(PK)"),
			},
		},
		{
			Update: &types.Update{
				TableName: aws.String(s.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: sourcePartition},
					"SK": &types.AttributeValueMemberS{Value: puzzleID},
				},
				UpdateExpression: aws.String("SET lastDailyDate = :date"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":date": &types.AttributeValueMemberS{Value: date},
				},
			},
		},
	}

	if mode == repository.FinalizeModeConfirm {
		items = append(items, types.TransactWriteItem{
			Delete: &types.Delete{
				TableName: aws.String(s.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: repository.DailyCandidatePK},
					"SK": &types.AttributeValueMemberS{Value: repository.DailySingletonSK},
				},
				ConditionExpression: aws.String("puzzleId = :pid"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":pid": &types.AttributeValueMemberS{Value: puzzleID},
				},
			},
		})
	}

	if err := s.store.WriteTransaction(ctx, items); err != nil {
		if repository.IsConditionalCheckFailureOnLeg(err, 0) {
			return repository.ErrScheduleAlreadyFinalized
		}
		return fmt.Errorf("finalizing daily transaction for %s (mode=%s): %w", date, mode, err)
	}
	return nil
}

// SubmitPlay commits a daily-puzzle solve via a single transaction with
// up to three legs:
//
//  1. UpdateItem PLAY → outcome=solved, submittedAt, serverElapsedMs,
//     clientClaimedMs. Conditional on outcome=started for idempotency —
//     a duplicate submission produces repository.ErrPlayNotInStartedState
//     (caller maps to HTTP 409).
//  2. UpdateItem schedule row → ADD counters.solved 1. Date keys off
//     submission.AssignedAt (cross-midnight submissions credit the
//     prior date's counter).
//  3. PutItem leaderboard row at DAILY-LEADERBOARD#{playOriginDate} —
//     signed-in only. Anonymous (deviceId-keyed) submissions and anti-cheat
//     flagged solves (submission.CheatFlag != "") skip this leg; flagged
//     solves also record the reason on the PLAY row in leg 1.
//
// Delegates the actual TransactWriteItems call to s.store.WriteTransaction,
// keeping orchestration in service/ per the architecture rule.
func (s *Service) SubmitPlay(ctx context.Context, playerID, date string, submission *repository.SubmitInput) error {
	playOriginDate := submission.AssignedAt.UTC().Format("2006-01-02")
	submittedAt := submission.SubmittedAt.UTC().Format(time.RFC3339)
	elapsedMs := submission.SubmittedAt.Sub(submission.AssignedAt).Milliseconds()
	if elapsedMs < 0 {
		return fmt.Errorf("invalid submission: submittedAt before assignedAt (delta=%dms)", elapsedMs)
	}

	playUpdateExpr := "SET #outcome = :solved, submittedAt = :submittedAt, " +
		"serverElapsedMs = :serverMs, clientClaimedMs = :clientMs"
	playValues := map[string]types.AttributeValue{
		":solved":      &types.AttributeValueMemberS{Value: repository.PlayOutcomeSolved},
		":started":     &types.AttributeValueMemberS{Value: repository.PlayOutcomeStarted},
		":submittedAt": &types.AttributeValueMemberS{Value: submittedAt},
		":serverMs":    &types.AttributeValueMemberN{Value: strconv.FormatInt(elapsedMs, 10)},
		":clientMs":    &types.AttributeValueMemberN{Value: strconv.FormatInt(submission.ClientMs, 10)},
	}
	// Anti-cheat: a flagged solve records the suspicion reason on the PLAY
	// row and skips the leaderboard leg below — the solve still counts but
	// the suspect time does not rank.
	if submission.CheatFlag != "" {
		playUpdateExpr += ", cheatFlag = :cheatFlag"
		playValues[":cheatFlag"] = &types.AttributeValueMemberS{Value: submission.CheatFlag}
	}

	items := []types.TransactWriteItem{
		{
			Update: &types.Update{
				TableName: aws.String(s.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: repository.BuildPlayPK(playerID)},
					"SK": &types.AttributeValueMemberS{Value: repository.BuildPlaySK(date)},
				},
				UpdateExpression:    aws.String(playUpdateExpr),
				ConditionExpression: aws.String("#outcome = :started"),
				ExpressionAttributeNames: map[string]string{
					"#outcome": "outcome",
				},
				ExpressionAttributeValues: playValues,
			},
		},
		{
			Update: &types.Update{
				TableName: aws.String(s.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: repository.BuildDailySchedulePK(playOriginDate)},
					"SK": &types.AttributeValueMemberS{Value: repository.DailySingletonSK},
				},
				UpdateExpression: aws.String("ADD #counters.#solved :one"),
				ExpressionAttributeNames: map[string]string{
					"#counters": "counters",
					"#solved":   repository.ScheduleCounterSolved,
				},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":one": &types.AttributeValueMemberN{Value: "1"},
				},
			},
		},
	}

	if !submission.IsAnonymous && submission.CheatFlag == "" {
		items = append(items, types.TransactWriteItem{
			Put: &types.Put{
				TableName: aws.String(s.tableName),
				Item: map[string]types.AttributeValue{
					"PK":              &types.AttributeValueMemberS{Value: repository.BuildDailyLeaderboardPK(playOriginDate)},
					"SK":              &types.AttributeValueMemberS{Value: repository.BuildLeaderboardSK(elapsedMs, submission.UserID)},
					"userId":          &types.AttributeValueMemberS{Value: submission.UserID},
					"serverElapsedMs": &types.AttributeValueMemberN{Value: strconv.FormatInt(elapsedMs, 10)},
					"submittedAt":     &types.AttributeValueMemberS{Value: submittedAt},
					"puzzleId":        &types.AttributeValueMemberS{Value: submission.PuzzleID},
				},
			},
		})
	}

	if err := s.store.WriteTransaction(ctx, items); err != nil {
		if repository.IsConditionalCheckFailureOnLeg(err, 0) {
			return repository.ErrPlayNotInStartedState
		}
		return fmt.Errorf("submitting daily play %s/%s: %w", playerID, date, err)
	}
	return nil
}

// GetDaily returns the daily view for (playerID, date). It orchestrates
// schedule/play reads, sync-fallback finalize on cold-start, puzzle read,
// and play-row materialization.
//
// Sentinel errors:
//
//	ErrInvalidDate          — date string does not match YYYY-MM-DD
//	ErrOutOfWindow          — date is not today or yesterday (UTC)
//	ErrScheduleNotFinalized — schedule absent for yesterday (can't retro-finalize)
//	ErrPoolExhausted        — sync-fallback: no puzzle available (500)
//
// All other errors are wrapped DDB I/O failures; callers map them to 500.
func (s *Service) GetDaily(ctx context.Context, in GetInput) (*DailyView, error) {
	start := time.Now()

	parsed, err := time.ParseInLocation(dailyDateLayout, in.Date, time.UTC)
	if err != nil {
		log.Printf("daily service: 400 invalid_date date=%q player=%s anon=%t", in.Date, in.PlayerID, in.IsAnonymous)
		return nil, ErrInvalidDate
	}

	now := s.clock().UTC()
	todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	yesterdayUTC := todayUTC.AddDate(0, 0, -1)
	todayStr := todayUTC.Format(dailyDateLayout)
	yesterdayStr := yesterdayUTC.Format(dailyDateLayout)

	requested := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
	if requested.Before(yesterdayUTC) || requested.After(todayUTC) {
		log.Printf("daily service: 404 out_of_window date=%s player=%s anon=%t", in.Date, in.PlayerID, in.IsAnonymous)
		return nil, ErrOutOfWindow
	}

	readStart := time.Now()
	schedule, existingPlay, sErr, pErr := fetchScheduleAndPlay(ctx, s.store, in.Date, in.PlayerID)
	readMs := time.Since(readStart).Milliseconds()
	if sErr != nil {
		log.Printf("daily service: 500 schedule_read_failed date=%s err=%v", in.Date, sErr)
		return nil, fmt.Errorf("daily service: reading schedule for %s: %w", in.Date, sErr)
	}

	var syncMs int64
	if schedule == nil {
		// Sync-fallback engages ONLY for today. Yesterday's schedule
		// should always exist by the time today is being requested — if it
		// doesn't, the system is in an unrecoverable state and we return
		// ErrScheduleNotFinalized rather than attempt to retro-finalize.
		if in.Date != todayStr {
			log.Printf("daily service: 404 schedule_absent date=%s player=%s", in.Date, truncatePlayer(in.PlayerID))
			return nil, ErrScheduleNotFinalized
		}
		syncStart := time.Now()
		finalized, syncErr := s.SyncFinalizeForToday(ctx, todayStr, yesterdayStr, s.clock())
		syncMs = time.Since(syncStart).Milliseconds()
		if errors.Is(syncErr, ErrPoolExhausted) {
			log.Printf("daily service: 500 pool_exhausted date=%s player=%s sync_ms=%d", in.Date, truncatePlayer(in.PlayerID), syncMs)
			return nil, ErrPoolExhausted
		}
		if syncErr != nil {
			log.Printf("daily service: 500 sync_finalize_failed date=%s sync_ms=%d err=%v", in.Date, syncMs, syncErr)
			return nil, fmt.Errorf("daily service: sync finalize for %s: %w", in.Date, syncErr)
		}
		schedule = finalized
	}

	if pErr != nil {
		log.Printf("daily service: 500 play_read_failed date=%s player=%s err=%v", in.Date, truncatePlayer(in.PlayerID), pErr)
		return nil, fmt.Errorf("daily service: reading play for %s/%s: %w", in.PlayerID, in.Date, pErr)
	}

	size, mode, err := parseSourcePartition(schedule.SourcePartition)
	if err != nil {
		log.Printf("daily service: 500 source_partition_malformed date=%s sourcePartition=%q err=%v", in.Date, schedule.SourcePartition, err)
		return nil, fmt.Errorf("daily service: malformed sourcePartition %q for %s: %w", schedule.SourcePartition, in.Date, err)
	}

	puzzleStart := time.Now()
	puzzle, err := s.store.GetPuzzle(ctx, size, mode, schedule.PuzzleID)
	puzzleMs := time.Since(puzzleStart).Milliseconds()
	if err != nil {
		log.Printf("daily service: 500 puzzle_read_failed date=%s puzzleId=%s err=%v", in.Date, schedule.PuzzleID, err)
		return nil, fmt.Errorf("daily service: reading puzzle %s for %s: %w", schedule.PuzzleID, in.Date, err)
	}
	if puzzle == nil {
		// Schedule pointed at a puzzle that does not exist — broken invariant.
		// Don't return ErrScheduleNotFinalized; that would let a corrupted
		// schedule masquerade as "no daily today".
		log.Printf("daily service: 500 puzzle_missing date=%s puzzleId=%s", in.Date, schedule.PuzzleID)
		return nil, fmt.Errorf("daily service: puzzle %s referenced by schedule for %s does not exist", schedule.PuzzleID, in.Date)
	}

	play, playMs, err := materializePlayRow(ctx, s.store, s.tableName, existingPlay, in.PlayerID, in.Date, schedule.PuzzleID, s.clock)
	if err != nil {
		log.Printf("daily service: 500 play_materialize_failed date=%s player=%s err=%v", in.Date, truncatePlayer(in.PlayerID), err)
		return nil, fmt.Errorf("daily service: materializing play row for %s/%s: %w", in.PlayerID, in.Date, err)
	}

	view := &DailyView{
		PuzzleID:   schedule.PuzzleID,
		Grid:       puzzle.GridSize,
		Regions:    puzzle.RegionMap,
		AssignedAt: play.AssignedAt,
		Outcome:    play.Outcome,
		IsRecycle:  schedule.Mode == repository.FinalizeModeRecycle,
	}
	if play.Outcome == repository.PlayOutcomeSolved {
		elapsed := play.ServerElapsedMs
		submittedAt := play.SubmittedAt
		view.ServerElapsedMs = &elapsed
		view.SubmittedAt = &submittedAt
	}

	totalMs := time.Since(start).Milliseconds()
	log.Printf("daily service: total_ms=%d read_ms=%d sync_ms=%d puzzle_ms=%d play_ms=%d date=%s player=%s",
		totalMs, readMs, syncMs, puzzleMs, playMs, in.Date, truncatePlayer(in.PlayerID))

	return view, nil
}

// SubmitDaily records a solved daily play and returns the elapsed time
// + leaderboard rank.
//
// Sentinel errors:
//
//	ErrInvalidDate        — date string does not match YYYY-MM-DD
//	ErrOutOfWindow        — date is not today or yesterday (UTC)
//	ErrInvalidSolution    — solution shape invalid or cells don't match puzzle
//	ErrInvalidPlayTime    — playTimeMs is negative
//	ErrPlayNotStarted     — no play row for this player/date
//	ErrAlreadySolved      — play already in solved state (inc. race-loser)
//	ErrNegativeClockSkew  — server clock predates the play row's assignedAt
//
// All other errors are wrapped I/O failures; callers map them to 500.
func (s *Service) SubmitDaily(ctx context.Context, in SubmitInput) (*SubmitResult, error) {
	start := time.Now()

	// 1. Validate date format + window.
	parsed, err := time.ParseInLocation(dailyDateLayout, in.Date, time.UTC)
	if err != nil {
		log.Printf("daily service: submit 400 invalid_date date=%q player=%s", in.Date, truncatePlayer(in.PlayerID))
		return nil, ErrInvalidDate
	}
	now := s.clock().UTC()
	todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	yesterdayUTC := todayUTC.AddDate(0, 0, -1)
	requested := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
	if requested.Before(yesterdayUTC) || requested.After(todayUTC) {
		log.Printf("daily service: submit 404 out_of_window date=%s player=%s", in.Date, truncatePlayer(in.PlayerID))
		return nil, ErrOutOfWindow
	}

	// 2. Validate solution shape.
	if !solutionShapeValid(in.Solution) {
		log.Printf("daily service: submit 400 invalid_solution_shape date=%s player=%s", in.Date, truncatePlayer(in.PlayerID))
		return nil, ErrInvalidSolution
	}

	// 2b. Validate playTimeMs — negative values are rejected. Nil (absent)
	// is defaulted to 0 by the handler, which passes this check.
	if in.PlayTimeMs < 0 {
		log.Printf("daily service: submit 400 invalid_play_time_ms date=%s player=%s playTimeMs=%d",
			in.Date, truncatePlayer(in.PlayerID), in.PlayTimeMs)
		return nil, ErrInvalidPlayTime
	}

	// 3. Fetch schedule + play in parallel.
	readStart := time.Now()
	schedule, play, sErr, pErr := fetchScheduleAndPlay(ctx, s.store, in.Date, in.PlayerID)
	readMs := time.Since(readStart).Milliseconds()

	// 4. Play state checks.
	if pErr != nil {
		log.Printf("daily service: submit 500 play_read_failed date=%s player=%s err=%v", in.Date, truncatePlayer(in.PlayerID), pErr)
		return nil, fmt.Errorf("daily service: submit reading play for %s/%s: %w", in.PlayerID, in.Date, pErr)
	}
	if play == nil {
		log.Printf("daily service: submit 400 play_not_started date=%s player=%s", in.Date, truncatePlayer(in.PlayerID))
		return nil, ErrPlayNotStarted
	}
	if play.Outcome == repository.PlayOutcomeSolved {
		log.Printf("daily service: submit 409 already_solved date=%s player=%s", in.Date, truncatePlayer(in.PlayerID))
		return nil, ErrAlreadySolved
	}

	// 5. Schedule must exist — play row being present is the invariant guard.
	if sErr != nil {
		log.Printf("daily service: submit 500 schedule_read_failed date=%s err=%v", in.Date, sErr)
		return nil, fmt.Errorf("daily service: submit reading schedule for %s: %w", in.Date, sErr)
	}
	if schedule == nil {
		log.Printf("daily service: submit 500 schedule_missing_for_play date=%s player=%s", in.Date, truncatePlayer(in.PlayerID))
		return nil, fmt.Errorf("daily service: submit schedule missing for %s but play exists (invariant violation)", in.Date)
	}

	// 6. Parse sourcePartition → size + mode.
	size, mode, err := parseSourcePartition(schedule.SourcePartition)
	if err != nil {
		log.Printf("daily service: submit 500 source_partition_malformed date=%s sourcePartition=%q err=%v", in.Date, schedule.SourcePartition, err)
		return nil, fmt.Errorf("daily service: submit malformed sourcePartition %q for %s: %w", schedule.SourcePartition, in.Date, err)
	}

	// 7. Fetch puzzle.
	puzzleStart := time.Now()
	puzzle, err := s.store.GetPuzzle(ctx, size, mode, schedule.PuzzleID)
	puzzleMs := time.Since(puzzleStart).Milliseconds()
	if err != nil {
		log.Printf("daily service: submit 500 puzzle_read_failed date=%s puzzleId=%s err=%v", in.Date, schedule.PuzzleID, err)
		return nil, fmt.Errorf("daily service: submit reading puzzle %s for %s: %w", schedule.PuzzleID, in.Date, err)
	}
	if puzzle == nil {
		log.Printf("daily service: submit 500 puzzle_missing date=%s puzzleId=%s", in.Date, schedule.PuzzleID)
		return nil, fmt.Errorf("daily service: submit puzzle %s referenced by schedule for %s does not exist", schedule.PuzzleID, in.Date)
	}

	// 8. Verify solution matches puzzle.
	if !solutionMatches(in.Solution, puzzle.Solution) {
		log.Printf("daily service: submit 400 invalid_solution date=%s player=%s puzzleId=%s", in.Date, truncatePlayer(in.PlayerID), schedule.PuzzleID)
		return nil, ErrInvalidSolution
	}

	// 9. Parse play.AssignedAt.
	assignedAt, err := time.Parse(time.RFC3339, play.AssignedAt)
	if err != nil {
		log.Printf("daily service: submit 500 assignedAt_parse_failed date=%s assignedAt=%q err=%v", in.Date, play.AssignedAt, err)
		return nil, fmt.Errorf("daily service: submit parsing assignedAt %q for %s: %w", play.AssignedAt, in.Date, err)
	}

	// 10. Compute serverElapsedMs; reject negative clock skew.
	serverElapsedMs := now.Sub(assignedAt).Milliseconds()
	if serverElapsedMs < 0 {
		log.Printf("daily service: submit 500 negative_clock_skew date=%s assignedAt=%s now=%s delta_ms=%d player=%s",
			in.Date, assignedAt.Format(time.RFC3339), now.Format(time.RFC3339), serverElapsedMs, truncatePlayer(in.PlayerID))
		return nil, ErrNegativeClockSkew
	}

	// 11. Anti-cheat: flag implausibly fast solves and large client/server
	// time divergence. A flagged solve still counts but is excluded from the
	// leaderboard (silent — the player-facing response is unchanged). Uses
	// the difficulty from the puzzle already read above (no extra DDB read).
	cheatFlag := evaluateCheatFlag(puzzle.Difficulty, serverElapsedMs, in.PlayTimeMs)
	if cheatFlag != "" {
		log.Printf("WARN: daily service: submit cheat_flag=%s date=%s player=%s difficulty=%d serverElapsedMs=%d clientClaimedMs=%d",
			cheatFlag, in.Date, truncatePlayer(in.PlayerID), puzzle.Difficulty, serverElapsedMs, in.PlayTimeMs)
	}

	// 12. Build submission and call SubmitPlay.
	submitInput := repository.SubmitInput{
		PuzzleID:    schedule.PuzzleID,
		AssignedAt:  assignedAt,
		SubmittedAt: now,
		ClientMs:    in.PlayTimeMs,
		IsAnonymous: in.IsAnonymous,
		CheatFlag:   cheatFlag,
	}
	if !in.IsAnonymous {
		submitInput.UserID = in.PlayerID
	}

	submitStart := time.Now()
	submitErr := s.SubmitPlay(ctx, in.PlayerID, in.Date, &submitInput)
	submitMs := time.Since(submitStart).Milliseconds()
	if submitErr != nil {
		if errors.Is(submitErr, repository.ErrPlayNotInStartedState) {
			log.Printf("daily service: submit 409 race_loser date=%s player=%s", in.Date, truncatePlayer(in.PlayerID))
			return nil, ErrAlreadySolved
		}
		log.Printf("daily service: submit 500 transaction_failed date=%s player=%s err=%v", in.Date, truncatePlayer(in.PlayerID), submitErr)
		return nil, fmt.Errorf("daily service: submit transaction for %s/%s: %w", in.PlayerID, in.Date, submitErr)
	}

	// 13. Fetch leaderboard rank (best-effort; signed-in + unflagged only).
	// A flagged solve has no leaderboard row, so a rank lookup would be
	// misleading — leave LeaderboardRank nil (silent exclusion).
	result := &SubmitResult{ServerElapsedMs: serverElapsedMs}
	var rankMs int64
	if !in.IsAnonymous && cheatFlag == "" {
		rankStart := time.Now()
		rank, rankErr := s.store.LeaderboardRank(ctx, in.Date, serverElapsedMs, in.PlayerID)
		rankMs = time.Since(rankStart).Milliseconds()
		if rankErr != nil {
			log.Printf("WARN: daily service: submit rank_fetch_failed date=%s player=%s err=%v", in.Date, truncatePlayer(in.PlayerID), rankErr)
		} else {
			result.LeaderboardRank = &rank
		}
	}

	totalMs := time.Since(start).Milliseconds()
	log.Printf("daily service: submit total_ms=%d read_ms=%d puzzle_ms=%d submit_ms=%d rank_ms=%d date=%s player=%s",
		totalMs, readMs, puzzleMs, submitMs, rankMs, in.Date, truncatePlayer(in.PlayerID))

	return result, nil
}
