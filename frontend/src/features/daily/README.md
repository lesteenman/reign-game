# features/daily

The daily puzzle feature — one puzzle per UTC calendar day, with a speed-based leaderboard.

## Contents

```
hooks/
  useDailyPuzzle.ts         TanStack useQuery: IDB short-circuit → getDaily,
                            surfaces either solved-payload or playing-payload.
  useSubmitDaily.ts         TanStack useMutation: submitDailyResult + IDB persist;
                            normalizes 409 to a synthetic success.
screens/
  DailyFlow.tsx             Composes the two hooks; renders 7 states driven by
                            query/mutation flags (loading / error / IDB-solved /
                            playing / submitting / submit-solved / submit-error).
  DailyGameBoard.tsx        Adapts the daily payload to the shared GameBoard contract.
  PostCompletionScreen.tsx  Post-solve view: solve time, rank, countdown to next puzzle.
```

## State pipeline

Pre-#176 daily-half slice, `DailyFlow` hand-rolled a 6-arm `useState<FlowState>` discriminated union + a 76-line `useEffect` driving the load cascade + a `stateRef`-stabilised `handleSolved` callback. The migration replaced all of that with `useDailyPuzzle` (read) + `useSubmitDaily` (write); render branches read directly from query/mutation flags. Server-state branching is now uniform across the daily and curation flows.

## Entry point

`screens/DailyFlow` is mounted directly by the router (`src/app/router.tsx`) when the URL carries `?flow=daily`. It owns the fetch, submit, and state-machine logic for one daily session.

## Wiring

`src/app/router.tsx`'s inline `<PlayRoute>` dispatcher reads `?flow=` from the URL and renders `<DailyFlow />` for `flow=daily` (curation goes to `features/curation/pages/PlayPuzzlePage`). The dispatch lives at the router level so neither feature imports the other — see PR #N (GamePage split) for the BR rationale.

These files are `screens/`, not `pages/`, because `DailyFlow` is mounted as the `/play` element's content but not as its own router-pattern target — the router only knows about `/play`.
