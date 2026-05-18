# Daily Recycle Counter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the daily-puzzle recycle bug so a puzzle that any player started is not re-assigned the next day, and bundle the bundled `formatTime` HH:MM:SS overflow fix.

**Architecture:** Two-leg `TransactWriteItems` on first-GET (PLAY put + schedule `ADD counters.started`) gives the recycle decision tree a real signal to read. `chooseFinalizeTarget` then gates on `Solved == 0 && Started == 0`. Frontend formatting fix is one-line, isolated.

**Tech Stack:** Go 1.x (backend, AWS SDK v2), React 19 + TypeScript + Vitest (frontend).

**Spec:** `docs/superpowers/specs/2026-05-18-daily-recycle-counter-design.md`
**Issue:** [#184](https://github.com/lesteenman/reign-game/issues/184)
**Branch:** `feat/184-daily-recycle-counter`

---

## File Structure

**Modified:**
- `backend/internal/service/daily/sync.go` — `chooseFinalizeTarget` gate, doc comment
- `backend/internal/service/daily/sync_test.go` — flip existing "recycle when started>0" test to "confirm", add new gated-recycle test
- `backend/internal/service/daily/helpers.go` — `materializePlayRow` switches to `WriteTransaction`
- `backend/internal/service/daily/daily.go` — `Store` interface drops `PutPlayStartedIfAbsent`; `GetDaily` passes `s.tableName` to `materializePlayRow`
- `backend/internal/service/daily/get_test.go` — fake store: drop `PutPlayStartedIfAbsent`, add `WriteTransaction`; existing PLAY-creation tests updated; add explicit "started counter increments on first GET" assertion
- `backend/internal/service/daily/cron_test.go`, `daily_test.go`, `finalize_test.go`, `submit_play_test.go`, `submit_test.go` — fake stores: drop `PutPlayStartedIfAbsent` (interface no longer requires it)
- `backend/internal/repository/daily.go` — delete `PutPlayStartedIfAbsent` (no remaining callers after Store interface change)
- `backend/internal/repository/daily_test.go` — delete `TestPutPlayStartedIfAbsent`
- `backend/internal/repository/README.md` — drop reference to removed method
- `frontend/src/shared/game/components/GameBoard.tsx` — `formatTime` → HH:MM:SS overflow + export

**Created:**
- `frontend/src/shared/game/components/formatTime.test.ts` — unit tests for `formatTime`

---

## Task 1 — Backend: gate `chooseFinalizeTarget` on `Started == 0`

**Files:**
- Modify: `backend/internal/service/daily/sync_test.go`
- Modify: `backend/internal/service/daily/sync.go:124-147`

- [ ] **Step 1.1: Add failing test for the new gate**

Append to `backend/internal/service/daily/sync_test.go` (after the existing `TestSyncFinalizeForToday_*` block):

```go
func TestChooseFinalizeTarget_StartedButNotSolved_Confirms(t *testing.T) {
	// Arrange — yesterday has started>0, solved==0; candidate present.
	// New gate: do NOT recycle (someone engaged with yesterday's puzzle).
	candidate := &repository.CandidateRecord{
		PuzzleID:        "puzzle-candidate",
		SourcePartition: "9#standard",
	}
	yesterday := &repository.ScheduleRecord{
		Date:            "2026-05-17",
		PuzzleID:        "puzzle-yesterday",
		SourcePartition: "9#standard",
		Counters:        repository.ScheduleCounters{Started: 1, Solved: 0},
	}

	// Act
	puzzleID, sourcePartition, mode, err := chooseFinalizeTarget(candidate, yesterday)

	// Assert
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if mode != repository.FinalizeModeConfirm {
		t.Errorf("expected FinalizeModeConfirm (started>0 blocks recycle), got %q", mode)
	}
	if puzzleID != "puzzle-candidate" {
		t.Errorf("expected candidate puzzleID, got %q", puzzleID)
	}
	if sourcePartition != "9#standard" {
		t.Errorf("expected candidate sourcePartition, got %q", sourcePartition)
	}
}

func TestChooseFinalizeTarget_NoStartsNoSolves_Recycles(t *testing.T) {
	// Arrange — yesterday untouched (nobody started, nobody solved);
	// candidate present. Recycle is the right call.
	candidate := &repository.CandidateRecord{
		PuzzleID:        "puzzle-candidate",
		SourcePartition: "9#standard",
	}
	yesterday := &repository.ScheduleRecord{
		Date:            "2026-05-17",
		PuzzleID:        "puzzle-yesterday",
		SourcePartition: "9#standard",
		Counters:        repository.ScheduleCounters{Started: 0, Solved: 0},
	}

	// Act
	_, _, mode, err := chooseFinalizeTarget(candidate, yesterday)

	// Assert
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if mode != repository.FinalizeModeRecycle {
		t.Errorf("expected FinalizeModeRecycle, got %q", mode)
	}
}
```

Also flip the existing buggy-behaviour test. Find `TestSyncFinalizeForToday_RecycleYesterday_NoSolves` (around `sync_test.go:206-250`) and rewrite it. Its current arrange sets `Started: 5, Solved: 0` and asserts recycle — that's exactly what the bug looked like. Replace its body with:

```go
func TestSyncFinalizeForToday_RecycleYesterday_NoSolves(t *testing.T) {
	// Arrange — nobody started AND nobody solved yesterday;
	// candidate present. With the gated recycle, this is the only
	// way recycle fires when a candidate exists.
	today := "2026-05-02"
	yesterday := "2026-05-01"
	repo := &fakeRepo{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			yesterday: {
				Date:            yesterday,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				Counters:        repository.ScheduleCounters{Started: 0, Solved: 0},
			},
		},
		candidate: &repository.CandidateRecord{
			PuzzleID:        "puzzle-candidate",
			SourcePartition: "9#double",
		},
		scheduleAfterFinalize: map[string]*repository.ScheduleRecord{
			today: {
				Date:            today,
				PuzzleID:        "puzzle-yesterday",
				SourcePartition: "9#standard",
				AssignedAt:      "2026-05-02T00:00:01Z",
			},
		},
	}
	svc := newTestService(repo)

	// Act
	_, err := svc.SyncFinalizeForToday(context.Background(), today, yesterday, time.Now())

	// Assert
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.finalizeCall.mode != repository.FinalizeModeRecycle {
		t.Errorf("expected mode=recycle, got %q", repo.finalizeCall.mode)
	}
	if repo.finalizeCall.puzzleID != "puzzle-yesterday" {
		t.Errorf("expected puzzleID=puzzle-yesterday (recycle), got %q", repo.finalizeCall.puzzleID)
	}
	if repo.finalizeCall.sourcePartition != "9#standard" {
		t.Errorf("expected sourcePartition=9#standard (yesterday's), got %q", repo.finalizeCall.sourcePartition)
	}
}
```

- [ ] **Step 1.2: Run tests to confirm RED**

```
cd backend && go test ./internal/service/daily/ -run 'TestChooseFinalizeTarget|TestSyncFinalizeForToday_RecycleYesterday_NoSolves' -v
```
Expected: `TestChooseFinalizeTarget_StartedButNotSolved_Confirms` fails (production code still recycles when Started>0). `TestSyncFinalizeForToday_RecycleYesterday_NoSolves` passes (Started==0 ∧ Solved==0 still recycles under the current buggy logic too — it's the second-line check that's broken). `TestChooseFinalizeTarget_NoStartsNoSolves_Recycles` passes.

- [ ] **Step 1.3: Fix `chooseFinalizeTarget`**

Edit `backend/internal/service/daily/sync.go:143-146`:

```go
	if yesterday.Counters.Solved == 0 && yesterday.Counters.Started == 0 {
		return yesterday.PuzzleID, yesterday.SourcePartition, repository.FinalizeModeRecycle, nil
	}
	return candidate.PuzzleID, candidate.SourcePartition, repository.FinalizeModeConfirm, nil
```

- [ ] **Step 1.4: Update doc comment**

Edit the algorithm block at `sync.go:43-48`:

```go
//  3. Decision tree:
//     candidate empty AND yesterday missing -> ErrPoolExhausted
//     candidate empty AND yesterday present -> recycle yesterday
//     candidate present AND yesterday missing -> confirm candidate
//     candidate present AND yesterday.started == 0 AND yesterday.solved == 0 -> recycle yesterday
//     candidate present AND (yesterday.started > 0 OR yesterday.solved > 0) -> confirm candidate
```

- [ ] **Step 1.5: Run tests to confirm GREEN**

```
cd backend && go test ./internal/service/daily/ -run 'TestChooseFinalizeTarget|TestSyncFinalizeForToday_RecycleYesterday_NoSolves' -v
```
Expected: all PASS.

- [ ] **Step 1.6: Commit**

```bash
git add backend/internal/service/daily/sync.go backend/internal/service/daily/sync_test.go
git commit -m "$(cat <<'EOF'
fix(backend): gate daily recycle on started==0 as well as solved==0 (#184)

A daily puzzle that any player opened (started>0) is no longer recycled
when nobody solved it. The decision tree in chooseFinalizeTarget now
requires both counters to be zero before reusing yesterday's puzzleId.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2 — Backend: atomic `materializePlayRow` (PUT + counter bump in one TX)

**Files:**
- Modify: `backend/internal/service/daily/helpers.go:58-98`
- Modify: `backend/internal/service/daily/daily.go:390` (call site adds `s.tableName`)
- Modify: `backend/internal/service/daily/get_test.go` (fake store: drop `PutPlayStartedIfAbsent`, add `WriteTransaction`)

- [ ] **Step 2.1: Add failing tests for the atomic-TX behaviour**

In `backend/internal/service/daily/get_test.go`, the existing `getDailyFakeStore` implements `PutPlayStartedIfAbsent` (line 44). Replace that method with `WriteTransaction`:

```go
// WriteTransaction records the items so tests can assert the PLAY put
// + counters.started bump both fired. Returns a synthetic
// TransactionCanceledException on demand to simulate race-loser.
func (f *getDailyFakeStore) WriteTransaction(_ context.Context, items []types.TransactWriteItem) error {
	f.writeTransactionCalls = append(f.writeTransactionCalls, items)
	if f.writeTransactionErr != nil {
		return f.writeTransactionErr
	}
	return nil
}
```

Add fields to the fake struct:

```go
writeTransactionCalls [][]types.TransactWriteItem
writeTransactionErr   error
```

And add helper imports if missing:

```go
"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
```

Append three tests at the bottom of `get_test.go`:

```go
func TestGetDaily_FirstGet_AtomicallyPutsPlayAndBumpsStarted(t *testing.T) {
	// Arrange — schedule exists, no PLAY row yet. First GET should
	// issue ONE TransactWriteItems with two legs:
	//   leg 0: Put PLAY with attribute_not_exists(PK)
	//   leg 1: Update DAILY#{date} ADD counters.started 1
	store := &getDailyFakeStore{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			"2026-05-18": {
				Date:            "2026-05-18",
				PuzzleID:        "puzzle-1",
				SourcePartition: "9#standard",
			},
		},
		puzzleByID: map[string]*repository.PuzzleRecord{
			"puzzle-1": fakePuzzleRecord("puzzle-1"),
		},
	}
	svc := New(store, "puzzle-pool", func() time.Time { return time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC) }, nil)

	// Act
	_, err := svc.GetDaily(context.Background(), GetInput{PlayerID: "alice", Date: "2026-05-18"})

	// Assert
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if len(store.writeTransactionCalls) != 1 {
		t.Fatalf("expected 1 WriteTransaction call, got %d", len(store.writeTransactionCalls))
	}
	items := store.writeTransactionCalls[0]
	if len(items) != 2 {
		t.Fatalf("expected 2 TX legs (PLAY put + counter bump), got %d", len(items))
	}
	if items[0].Put == nil {
		t.Errorf("leg 0 should be Put (PLAY row), got %+v", items[0])
	} else {
		pk := items[0].Put.Item["PK"].(*types.AttributeValueMemberS).Value
		if pk != "PLAY#alice" {
			t.Errorf("leg 0 PK = %q, want PLAY#alice", pk)
		}
		if items[0].Put.ConditionExpression == nil || *items[0].Put.ConditionExpression != "attribute_not_exists(PK)" {
			t.Errorf("leg 0 should have attribute_not_exists(PK) condition")
		}
	}
	if items[1].Update == nil {
		t.Errorf("leg 1 should be Update (counter bump), got %+v", items[1])
	} else {
		pk := items[1].Update.Key["PK"].(*types.AttributeValueMemberS).Value
		if pk != "DAILY#2026-05-18" {
			t.Errorf("leg 1 PK = %q, want DAILY#2026-05-18", pk)
		}
		if items[1].Update.UpdateExpression == nil || !strings.Contains(*items[1].Update.UpdateExpression, "started") {
			t.Errorf("leg 1 UpdateExpression should reference started counter, got %v", items[1].Update.UpdateExpression)
		}
	}
}

