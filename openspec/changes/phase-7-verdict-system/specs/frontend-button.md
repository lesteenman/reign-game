# Spec: Frontend Verdict Surface

The visible-and-functional frontend contract for the verdict UI.

## FB-01: Verdict surface renders only for `publicMetadata.role === 'admin'`

**Rule.** The verdict surface (two buttons + state machine) renders only when `getClerkUserRole(user.publicMetadata) === 'admin'` returns true. The check uses the same helper that gates the admin link in the user menu and the admin route — `frontend/src/components/auth/role.ts`. Anonymous users (no Clerk session) and User-role users see no verdict UI at any point in the gameplay flow.

**Value.** Locked decision: admin-only voting. Hiding the surface entirely (not "show-and-disable") avoids advertising a feature non-admins cannot use. Backend middleware is the source of truth — frontend hiding is cosmetic, but cosmetic gating is the difference between a clean UX and a confusing one. Mirrors AS-10 from the Phase 6 admin-link convention.

**Verification.** Vitest tests render `GamePage` with three Clerk hook stubs: `signedOut`, `signedIn role=user`, `signedIn role=admin`. Only the admin case finds the verdict buttons in the DOM. Per-render DOM-presence assertions, not just click-disabled assertions.

## FB-02: Verdict surface appears on completion AND on skip, with completion / skip variants

**Rule.** For an admin user, the verdict surface renders in two places inside `GamePage.tsx`. The two surfaces share a single `<VerdictSurface>` component but render *different button sets* — the post-grill resolution diverged the two flows further than the original "same buttons, different visual weight" model.

**1. Completion overlay** (when `isSolved` flips true and the existing overlay shows). **Variant: `completion`**.
- Two verdict buttons rendered inside the overlay alongside the existing "Play Again" / "Home" navigation:
  - **"Good puzzle"** → `submitVerdict({ value: 'up', outcome: 'solved', ... })`.
  - **"Bad puzzle"** → `submitVerdict({ value: 'down', outcome: 'solved', ... })`.
- The admin can also navigate forward (Play Again / Home) without voting — silence is a valid "no opinion" vote.
- Visual weight: prominent. The verdict prompt is the natural "rate before moving on" beat.

**2. Skip modal** (when an admin clicks the explicit Skip button — added in this slice; see FB-11). **Variant: `skip`**.
- The skip button does NOT immediately PUT skipped status. It opens a modal with three buttons:
  - **"Cancel"** → close the modal, return to the puzzle. No API calls. Timer keeps running.
  - **"I hate this"** → `PUT /api/puzzles/{id}/status` with `"skipped"` AND `submitVerdict({ value: 'down', outcome: 'skipped', ... })`, run in parallel. Close modal, navigate to curation picker (or `/`).
  - **"Just skip"** → `PUT /api/puzzles/{id}/status` with `"skipped"` only. No verdict row written. Close modal, navigate forward.
- No upvote-on-skip path. Rationale: an admin who liked the puzzle but bailed mid-attempt is rare and weakly-signaled; the high-value signal on skip is "this puzzle is bad," and that's the only verdict the surface offers.
- Visual weight: de-emphasized chrome (smaller padding, quieter prompt copy, secondary-button styling on the action buttons), keeping the modal pattern from BRAND_GUIDELINES §5.6 but with a more compact card. The Cancel button is a Ghost variant — it's the dismissal action, not the primary intent.

**Component shape.** `<VerdictSurface>` takes:
- `variant: 'completion' | 'skip'` — controls the rendered button row (set + labels + click handlers) AND the chrome de-emphasis tokens.
- `outcome: 'solved' | 'skipped'` — flows through to the verdict row's `outcome` field. Always `'solved'` when `variant === 'completion'`; always `'skipped'` when `variant === 'skip'` — they're parallel constants.
- `puzzleId`, `gridSize`, `mode`, `playTimeMs` — payload for the verdict service.
- `onDismiss?: () => void` — called when Cancel (skip variant) closes the modal without action; the parent restores the playable view.
- `onAfterVerdict?: () => void` — called after a successful verdict submission OR a "Just skip" (the parent navigates forward).

**Value.** Resolved post-second-grill: the skip flow is *user-action-and-verdict* combined into a single decision point. Cancel keeps the admin on the puzzle if they regret clicking Skip; the two destructive options are explicit about what's happening (skip alone vs skip-plus-cull). Avoids the "modal whack-a-mole" of skip → status PUT → another modal → vote.

**Verification.** Vitest tests:
- Complete a puzzle as admin → completion-variant verdict surface visible inside the completion overlay. Click "Good puzzle" → service called with `value: 'up', outcome: 'solved'`. Click "Bad puzzle" → service called with `value: 'down', outcome: 'solved'`. Click Play Again or Home without voting → no service call.
- Click the Skip button as admin → skip-variant modal opens with three buttons. Click Cancel → modal closes, no API calls, puzzle still playable. Click "I hate this" → both `updatePuzzleStatus(..., 'skipped')` and `submitVerdict({ value: 'down', outcome: 'skipped' })` called; navigation forward. Click "Just skip" → only `updatePuzzleStatus(..., 'skipped')` called; navigation forward.
- The same `<VerdictSurface>` component is used in both places, verified by import-graph or by mounting both variants in one test file.

