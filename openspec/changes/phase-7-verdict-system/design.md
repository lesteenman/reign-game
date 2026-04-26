# Phase 7 Design: Verdict System

## 1. Architecture Overview

```
   Browser (admin)              Our API                    DynamoDB
  ───────────────              ─────────                  ──────────
       │                           │                            │
       │  Plays puzzle to          │                            │
       │  completion / skip        │                            │
       │                           │                            │
       │  Renders verdict          │                            │
       │  surface (admin role      │                            │
       │  required)                │                            │
       │                           │                            │
       │  PUT /api/admin/puzzles/  │                            │
       │      {id}/verdict         │                            │
       ├──────────────────────────▶│                            │
       │   { value, playTimeMs,    │  RequireAuth + RequireAdmin│
       │     outcome, clientVer }  │  (Phase 6 chain)           │
       │                           │                            │
       │                           │  1. UpdateItem (verdict    │
       │                           │     row, idempotent)       │
       │                           ├───────────────────────────▶│
       │                           │                            │
       │                           │  2. Recompute summary      │
       │                           │     UpdateItem on          │
       │                           │     PuzzleRecord           │
       │                           ├───────────────────────────▶│
       │                           │                            │
       │  200 + summary            │                            │
       │◀──────────────────────────┤                            │
       │                           │                            │
```

Browser only knows about `/api/admin/*`. The Phase 6 middleware chain enforces admin role. The handler does two writes to DynamoDB: one to the verdict row family, one to the puzzle's summary projection. Read paths (admin pool, future analyst) read the summary directly off `PuzzleRecord`.

## 2. Package Layout

### Backend

```
backend/
├── internal/
│   ├── handler/
│   │   ├── verdict.go             NEW — VerdictHandler
│   │   └── verdict_test.go        NEW — auth matrix + validation + idempotency
│   └── repository/
│       └── puzzle.go              CHANGED:
│                                   - remove `Verdict string` from PuzzleRecord
│                                   - add VerdictSummary struct + field
│                                   - add VerdictRecord struct
│                                   - add PutVerdict / RecomputeVerdictSummary methods
└── cmd/
    ├── api/main.go                CHANGED — register the new admin route
    └── genfixtures/main.go        CHANGED — drop the verdict:"none" seed
```

### Frontend

```
frontend/src/
├── components/
│   └── auth/
│       └── role.ts                UNCHANGED — reused for verdict gating
├── pages/
│   ├── GamePage.tsx               CHANGED — render verdict surface on
│   │                                completion + post-skip; gate by admin role
│   └── GamePage.test.tsx          CHANGED — three-state visibility coverage
├── services/
│   ├── verdictService.ts          NEW — submitVerdict(puzzleId, value, ...)
│   └── verdictService.test.ts     NEW — happy / error / 401-403 silent-hide
└── components/
    └── game/
        ├── VerdictSurface.tsx     NEW — two-button widget; isolated for testing
        └── VerdictSurface.test.tsx NEW
```

## 3. Backend — Repository

### PuzzleRecord schema change

```go
// VerdictSummary is the denormalized verdict projection on PuzzleRecord.
// Read-side consumers (admin pool, analysis agent) use this directly to
// avoid a fanout query into the verdict row family.
type VerdictSummary struct {
    Up            int    `dynamodbav:"up"            json:"up"`
    Down          int    `dynamodbav:"down"          json:"down"`
    LastUpdatedAt string `dynamodbav:"lastUpdatedAt" json:"lastUpdatedAt"`
}

// PuzzleRecord — the Verdict string field is removed; VerdictSummary
// replaces it as the only verdict-shaped attribute on the row.
type PuzzleRecord struct {
    // … existing fields …
    VerdictSummary VerdictSummary `dynamodbav:"verdictSummary"`
    // (Verdict string `dynamodbav:"verdict"` is REMOVED.)
}
```

### VerdictRecord — new row family

```go
// VerdictRecord is one rater's verdict on one puzzle.
//
// PK: "VERDICT#{size}#{mode}#{puzzleId}"
// SK: "{raterRole}#{raterId}"
//
// Re-submission by the same (puzzleId, raterId) overwrites — at most one
// row per (puzzle, rater) pair. Idempotent PUT semantics.
type VerdictRecord struct {
    PuzzleID      string `dynamodbav:"-"             json:"puzzleId"`
    GridSize      int    `dynamodbav:"-"             json:"gridSize"`
    Mode          string `dynamodbav:"-"             json:"mode"`
    RaterID       string `dynamodbav:"-"             json:"raterId"`
    RaterRole     string `dynamodbav:"-"             json:"raterRole"`
    Value         string `dynamodbav:"value"         json:"value"` // "up" | "down"
    PlayTimeMs    int64  `dynamodbav:"playTimeMs"    json:"playTimeMs"`
    Outcome       string `dynamodbav:"outcome"       json:"outcome"` // "solved" | "skipped"
    ClientVersion string `dynamodbav:"clientVersion" json:"clientVersion"`
    SubmittedAt   string `dynamodbav:"submittedAt"   json:"submittedAt"`
}
```

