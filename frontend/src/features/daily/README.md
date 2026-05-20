# features/daily

The daily puzzle feature — one puzzle per UTC calendar day, with a speed-based leaderboard.

## Contents

```
screens/
  DailyFlow.tsx             Entry point. State machine: loading → playing → submitting → solved.
  DailyGameBoard.tsx        Adapts the daily payload to the shared GameBoard contract.
  PostCompletionScreen.tsx  Post-solve view: solve time, rank, countdown to next puzzle.
```

## Entry point

`screens/DailyFlow` is mounted directly by the router (`src/app/router.tsx`) when the URL carries `?flow=daily`. It owns the fetch, submit, and state-machine logic for one daily session.

## Wiring

`src/app/router.tsx`'s inline `<PlayRoute>` dispatcher reads `?flow=` from the URL and renders `<DailyFlow />` for `flow=daily` (curation goes to `features/curation/pages/PlayPuzzlePage`). The dispatch lives at the router level so neither feature imports the other — see PR #N (GamePage split) for the BR rationale.

These files are `screens/`, not `pages/`, because `DailyFlow` is mounted as the `/play` element's content but not as its own router-pattern target — the router only knows about `/play`.