## FB-03: Button labels and verdict values

**Rule.** Button labels are user-readable English; the underlying API values are the technical `up` / `down` strings. The mapping by variant:

| Variant | Button | Label | API call |
|---|---|---|---|
| `completion` | up | "Good puzzle" | `submitVerdict({ value: 'up', outcome: 'solved', ... })` |
| `completion` | down | "Bad puzzle" | `submitVerdict({ value: 'down', outcome: 'solved', ... })` |
| `skip` | cancel | "Cancel" | (none — dismiss only) |
| `skip` | down | "I hate this" | `updatePuzzleStatus + submitVerdict({ value: 'down', outcome: 'skipped' })` (parallel) |
| `skip` | none | "Just skip" | `updatePuzzleStatus(..., 'skipped')` only |

Component does not surface `up` / `down` strings in the DOM — only the user-readable labels. The schema retains `up` / `down` for forward compatibility (when public-rater voting lands, the surface label stays the same; the API value is what matters).

**Value.** Wording is unambiguous in context. "Good puzzle" / "Bad puzzle" reads cleanly during a curation session. "I hate this" / "Just skip" is intentionally informal — it's an internal admin tool, not a player-facing surface, and the casual language matches the curator's mental state ("ugh, this one's broken").

**Verification.** Vitest tests assert exact button text via `getByRole('button', { name: ... })` per row above. Service mocks assert the correct payload shape per click.

## FB-03: The two buttons are labelled "Good puzzle" and "Bad puzzle"

**Rule.** The button labels are user-readable English: "Good puzzle" maps to `value: "up"`, "Bad puzzle" maps to `value: "down"`. The component does not surface the raw `up` / `down` values to the user.

**Value.** "Good" / "Bad" is unambiguous. "Upvote" / "Downvote" is correct domain language for the API but reads as social-media jargon in a single-admin curation context. The schema retains `up` / `down` for forward compatibility (when public-rater voting lands, the surface label stays the same; the API value is what matters).

**Verification.** Visual snapshot or DOM-text assertion in the `VerdictSurface` test file. Click "Good puzzle" → service called with `value: "up"`. Click "Bad puzzle" → service called with `value: "down"`.

## FB-04: Submission posts to `PUT /api/admin/puzzles/{id}/verdict`

**Rule.** The `submitVerdict` service helper in `frontend/src/services/verdictService.ts` calls `apiPut` against `/api/admin/puzzles/{puzzleId}/verdict` with `?size={gridSize}&mode={mode}` and JSON body `{ value, playTimeMs, outcome, clientVersion }`. `clientVersion` is read from `import.meta.env.VITE_GIT_SHA` and defaults to `"dev"` in local dev.

**Value.** One service entry point, one call site, one URL string. Future changes (e.g., switching from PUT to POST, adding a header) happen in one file.

**Verification.** Vitest test for `submitVerdict`: stubs `fetch`, asserts URL, query params, method (`PUT`), `Content-Type: application/json`, body shape.

## FB-05: 401 / 403 on submission is silently swallowed

**Rule.** If the verdict POST returns 401 or 403, the service catches the `ApiError` and returns successfully without surfacing an error to the user. All other non-2xx responses (400, 404, 5xx) propagate as `ApiError` and are surfaced as a retry-able error in the UI.

**Value.** Defense in depth. The role check that rendered the surface should have prevented this — but if the admin's role was revoked mid-session, the buttons may briefly remain visible before the next role refresh. A 401 / 403 in that window should not produce a scary error toast; it should silently disappear (and the next page load will hide the buttons entirely).

**Verification.** Vitest test: stub `fetch` returning 401 → `submitVerdict` resolves without throwing. Stub 403 → same. Stub 500 → throws `ApiError`.

## FB-06: Submission state machine — `idle` → `submitting` → `done` | `error`

**Rule.** `<VerdictSurface>` exposes four internal states:

- `idle` — both buttons enabled, no message.
- `submitting` — both buttons disabled (`disabled={true}`), no spinner this phase.
- `done` — buttons replaced by "Thanks — recorded." text. No further interaction.
- `error` — buttons replaced by "Couldn't save your verdict." plus a single "Retry" button that re-invokes `submitVerdict` with the previously-clicked value.

State machine is monotonic on success: once `done`, the surface does not re-enter `idle` for the same puzzle in the same render.

**Value.** Predictable UI. No race condition where a slow first click and an impatient second click submit twice (the disabled-buttons-during-submitting guard handles it).

