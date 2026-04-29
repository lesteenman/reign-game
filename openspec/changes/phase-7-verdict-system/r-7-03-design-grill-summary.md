# R-7-03 Design Grill Summary

Final design artifact for Phase 7's third slice: per-flow IndexedDB storage. Pairs with `specs/storage.md` (the rule-level spec) and the R-7-03 section in `tasks.md` (the implementation breakdown).

## Problem

Pre-Phase-7 frontend persists in-progress play state in a single IndexedDB row (`gameState` store, `id: 'current'`). Switching pools — e.g., from a 5×5 standard puzzle to a 7×7 standard puzzle — overwrites the prior in-progress state without any user signal. R-7-02 added an explicit Skip button so the *only* path to a `skipped` status is intentional; per-flow keying is the storage half of that contract.

## Locked decisions

| ID | Decision | Choice |
|---|---|---|
| D1 | Key shape | Composite-string `id` value `"{flowType}:{flowId}"` on the existing `gameState` store. Not compound IDB key, not store-per-flowType. |
| D2/D3 | Migration of existing rows | Bump `DB_VERSION` to 2; clear `gameState` store on upgrade. No row-level migration. Audience is admins; one-shot loss is acceptable. |
| D4 | `?new=true` URL contract | Removed. URL specifies `flow` + flow-identifying params; `loadState` decides resume vs. fetch. Combined with clear-on-solve (D9), the same URL after a completion naturally fetches fresh — one code path, no flag. |
| D5 | Two rows per slot | Structurally impossible (`put` is upsert by keyPath). Locked invariant. |
| D6 | `flowType` type | Typed union `'curation' \| 'daily' \| 'pack'` declared now. `'daily'` / `'pack'` pre-declared so future producers extend the same union. |
| D7 | Storage size | Worst case ~270 KB total across all plausible curation pools. IDB quotas are in GB. Non-issue. |
| D8 | Cross-tab safety | Last-write-wins, unchanged from today. No locking added. |
| D9 | Clear-on-solve | Solved completion clears the slot. Without this, ST-06's resume path would resume a solved puzzle. |

## Decisions handed to the human

D2/D3 (migration strategy) and D4 (resume UX) were the two flagged for explicit human decision. Both were resolved in the same conversation:

- D2/D3: confirmed graceful drop (β).
- D4: refined further than the original "(ii) auto-resume on tile click; explicit `new=true` for Play Again" recommendation. Final shape: **no `?new=true` flag at all.** The URL specifies the flow + mode; the storage layer decides. Clear-on-solve makes this lossless.

The simplification eliminates a URL contract that has been a small footgun (it's possible today to navigate to `/play?new=true` without size / mode and get unexpected behavior). One code path is better than two.

## Spec drift note

R-7-02's `tasks.md` (line 167) claimed CurationPage would pass `flow=curation` in the URL. The wiring was specced but never shipped — CurationPage still navigates with `?new=true&size=N&mode=M`. R-7-03 is the slice that actually implements the URL contract envisioned in R-7-02. Not a history rewrite; surfaced in the R-7-03 PR body as context.

## Glossary additions

Two new terms added to `GLOSSARY.md` in the **Flows & Storage** section:

- **Flow** — top-level mode of play. Determines which API surface produces puzzles and which storage slot tracks in-progress state. Typed union `'curation' | 'daily' | 'pack'`. Carried as the `flow` query parameter on `/play` URLs and as the first segment of the IDB Flow Slot key.
- **Flow Slot** — single `(flowType, flowId)` entry in the `gameState` IDB store, composite-string key. Holds at most one in-progress puzzle. A solved slot is cleared (no resume of solved puzzles).

## Out of scope (deferred)

- **PuzzleSelector "Resume" vs "New" affordance.** Showing per-pool resume state in the picker requires reading IDB before render and adding visual states. Doubles the slice; punted.
- **Multi-tab coordination.** Same as today — last-write-wins. Phase-9-or-later if it ever matters.
- **Cross-flow resume browser** (e.g., a "continue daily 2026-04-29 OR continue curation 5×5" picker). Premature — only curation is wired this phase.

## Implementation notes (for the slice that follows)

- Branch off `main`, not off `feat/R-7-03-exploration`. This branch holds design artifacts only.
- TDD: red/green for the `db.ts` upgrade behavior first (the IDB `onupgradeneeded` handler is the highest-risk change). Then `useGameStorage` API surface. Then GamePage wiring. Then sweep `?new=true` from production code and tests.
- Manual upgrade verification: deploy to dev, confirm an admin's first-after-deploy navigation hits the wipe path (no resume).