func TestGetDaily_ExistingPlay_DoesNotIssueTransaction(t *testing.T) {
	// Arrange — PLAY row already exists for (alice, today). Second GET
	// should short-circuit: no WriteTransaction call, no counter
	// double-increment.
	store := &getDailyFakeStore{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			"2026-05-18": {
				Date:            "2026-05-18",
				PuzzleID:        "puzzle-1",
				SourcePartition: "9#standard",
			},
		},
		puzzleByID: map[string]*repository.PuzzleRecord{
			"puzzle-1": fakePuzzleRecord("puzzle-1"),
		},
		playByKey: map[string]*repository.PlayRecord{
			"alice|2026-05-18": {
				PlayerID:   "alice",
				Date:       "2026-05-18",
				PuzzleID:   "puzzle-1",
				Outcome:    repository.PlayOutcomeStarted,
				AssignedAt: "2026-05-18T08:00:00Z",
			},
		},
	}
	svc := New(store, "puzzle-pool", func() time.Time { return time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC) }, nil)

	// Act
	_, err := svc.GetDaily(context.Background(), GetInput{PlayerID: "alice", Date: "2026-05-18"})

	// Assert
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if len(store.writeTransactionCalls) != 0 {
		t.Errorf("expected 0 WriteTransaction calls for second GET, got %d", len(store.writeTransactionCalls))
	}
}

