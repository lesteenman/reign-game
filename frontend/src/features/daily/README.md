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

`screens/DailyFlow` is the sub-flow component composed by `pages/GamePage` when the URL carries `?flow=daily`. It owns the fetch, submit, and state-machine logic for one daily session.

## Wiring

Mounted by `pages/GamePage` when the URL has `?flow=daily`. GamePage is the router-mounted component; DailyFlow and its siblings are sub-flow screens that live here rather than in `pages/`.
