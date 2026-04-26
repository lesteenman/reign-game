# Spec: Frontend Verdict Surface

The visible-and-functional frontend contract for the verdict UI.

## FB-01: Verdict surface renders only for `publicMetadata.role === 'admin'`

**Rule.** The verdict surface (two buttons + state machine) renders only when `getClerkUserRole(user.publicMetadata) === 'admin'` returns true. The check uses the same helper that gates the admin link in the user menu and the admin route — `frontend/src/components/auth/role.ts`. Anonymous users (no Clerk session) and User-role users see no verdict UI at any point in the gameplay flow.

**Value.** Locked decision: admin-only voting. Hiding the surface entirely (not "show-and-disable") avoids advertising a feature non-admins cannot use. Backend middleware is the source of truth — frontend hiding is cosmetic, but cosmetic gating is the difference between a clean UX and a confusing one. Mirrors AS-10 from the Phase 6 admin-link convention.

**Verification.** Vitest tests render `GamePage` with three Clerk hook stubs: `signedOut`, `signedIn role=user`, `signedIn role=admin`. Only the admin case finds the verdict buttons in the DOM. Per-render DOM-presence assertions, not just click-disabled assertions.

## FB-02: Verdict surface appears on completion AND on skip

**Rule.** For an admin user, the verdict surface renders in two places inside `GamePage.tsx`:

1. The completion overlay (when `isSolved` flips true and the existing overlay shows). The two verdict buttons render alongside the existing "Play Again" / "Home" buttons.
2. The post-skip transient state (when an admin chooses to skip a puzzle). After the existing `PUT /api/puzzles/{id}/status` with `"skipped"` succeeds, the user is held on a small "you skipped — verdict?" panel before navigating home. Non-admins skip → straight home as today.

Both surfaces use the same `<VerdictSurface>` component. The `outcome` prop is the only difference: `"solved"` vs `"skipped"`.

**Value.** Locked decision: same UI, same flow on both endpoints. The admin makes the same judgment ("is this puzzle worth keeping?") in both cases. Differentiating UIs adds work without value.

**Verification.** Vitest test: complete a puzzle as admin → verdict surface visible on overlay. Skip a puzzle as admin → verdict surface visible after status PUT. Both call `submitVerdict` with the correct `outcome` prop.

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

**Verification.** Vitest test: render `GameBoard` with `signedOut` and `signedIn role=user` Clerk stubs — no DOM nodes match `text="Good puzzle"` or `text="Bad puzzle"`. The completion overlay otherwise renders identically (Play Again + Home).