func TestGetDaily_RaceLoserOnFirstGet_ReReadsWinnerPlay(t *testing.T) {
	// Arrange — schedule exists, no PLAY row initially, but
	// WriteTransaction returns a leg-0 conditional-check failure
	// (another player's first-GET race won between our GetPlay and
	// our WriteTransaction). After the failure we re-read GetPlay and
	// return the winner's row.
	winner := &repository.PlayRecord{
		PlayerID:   "alice",
		Date:       "2026-05-18",
		PuzzleID:   "puzzle-1",
		Outcome:    repository.PlayOutcomeStarted,
		AssignedAt: "2026-05-18T11:00:00Z",
	}
	store := &getDailyFakeStore{
		scheduleByDate: map[string]*repository.ScheduleRecord{
			"2026-05-18": {
				Date:            "2026-05-18",
				PuzzleID:        "puzzle-1",
				SourcePartition: "9#standard",
			},
		},
		puzzleByID: map[string]*repository.PuzzleRecord{
			"puzzle-1": fakePuzzleRecord("puzzle-1"),
		},
		writeTransactionErr: makeCancelErr("ConditionalCheckFailed", "None"),
		playByKeyAfterTX: map[string]*repository.PlayRecord{
			"alice|2026-05-18": winner,
		},
	}
	svc := New(store, "puzzle-pool", func() time.Time { return time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC) }, nil)

	// Act
	got, err := svc.GetDaily(context.Background(), GetInput{PlayerID: "alice", Date: "2026-05-18"})

	// Assert
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got.AssignedAt != winner.AssignedAt {
		t.Errorf("expected winner's assignedAt %q, got %q", winner.AssignedAt, got.AssignedAt)
	}
}
```

`makeCancelErr(...)` is already defined in `backend/internal/service/daily/finalize_test.go:63` — same package, callable directly from `get_test.go`. It takes one string per TX leg ("ConditionalCheckFailed" for the failing leg, "None" for the rest) and returns a `*types.TransactionCanceledException` shaped the way `repository.IsConditionalCheckFailureOnLeg` expects. No new helper needed.

Imports to add at the top of `get_test.go` if not already present:

```go
"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
```

Also extend the fake store fields:

```go
playByKey         map[string]*repository.PlayRecord  // returned by GetPlay BEFORE WriteTransaction
playByKeyAfterTX  map[string]*repository.PlayRecord  // returned by GetPlay AFTER a failed WriteTransaction (race-loser re-read)
```

And update the fake's `GetPlay` to consult `playByKeyAfterTX` when at least one `WriteTransaction` call has been made:

```go
func (f *getDailyFakeStore) GetPlay(_ context.Context, playerID, date string) (*repository.PlayRecord, error) {
	key := playerID + "|" + date
	if len(f.writeTransactionCalls) > 0 {
		if rec, ok := f.playByKeyAfterTX[key]; ok {
			return rec, nil
		}
	}
	return f.playByKey[key], nil
}
```

(If the existing fake's `GetPlay` has additional behaviour, preserve it — just add the after-TX branch.)

- [ ] **Step 2.2: Run the three new tests to confirm RED**

```
cd backend && go test ./internal/service/daily/ -run 'TestGetDaily_FirstGet_AtomicallyPutsPlayAndBumpsStarted|TestGetDaily_ExistingPlay_DoesNotIssueTransaction|TestGetDaily_RaceLoserOnFirstGet_ReReadsWinnerPlay' -v
```
Expected: all three FAIL (compile error: fake no longer matches `Store` interface; or runtime failure because production code still calls `PutPlayStartedIfAbsent`).

If it's a compile error in the OTHER fakes (`cron_test.go`, `daily_test.go`, etc.) saying they don't satisfy `Store`, that's expected — Task 3 removes the interface method. For this step, just verify the three new tests compile within `get_test.go` and run-fail.

- [ ] **Step 2.3: Refactor `materializePlayRow` to use `WriteTransaction`**

Replace the body of `materializePlayRow` in `backend/internal/service/daily/helpers.go:58-98` with:

```go
// materializePlayRow returns the PLAY row for (playerID, date),
// creating it on first GET via a two-leg TransactWriteItems that
// atomically puts the PLAY row and bumps the schedule's
// counters.started — the signal chooseFinalizeTarget reads at T=0 to
// decide recycle-vs-confirm. On the race-loser branch it re-reads the
// row so the caller surfaces the winner's assignedAt. existingPlay is
// the result of an upstream GetPlay; when non-nil the function
// short-circuits and returns it directly. playMs is the wall-clock
// cost of any PLAY-related DDB calls this function issues itself.
func materializePlayRow(
	ctx context.Context,
	store Store,
	tableName string,
	existingPlay *repository.PlayRecord,
	playerID, date, puzzleID string,
	clock func() time.Time,
) (*repository.PlayRecord, int64, error) {
	if existingPlay != nil {
		return existingPlay, 0, nil
	}

	playStart := time.Now()
	now := clock().UTC()
	assignedAt := now.Format(time.RFC3339)

	items := []types.TransactWriteItem{
		{
			Put: &types.Put{
				TableName: aws.String(tableName),
				Item: map[string]types.AttributeValue{
					"PK":         &types.AttributeValueMemberS{Value: repository.BuildPlayPK(playerID)},
					"SK":         &types.AttributeValueMemberS{Value: repository.BuildPlaySK(date)},
					"outcome":    &types.AttributeValueMemberS{Value: repository.PlayOutcomeStarted},
					"assignedAt": &types.AttributeValueMemberS{Value: assignedAt},
					"puzzleId":   &types.AttributeValueMemberS{Value: puzzleID},
				},
				ConditionExpression: aws.String("attribute_not_exists(PK)"),
			},
		},
		{
			Update: &types.Update{
				TableName: aws.String(tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: repository.BuildDailySchedulePK(date)},
					"SK": &types.AttributeValueMemberS{Value: repository.DailySingletonSK},
				},
				UpdateExpression: aws.String("ADD #counters.#started :one"),
				ExpressionAttributeNames: map[string]string{
					"#counters": "counters",
					"#started":  repository.ScheduleCounterStarted,
				},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":one": &types.AttributeValueMemberN{Value: "1"},
				},
			},
		},
	}

	txErr := store.WriteTransaction(ctx, items)
	if txErr == nil {
		return &repository.PlayRecord{
			PlayerID:   playerID,
			Date:       date,
			Outcome:    repository.PlayOutcomeStarted,
			AssignedAt: assignedAt,
			PuzzleID:   puzzleID,
		}, time.Since(playStart).Milliseconds(), nil
	}

	if !repository.IsConditionalCheckFailureOnLeg(txErr, 0) {
		return nil, time.Since(playStart).Milliseconds(), txErr
	}

	// Race-loser: leg 0 condition failed because another first-GET
	// landed first. Re-read so the caller surfaces the winner's
	// assignedAt.
	winner, err := store.GetPlay(ctx, playerID, date)
	if err != nil {
		return nil, time.Since(playStart).Milliseconds(), err
	}
	if winner == nil {
		return nil, time.Since(playStart).Milliseconds(), errors.New("play row vanished after race-loser conditional fail")
	}
	return winner, time.Since(playStart).Milliseconds(), nil
}
```

Add imports if missing at the top of `helpers.go`:

```go
import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/eriksteenman/reign-game/backend/internal/repository"
)
```

(The existing `strconv` import may not be needed in helpers.go after the change — let `goimports` sort it out, or leave the import set as-is and run lint.)

- [ ] **Step 2.4: Update the call site in `daily.go`**

`backend/internal/service/daily/daily.go:390` becomes:

```go
play, playMs, err := materializePlayRow(ctx, s.store, s.tableName, existingPlay, in.PlayerID, in.Date, schedule.PuzzleID, s.clock)
```

- [ ] **Step 2.5: Run the three new tests to confirm GREEN**

```
cd backend && go test ./internal/service/daily/ -run 'TestGetDaily_FirstGet_AtomicallyPutsPlayAndBumpsStarted|TestGetDaily_ExistingPlay_DoesNotIssueTransaction|TestGetDaily_RaceLoserOnFirstGet_ReReadsWinnerPlay' -v
```
Expected: all three PASS.

(The OTHER packages and the rest of `service/daily/` may still fail to compile because their fake stores implement `PutPlayStartedIfAbsent` and no longer match the interface — Task 3 handles that.)

- [ ] **Step 2.6: Commit (work-in-progress, compiles for get_test only)**

Do NOT commit yet — compile is broken across the package. Bundle with Task 3.

---

## Task 3 — Backend: remove `PutPlayStartedIfAbsent` from `Store`, repo, all fakes

**Files:**
- Modify: `backend/internal/service/daily/daily.go:55-65` (Store interface)
- Modify: `backend/internal/service/daily/cron_test.go`, `daily_test.go`, `finalize_test.go`, `submit_play_test.go`, `submit_test.go`, `sync_test.go` (fake stores)
- Modify: `backend/internal/repository/daily.go` (delete method + its doc + the `ErrPlayAlreadyExists` sentinel if unused)
- Modify: `backend/internal/repository/daily_test.go` (delete `TestPutPlayStartedIfAbsent`)
- Modify: `backend/internal/repository/README.md`

- [ ] **Step 3.1: Drop the interface method**

Edit `backend/internal/service/daily/daily.go:55-65`. Remove this line:

```go
PutPlayStartedIfAbsent(ctx context.Context, playerID, date, puzzleID string, assignedAt time.Time) error
```

- [ ] **Step 3.2: Remove the method from every fake store**

In each of these files, delete the `PutPlayStartedIfAbsent` method on the local fake struct (the signature is identical in each):

- `backend/internal/service/daily/cron_test.go` (around line 69)
- `backend/internal/service/daily/daily_test.go` (around line 85)
- `backend/internal/service/daily/finalize_test.go` (around line 32)
- `backend/internal/service/daily/submit_play_test.go` (around line 31)
- `backend/internal/service/daily/submit_test.go` (around line 44)
- `backend/internal/service/daily/sync_test.go` (around line 126)
- `backend/internal/service/daily/get_test.go` (around line 44) — already removed in Task 2.

Use `grep -n "PutPlayStartedIfAbsent" backend/internal/service/daily/*.go` to confirm exhaustive removal.

- [ ] **Step 3.3: Check `ErrPlayAlreadyExists` callers**

```
grep -rn "ErrPlayAlreadyExists" backend/
```

If no remaining callers (besides the deletion site), remove the constant from `backend/internal/repository/daily.go:57-60`. If callers remain (e.g. another part of the code paths the same sentinel), leave it.

- [ ] **Step 3.4: Remove `PutPlayStartedIfAbsent` from the production repo**

In `backend/internal/repository/daily.go`, delete the function block at lines 383-411 (the comment block + the function body). Verify no remaining callers in `backend/`:

```
grep -rn "PutPlayStartedIfAbsent" backend/
```

Should return zero results.

- [ ] **Step 3.5: Remove the repo-level test**

In `backend/internal/repository/daily_test.go`, delete `TestPutPlayStartedIfAbsent` (around line 563-650, depending on the file's exact shape).

- [ ] **Step 3.6: Update the README**

In `backend/internal/repository/README.md:40`, drop the `PutPlayStartedIfAbsent` reference from the daily-play bullet:

Before:
```
- **Daily play / leaderboard** — `GetPlay`, `PutPlayStartedIfAbsent`, `SubmitPlayTransactionally` (3-leg `TransactWriteItems`), `LeaderboardRank`.
```

After:
```
- **Daily play / leaderboard** — `GetPlay`, `SubmitPlayTransactionally` (3-leg `TransactWriteItems`), `LeaderboardRank`. PLAY-row creation is performed by a 2-leg `TransactWriteItems` composed in `service/daily/` (PUT + `counters.started` bump).
```

- [ ] **Step 3.7: Build + run the whole backend test suite**

```
cd backend && go build ./... && go test ./...
```
Expected: BUILD passes, all tests PASS. If a fake store somewhere else in the codebase (outside `service/daily/`) still implements the dropped method, Go is fine — extra methods don't break interface satisfaction. Only failure modes are: missing required method, or broken test assertion.

If anything fails: read the error, fix surgically (don't widen scope), re-run.

- [ ] **Step 3.8: Commit**

```bash
git add backend/internal/service/daily/ backend/internal/repository/
git commit -m "$(cat <<'EOF'
fix(backend): atomically bump counters.started on first GET (#184)

PLAY-row creation moves from a single conditional PutItem to a two-leg
TransactWriteItems composed in service/daily/. Leg 0 is the existing
PUT-with-attribute_not_exists guard; leg 1 does ADD counters.started 1
on the daily schedule row. The "did anyone open yesterday's puzzle"
signal is now real, and combined with the recycle-gate fix from the
previous commit closes #184.

Removes PutPlayStartedIfAbsent from the Store interface, the production
repository, and all daily-service fake stores. Race-loser branch
preserved via IsConditionalCheckFailureOnLeg(_, 0).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4 — Frontend: `formatTime` HH:MM:SS overflow

**Files:**
- Modify: `frontend/src/shared/game/components/GameBoard.tsx:16-20` (function + export)
- Create: `frontend/src/shared/game/components/formatTime.test.ts`

- [ ] **Step 4.1: Write failing tests**

Create `frontend/src/shared/game/components/formatTime.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { formatTime } from './GameBoard';

describe('formatTime', () => {
  it('formats zero seconds as 00:00', () => {
    expect(formatTime(0)).toBe('00:00');
  });

  it('formats sub-minute as 00:SS', () => {
    expect(formatTime(59)).toBe('00:59');
  });

  it('formats exactly one minute as 01:00', () => {
    expect(formatTime(60)).toBe('01:00');
  });

  it('formats a few minutes as MM:SS', () => {
    expect(formatTime(303)).toBe('05:03');
  });

  it('formats just under an hour as 59:59', () => {
    expect(formatTime(3599)).toBe('59:59');
  });

  it('formats exactly one hour as 1:00:00', () => {
    expect(formatTime(3600)).toBe('1:00:00');
  });

  it('formats one hour two minutes three seconds as 1:02:03', () => {
    expect(formatTime(3723)).toBe('1:02:03');
  });

  it('formats 5h 50m 05s as 5:50:05 (regression: was 350:05)', () => {
    expect(formatTime(21005)).toBe('5:50:05');
  });

  it('formats ten hours as 10:00:00', () => {
    expect(formatTime(36000)).toBe('10:00:00');
  });
});
```

- [ ] **Step 4.2: Run tests to confirm RED**

```
cd frontend && npx vitest run src/shared/game/components/formatTime.test.ts
```
Expected: import fails (`formatTime` not exported) or all tests fail.

- [ ] **Step 4.3: Export + reimplement `formatTime`**

Edit `frontend/src/shared/game/components/GameBoard.tsx:15-20`:

```ts
/** Format seconds as MM:SS (under 1h) or H:MM:SS (1h+). Hour digit is
 * un-padded; presence of the leading `H:` itself signals hour-scale. */