**Verification.** Vitest test: click button → state advances to `submitting` synchronously, both buttons are `disabled`. Resolve the stubbed call → state advances to `done`, "Thanks — recorded." rendered. Reject the stubbed call → state advances to `error`, retry button rendered.

## FB-07: Submitted-set cache prevents repeat surface for the same puzzle in the same session

**Rule.** On successful submission, `<VerdictSurface>` adds the `puzzleId` to a `Set` persisted in `sessionStorage` under key `reign:verdict:submitted`. On mount, the surface checks the set; if the current `puzzleId` is in it, the surface renders nothing (returns null).

**Value.** Best-effort UX guard against the buttons re-appearing if the same admin re-completes a re-served puzzle in the same tab. Uses `sessionStorage` (cleared on tab close) rather than `localStorage` so a long-running admin who reviews many puzzles eventually gets a fresh slate.

**Verification.** Vitest test: stub `sessionStorage`, submit a verdict → key contains the puzzle ID. Re-render the surface with the same `puzzleId` → returns null (no buttons in DOM).

## FB-08: `playTimeMs` is sourced from `useTimer.elapsed` at the moment of action

**Rule.** The frontend captures `useTimer.elapsed` (in seconds) at the moment the user reaches the verdict surface — the same timer value the completion overlay uses for its display. The value is converted to milliseconds (`elapsed * 1000`) before being passed to `submitVerdict`.

**Value.** Single source for the player's on-task time. No risk of the verdict's `playTimeMs` disagreeing with the displayed completion time. Reuses existing pause / resume logic for visibility-change events.

**Verification.** Vitest test: render `GameBoard` with a stubbed timer at `elapsed=42` seconds, complete the puzzle, click "Good puzzle" → service called with `playTimeMs: 42000`.

## FB-09: Verdict submission is non-blocking on the gameplay flow

**Rule.** A failed verdict submission does not block the play-again / home navigation. The user can dismiss the error and continue. The completion overlay's "Play Again" and "Home" buttons remain enabled regardless of the verdict surface's state.

**Value.** Verdicts are an admin-curation side concern, not a play-loop concern. A verdict 5xx must not feel like the puzzle "didn't count."

**Verification.** Vitest test: stub the verdict POST to fail with 500 → verdict surface enters `error` state → "Play Again" button still calls the existing `onPlayAgain` handler and clears the overlay.

## FB-10: No verdict surface for non-admins, no error if a non-admin somehow reaches the route

**Rule.** Anonymous and User-role users never see verdict buttons. They never see "you cannot vote" messaging — the surface is invisible, not disabled. The gameplay flow (completion overlay, skip-to-home) is identical for non-admins as before this phase.

**Value.** Backend middleware is the source of truth. Frontend hiding is cosmetic. Non-admins do not need to know a curation feature exists.

**Verification.** Vitest test: render `GameBoard` with `signedOut` and `signedIn role=user` Clerk stubs — no DOM nodes match `text="Good puzzle"` or `text="Bad puzzle"` or `text="Skip puzzle"`. The completion overlay otherwise renders identically (Play Again + Home).

## FB-11: Explicit Skip button on GamePage (admin-only)

**Rule.** GamePage shows a **Skip puzzle** button in the bottom action row alongside Undo / Redo / Reset, visible only when `getClerkUserRole(user.publicMetadata) === 'admin'`. The button is rendered with the **Ghost** button variant (transparent background, muted text, no shadow) — Skip is a deliberate, rare action; visual de-emphasis distinguishes it from Undo/Redo/Reset (frequent, lightweight) and keeps the button row from competing visually with the grid.

Click behavior: opens the skip modal (FB-02 §2). The button click does NOT directly call `updatePuzzleStatus` — that happens inside the modal, after the admin confirms via "I hate this" or "Just skip." Cancel returns to the puzzle with no API calls.

**Value.** Replaces the implicit-skip-on-mode-switch behavior with an explicit click. Admins curating need to express "I'm bailing on this puzzle" cleanly; an implicit skip on navigation is unreliable (closing the tab, navigating to /admin, etc. all leave the puzzle in an ambiguous state). Per CLAUDE.md Roles model, only admin-role users see the button at all — regular User-role players have no need to skip explicitly because the curation flow is admin-only this phase.

R-7-03 (next slice) reworks IndexedDB storage so each pool has its own in-progress slot — until then, switching pools still implicitly skips, but the explicit Skip click is the new canonical way to mark a puzzle skipped.

**Verification.** Vitest tests:
- `signedIn role=admin` → "Skip puzzle" button visible in the bottom action row (using `getByRole('button', { name: 'Skip puzzle' })`).
- `signedOut` and `signedIn role=user` → no "Skip puzzle" button in DOM.
- Click as admin → skip modal opens (assert one of the three skip-variant buttons is rendered: e.g. `getByRole('button', { name: 'Cancel' })`).
- Click does NOT call `updatePuzzleStatus` (verified by service mock assertion: zero calls until "I hate this" or "Just skip" is clicked).