### New repository methods

```go
// PutVerdict writes a verdict row, overwriting any existing row for the
// same (puzzleId, raterId) pair. The summary is NOT updated here — the
// caller (handler) calls RecomputeVerdictSummary as a separate step so
// the two writes are testable independently.
func (r *PuzzleRepository) PutVerdict(ctx context.Context, v *VerdictRecord) error

// ListVerdictsForPuzzle returns every verdict row for a puzzle. Used by
// RecomputeVerdictSummary; exposed publicly so the analysis agent
// (Phase 9) and any future audit tools can read the row family directly.
func (r *PuzzleRepository) ListVerdictsForPuzzle(ctx context.Context, size int, mode, puzzleID string) ([]VerdictRecord, error)

// RecomputeVerdictSummary reads every verdict row for a puzzle, sums
// the values, and writes the result onto PuzzleRecord.verdictSummary.
// Last-write-wins on the summary projection — a concurrent vote that
// races may produce a millisecond-scale lag, which is documented as
// acceptable in design.md §6 Risks.
func (r *PuzzleRepository) RecomputeVerdictSummary(ctx context.Context, size int, mode, puzzleID string) (VerdictSummary, error)
```

The summary recompute reads the row family and writes the result — it does NOT do an in-place increment. Reasoning: an in-place increment is correct for "first-ever vote" but wrong on overwrite (admin flips up → down → up). A read-then-write is O(votes-per-puzzle) which is bounded at a few rows for the foreseeable future. When the public-rater role lands and the row count grows, an in-place delta-write becomes preferable; that's a Phase 11+ optimization.

### Why not a single transaction across both rows

DynamoDB supports `TransactWriteItems` — atomic across multiple rows. We deliberately don't use it here:

- The verdict row family is the source of truth. The summary is a cached projection. Surface-of-truth misalignment is recoverable by re-running RecomputeVerdictSummary.
- TransactWriteItems doubles write cost. At single-admin scale that's negligible, but we'd rather build the simple shape and add the transaction guard if the lag becomes visible.
- Phase 9's analysis agent reads the row family for analysis work — it's not summary-dependent.

## 4. Backend — Handler

### Endpoint

`PUT /api/admin/puzzles/{id}/verdict?size={size}&mode={mode}`