export function formatTime(seconds: number): string {
  const s = seconds % 60;
  const m = Math.floor(seconds / 60) % 60;
  const h = Math.floor(seconds / 3600);
  const ss = String(s).padStart(2, '0');
  const mm = String(m).padStart(2, '0');
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}
```

- [ ] **Step 4.4: Run tests to confirm GREEN**

```
cd frontend && npx vitest run src/shared/game/components/formatTime.test.ts
```
Expected: 9 passing.

- [ ] **Step 4.5: Run the full frontend suite to catch regressions**

```
cd frontend && npm run test
```
Expected: all PASS. If a snapshot test on a rendered timer string breaks (e.g. a component test expecting "350:05"), update the snapshot to the new format and confirm visually it's correct.

- [ ] **Step 4.6: Commit**

```bash
git add frontend/src/shared/game/components/GameBoard.tsx frontend/src/shared/game/components/formatTime.test.ts
git commit -m "$(cat <<'EOF'
fix(frontend): formatTime renders HH:MM:SS past 99 minutes

Surfaced when investigating #185 (closed as works-as-designed): a 5h50m
elapsed displayed as "350:05", which reads as a 4-digit number rather
than hours-and-minutes. Sub-hour values keep MM:SS; 1h+ switches to
H:MM:SS with un-padded hours.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5 — Verification + PR

