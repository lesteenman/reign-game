# `src/components/game/`

Game-specific (non-grid) UI surfaces. One production file plus one unit test.

## Responsibility

Components that participate in the gameplay surface but aren't part of the grid itself. Today this is one component: the admin verdict surface that asks "How was that puzzle?" after a solve or skip.

## Data flow

- **In:** Rendered by `GamePage.tsx` (twice) — inside the completion overlay (`variant="completion"`) and inside the skip modal (`variant="skip"`).
- **Out:** Calls `submitVerdict` (verdictService) and `updatePuzzleStatus` (puzzleService) directly. **This is the known leaf-I/O architecture violation.**

## Files

- **`VerdictSurface.tsx`** — Two-variant component (completion / skip) discriminated by props. Owns a four-state submission state machine (`idle | submitting | done | error`) and a sessionStorage idempotency cache keyed by `'reign:verdict:submitted'`. Completion variant renders Good-puzzle / Bad-puzzle buttons; skip variant renders Cancel / I-hate-this / Just-skip buttons. After a successful submit the puzzleId is cached so a re-mount with the same puzzle hides the surface entirely (FB-07).

## State management

- Local: `useState<SubmissionStatus>` (state machine), `useState(() => isSubmitted(puzzleId))` (lazy cache check at mount).
- Persistent: `sessionStorage['reign:verdict:submitted']` — array of puzzleIds.

## Rules specific to this directory

- **Variant-specific button-callbacks discriminate the `props` union at the callsite** (`if (props.variant !== 'skip') return Promise.resolve();`). The destructured `variant` local at the top of the function does NOT propagate type-narrowing to `props` itself — this is the documented pattern in the file.

## Known architecture violations

- **Leaf I/O:** Imports `submitVerdict` from `services/verdictService` and `updatePuzzleStatus` from `services/puzzleService`. Per the architecture rules, leaf components consume hooks; hooks own the I/O.
- **Submission state machine inline:** The `runSubmission` wrapper + the four-state machine should be a `useVerdictSubmission()` hook.

Track 3 fix: extract to `features/curation/components/VerdictSurface.tsx` and replace the direct service calls with `useSubmitVerdict()` + `useUpdatePuzzleStatus()` (both TanStack mutations).