The `size` and `mode` query params construct the partition key (mirroring `PUT /api/puzzles/{id}/status`'s convention — see `backend/internal/handler/status.go`). Without them, the handler can't address the puzzle row.

Request body:

```json
{
  "value": "up" | "down",
  "playTimeMs": 47230,
  "outcome": "solved" | "skipped",
  "clientVersion": "abc123def"
}
```

Response on success: `200 OK` with `{ "summary": { "up": N, "down": M, "lastUpdatedAt": "..." } }`.

Response on failure:

| Condition | Response |
|---|---|
| No session cookie | 401 (from `RequireAuth`) |
| Signed-in non-admin | 403 (from `RequireAdmin`) |
| `value` not in `{up, down}` | 400 `invalid_params` |
| `outcome` not in `{solved, skipped}` | 400 `invalid_params` |
| `playTimeMs` < 0 or NaN/Inf encoded as a string would not happen — JSON number; reject negative | 400 `invalid_params` |
| Missing `size` or `mode` query param | 400 `invalid_params` |
| Puzzle not found in DynamoDB | 404 `not_found` |
| DynamoDB write failure | 500 `internal_error` (handler logs the underlying error) |

### Handler skeleton

```go
// verdict_request is the JSON body for PUT /api/admin/puzzles/{id}/verdict.
type verdictRequest struct {
    Value         string `json:"value"`
    PlayTimeMs    int64  `json:"playTimeMs"`
    Outcome       string `json:"outcome"`
    ClientVersion string `json:"clientVersion"`
}

type verdictResponse struct {
    Summary repository.VerdictSummary `json:"summary"`
}

// VerdictRepo defines the repository methods VerdictHandler depends on.
type VerdictRepo interface {
    PutVerdict(ctx context.Context, v *repository.VerdictRecord) error
    RecomputeVerdictSummary(ctx context.Context, size int, mode, puzzleID string) (repository.VerdictSummary, error)
    GetPuzzle(ctx context.Context, size int, mode, puzzleID string) (*repository.PuzzleRecord, error) // NEW: needed for 404 check
}

func VerdictHandler(repo VerdictRepo) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Parse path / query / body. Validate value, outcome, playTimeMs ≥ 0.
        // 2. Read the authenticated user from auth.UserFromContext(r.Context()).
        //    raterId   = u.ID
        //    raterRole = "admin"   (RequireAdmin already verified this)
        // 3. Confirm puzzle exists (GetPuzzle → 404 if absent).
        // 4. Compose VerdictRecord with submittedAt = time.Now().UTC().Format(time.RFC3339).
        // 5. PutVerdict.
        // 6. RecomputeVerdictSummary; write the result back via UpdateItem on PuzzleRecord.
        // 7. Respond with verdictResponse{Summary: <recomputed>}.
        // 8. On any error, log via the existing httperr.WriteError convention.
    }
}
```

### Why `raterRole = "admin"` is hard-coded today

The locked decision is admin-only voting. `RequireAdmin` guarantees the caller has the admin role. Storing `raterRole` on the row even when only one role exists costs nothing and means the schema is right when the public-rater role lands. The hard-coded constant becomes a reading from `user.publicMetadata.role` at that point — a one-line change.

### Idempotency proof in tests

The handler test exercises:

1. PutVerdict with `value=up` → verify summary = `{up: 1, down: 0}`.
2. Same admin re-submits with `value=down` → verify summary = `{up: 0, down: 1}` (overwrite, not accumulate).
3. Same admin re-submits with `value=up` again → summary = `{up: 1, down: 0}`.
4. A second admin (different `raterId`) submits `value=up` → summary = `{up: 2, down: 0}`.

## 5. Frontend — Verdict Surface

### Component layout

```tsx
// frontend/src/components/game/VerdictSurface.tsx
//
// Two-button widget. Renders only when the caller has already verified
// the user is admin — the component does NOT inspect Clerk state itself,
// so it can be rendered in tests without a ClerkProvider.

interface VerdictSurfaceProps {
  puzzleId: string
  gridSize: number
  mode: Mode
  outcome: 'solved' | 'skipped'
  playTimeMs: number
  onSubmitted?: () => void
}

export function VerdictSurface(props: VerdictSurfaceProps) {
  const [state, setState] = useState<'idle' | 'submitting' | 'done' | 'error'>('idle')
  const [submittedValue, setSubmittedValue] = useState<'up' | 'down' | null>(null)

  async function submit(value: 'up' | 'down') {
    setState('submitting')
    setSubmittedValue(value)
    try {
      await submitVerdict({ ...props, value })
      setState('done')
      props.onSubmitted?.()
    } catch {
      setState('error')
    }
  }

  if (state === 'done') {
    return <p>Thanks — recorded.</p>
  }
  if (state === 'error') {
    return (
      <>
        <p>Couldn't save your verdict. Try again?</p>
        <button onClick={() => submit(submittedValue!)}>Retry</button>
      </>
    )
  }
  return (
    <div>
      <button disabled={state === 'submitting'} onClick={() => submit('up')}>Good puzzle</button>
      <button disabled={state === 'submitting'} onClick={() => submit('down')}>Bad puzzle</button>
    </div>
  )
}
```

### Role-gated rendering inside GamePage

Inside `GamePage.tsx`'s `GameBoard` component, the verdict surface renders next to the existing "Play Again" / "Home" buttons on the completion overlay, and inside a small post-skip transient state.

```tsx
import { useUser } from '@clerk/react'
import { getClerkUserRole } from '../components/auth/role'

// Inside GameBoard:
const { user } = useUser()
const isAdmin = getClerkUserRole(user?.publicMetadata) === 'admin'

// Inside the completion overlay JSX, after the Play Again / Home buttons:
{isAdmin && (
  <VerdictSurface
    puzzleId={puzzle.puzzleId}
    gridSize={puzzle.gridSize}
    mode={puzzle.mode}
    outcome="solved"
    playTimeMs={completionTime * 1000}
  />
)}
```

The skip flow currently navigates straight back to the landing page when the user skips. The slice's frontend work adds an interstitial "you skipped — verdict?" state inside `GamePage` for admins only. Non-admins skip → home as today.

### Service layer

```ts
// frontend/src/services/verdictService.ts
import { apiPut, ApiError } from './api'
import type { Mode } from '../engine/types'

export interface SubmitVerdictArgs {
  puzzleId: string
  gridSize: number
  mode: Mode
  value: 'up' | 'down'
  outcome: 'solved' | 'skipped'
  playTimeMs: number
}

const CLIENT_VERSION = import.meta.env.VITE_GIT_SHA || 'dev'

export async function submitVerdict(args: SubmitVerdictArgs): Promise<void> {
  try {
    await apiPut(
      `/api/admin/puzzles/${args.puzzleId}/verdict`,
      {
        value: args.value,
        outcome: args.outcome,
        playTimeMs: args.playTimeMs,
        clientVersion: CLIENT_VERSION,
      },
      { size: String(args.gridSize), mode: args.mode },
    )
  } catch (err) {
    if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
      // Defense-in-depth — the caller's role check should have prevented
      // this. Swallow silently so the play flow isn't disrupted.
      return
    }
    throw err
  }
}
```

`VITE_GIT_SHA` is injected at build time by the existing CD workflow (or defaulted to `dev` in local dev). No infra change needed — the env var is read in the same place as `VITE_API_URL` and `VITE_CLERK_PUBLISHABLE_KEY`.

### Once-per-session local cache

```ts
// inside VerdictSurface.tsx
const SESSION_KEY = 'reign:verdict:submitted'
function loadSubmittedSet(): Set<string> {
  try { return new Set(JSON.parse(sessionStorage.getItem(SESSION_KEY) || '[]')) } catch { return new Set() }
}
```

If the puzzle ID appears in `submittedSet`, the surface renders nothing. Best-effort UX guard against the buttons re-appearing on a re-served puzzle in the same tab. The backend's idempotent overwrite is the source of truth.

## 6. Risks

| Risk | Mitigation |
|------|------------|
| Verdict-summary projection lags the row family by milliseconds under concurrent admin votes | Document the source-of-truth contract: row family is canonical, summary is cached. Phase 9 recomputes from rows when correctness matters. Single-admin scale makes the lag invisible today. |
| Removing the legacy `Verdict string` field breaks an unseen consumer | Grep sweep mandatory in R-081's PR. The known consumers are `cmd/genfixtures/main.go`, `worker/generator.go`, and tests — all updated in the slice. No external consumers (no Lambda triggers, no analytics export). |
| An admin re-votes after their role is revoked mid-session | Backend middleware fails closed (403). Frontend cosmetic gating + once-per-session cache are best-effort UX, not security boundaries. |
| Verdict captured on a puzzle that doesn't exist in DynamoDB (synthetic / replayed) | Handler reads `PuzzleRecord` first. Absent → 404. Cheap defense. |
| `clientVersion` is empty on a developer build | Defaulted to `'dev'` via the `VITE_GIT_SHA` env var. Rows from local-dev verdicts carry `clientVersion: "dev"`, distinguishable from production rows. |
| `playTimeMs` is wrong (timer paused / restored across visibility events) | Acceptable — `useTimer.elapsed` already handles visibility-change pause / resume. The verdict captures the player's actual on-task time, which is what R-084 wants. Edge cases (timer drift) are documented in `useTimer.ts` and are tolerable for calibration. |
| Verdict surface renders for a brief flicker before Clerk loads `user` | The `useUser` hook returns `isLoaded: false` initially — the verdict surface only renders inside a branch that already requires `user.publicMetadata`. If `isLoaded` is false, the role check returns `''` (empty string) and the surface stays hidden. No flicker. |

## 7. Key Decisions (autonomous-mode summary)

The autonomous walk landed on these defaults. Each one has the reversibility cost called out so the human can reverse the decision cheaply on review.

1. **Per-rater rows in the existing puzzle-pool table, with a summary projection on PuzzleRecord.** Reversal cost: one repository refactor — moderate. Reversed by going to a separate `verdict-pool` table or to the append-only log shape (Approach E in `design-grill-summary.md`).
2. **Skip stays a status; verdict is up/down only.** Reversal cost: low. Reversed by adding `"skip"` as a third valid value to the verdict request, but doing so re-creates the parallel-concept problem with `status="skipped"`.
3. **Play-time captured from day one.** Reversal cost: zero — the field is additive. The cost of NOT doing it is high (corpus split into pre-/post-instrumentation slices).
4. **Existing `Verdict string` field removed in this slice.** Reversal cost: low (re-add the field as a deprecated alias). The conservative alternative is to keep the field one phase longer for rollback ease — flagged as OPEN in `design-grill-summary.md` for human override.
5. **Verdict surface UI is identical on completion vs skip.** Reversal cost: zero — the schema's `outcome` field already distinguishes them; only the UI changes.
6. **No conditional-write race protection on summary updates.** Reversal cost: low — add `ConditionExpression` to the summary update. Not done now because single-admin scale doesn't see the race.
7. **No GSI on `(verdictBucket, createdAt)` for the analyst's "find downvoted puzzles" query.** Reversal cost: medium — adds a Terraform change and a backfill of the GSI key on existing rows. Phase 9 owns this decision.
8. **`raterRole` hard-coded to `"admin"` in the handler.** Reversal cost: zero — replace the literal with a read from `user.publicMetadata.role` when the public-rater role lands.

## 8. Open Questions

Two items where the autonomous walk picked a default but the human override is cheap:

1. **Verdict surface on skip — same UI as on completion, or a separate compact widget?** Default: same UI. Reversal cost: zero — the `outcome` field on the row distinguishes them, only the UI changes.
2. **Legacy `Verdict string` field — remove now, or keep one phase longer for rollback ease?** Default: remove now (it's unused at read time). Reversal cost: low. Conservative alternative is "keep as `verdict_legacy` for one phase, remove in Phase 8."

Both are documented in `design-grill-summary.md` so the human can flip them on review without re-walking the design.

## 9. Testing Strategy

### Backend unit + integration

- `backend/internal/repository/puzzle_test.go` — extend with VerdictRecord round-trip, RecomputeVerdictSummary correctness (counts up/down), idempotent overwrite proof, ListVerdictsForPuzzle returning expected rows.
- `backend/internal/handler/verdict_test.go` — new file:
  - Auth matrix iteration (anonymous → 401, user → 403, admin → 200) using the existing `mountAdminWithAuth` helper.
  - Validation: invalid `value` → 400, missing `size` / `mode` → 400, negative `playTimeMs` → 400.
  - 404 path: puzzle not found → 404.
  - Happy path: 200 response, summary returned, row written, summary projection updated on PuzzleRecord.
  - Idempotency: second submission overwrites, third submission overwrites again, summary reflects last value.

### Frontend unit (Vitest)

- `frontend/src/components/game/VerdictSurface.test.tsx`:
  - idle state renders both buttons.
  - clicking "Good puzzle" calls `submitVerdict` with `value: 'up'`.
  - submission success → "Thanks — recorded" rendered.
  - submission 5xx → error message + retry button rendered.
  - submission 401/403 → silently swallowed (success-like state, no error).
- `frontend/src/services/verdictService.test.ts`: PUT body, query params, error mapping.
- `frontend/src/pages/GamePage.test.tsx` — extend with role-gated visibility:
  - admin role + completion → verdict surface visible.
  - user role + completion → verdict surface absent.
  - anonymous + completion → verdict surface absent.

### E2E (Playwright)

Out of scope this phase. Phase 7 ships behind admin auth, and the e2e suite (`frontend/playwright/e2e/`) does not yet wire a real Clerk admin session — that's a Phase 6 follow-up that happens at the same time as R-080's e2e expansion. Documented as a Phase 7b candidate, not a Phase 7 commit.

## 10. Observability

- Backend logs:
  - `verdict: write puzzle=<id> rater=<sub> value=<up|down>` on every successful write.
  - `verdict: summary recomputed puzzle=<id> up=<n> down=<m>` after each summary write.
  - `WARN: verdict: summary recompute failed puzzle=<id> err=<reason>` if the second UpdateItem fails (the row write succeeded; the summary lags).
- Frontend: no logging beyond the existing API error path.
- No new metrics. If verdict volume becomes interesting (i.e., when the public-rater role lands), add a CloudWatch metric filter on the `verdict: write` log pattern.

## 11. Security

- Backend middleware is the source of truth for authorization. Frontend gating is cosmetic.
- The verdict endpoint sits inside the Phase 6 admin route group; new routes added inside the group inherit auth by construction (BM-05).
- `clientVersion` is captured server-side from the request body. A malicious admin could lie about it, but the threat model is "honest admins" — there is no incentive to misreport.
- The `playTimeMs` value is captured from the trusted-but-not-validated frontend timer. A malicious admin could submit a fabricated value, but again the threat model is "honest admins." When the public-rater role lands, server-side reasonability bounds (e.g., `playTimeMs < 24 * 3600 * 1000`) become defense-in-depth.

## 12. Roadmap Effects

- Phase 7 keeps its current header (`Verdict System`) and slice IDs (R-081, R-082).
- No phase renumbering required.
- No new slice IDs claimed. The docs sweep (glossary + PROJECT_STRUCTURE.md update + ROADMAP flips) folds into R-082's PR — splitting it into a third slice (analogous to R-08C in Phase 6) is dead weight for two-slice work. R-082's PR carries the full slice close-out per `tasks.md`.