- [ ] **Step 5.1: Full test suite + lint**

```
task test
task lint:backend
```
Expected: green across the board.

- [ ] **Step 5.2: Manual smoke (optional but recommended)**

`task dev:up`, open the daily in a browser, confirm:
- Timer displays as `MM:SS` initially.
- After 1h+ (use playwright-cli with a fake clock, or just wait), displays `H:MM:SS`.
- The daily-flow e2e (`task e2e:up && task test:e2e -- daily-flow.spec.ts`) passes.

- [ ] **Step 5.3: Push + open PR**

```bash
git push -u origin feat/184-daily-recycle-counter
gh pr create --title "Daily recycle gated on Started==0 + counter wiring (#184)" --body "$(cat <<'EOF'
## Summary

Closes #184. Bundles a small `formatTime` HH:MM:SS overflow fix (one-time exception, design discussed in the spec).

- Atomic two-leg `TransactWriteItems` on first GET: PUT PLAY + `ADD counters.started 1`.
- `chooseFinalizeTarget` gates recycle on `Solved == 0 && Started == 0` (was `Solved == 0` only).
- `formatTime` renders `H:MM:SS` for ≥1h (was `MMM:SS` overflow).

## Key Decisions

- **"Started" = GET-creates-PLAY.** Simplest match to "anyone opened it"; theoretical bot-disqualifies-puzzle concern noted in spec, not material at current scale.
- **Atomic TransactWriteItems** instead of best-effort separate Update — clean invariant, free idempotency under repeated GETs.
- **`PutPlayStartedIfAbsent` removed** from Store, production repo, and all fakes — composing the TX in service/ is consistent with `FinalizeDaily` per the multi-leg-tx-in-service architecture rule.
- **No retro-fix for today's already-recycled schedule** (`DAILY#2026-05-18`). Tomorrow's behaviour is correct.
- **Timer formatting bundled** as a one-time exception, not a pattern.

## Spec

`docs/superpowers/specs/2026-05-18-daily-recycle-counter-design.md`

## Test plan

- [x] `chooseFinalizeTarget` table-driven tests (Started>0 → confirm, Started==0 & Solved==0 → recycle).
- [x] `materializePlayRow` issues one TransactWriteItems with two legs; race-loser re-reads existing PLAY; second GET short-circuits without TX.
- [x] `formatTime` unit tests cover 0, 59, 60, 3599, 3600, 3723, 21005, 36000.
- [ ] Manual: daily flow renders, timer crosses 1h boundary correctly.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 5.4: Hand off to `code-review-final` + `security-review-final` per the change workflow**

The change-workflow gate requires `requesting-code-review` + the architecture skill before the PR is merge-ready. The security trigger does NOT fire (no auth/middleware/handler/IAM/dep changes), but let `code-review-final` decide whether to escalate.

---

## Out-of-plan housekeeping (only if hit)

If the pre-push hook complains about something unrelated to this branch (e.g. golangci-lint version skew), surface it as a separate issue and either fix in a small follow-up PR or note in the PR body. Do NOT bundle unrelated fixes.
