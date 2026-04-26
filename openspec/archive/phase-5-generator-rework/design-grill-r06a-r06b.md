# Design grill — R-06A cleanup + R-06B e2e setup

Record of the design-grill pass (2026-04-22) for the two slices that close Phase 5. Each decision lists the resolution and the rationale in at most three sentences.

## Final design

R-06A grows from a frontend-only cleanup into a full-stack slice. A new public `GET /api/config/modes` endpoint drives the landing page's mode buttons from actual enabled combos, keeping free-user traffic off the admin endpoints. The backend config surface gets a proper domain-model / API-DTO split, closing KI-013 alongside KI-015. Dead string-type literals (KI-016) become a typed `as const` union. `PROJECT_STRUCTURE.md` gets a full refresh.

R-06B builds the e2e test infrastructure on top: a second LocalStack table (`puzzle-pool-e2e`) and a second backend instance on port `:5182` keep the test pool isolated from the dev pool. Playwright reorganizes under `frontend/playwright/` with `integration/` (mocked) and `e2e/` (live) subfolders to match the new project-wide test terminology. The slice ships two validating tests (Standard 5×5 play-through with undo, dynamic-modes wiring); the rest of the coverage map goes to ROADMAP.md as "R-06B follow-up."

## Decisions

### Product / behavior

**Dynamic mode buttons.** The landing page reads enabled combos from a new public `GET /api/config/modes` endpoint. The endpoint returns only `[{size, mode}]` pairs for combos with `enabled=true`. No thresholds, no ready counts, no admin data — this keeps free users strictly out of the `/api/admin/*` surface, which is about to be auth-gated under KI-009.

**Empty state.** When the endpoint returns zero combos, or the pool is empty for the chosen combo, the UI shows a friendly "no puzzles available, try again" message. The existing pool-empty UX already does this; extending it to the modes-empty case keeps the degraded surface consistent.

**R-06B minimum test scope.** Two tests ship in the slice: a Standard 5×5 play-through that exercises undo/redo inside the same flow, and a dynamic-modes test that confirms only enabled combos render. Everything else (Double 9×9 play-through, serve-lifecycle, pool-empty UI, generation-path coverage) is captured as a ROADMAP.md follow-up so the slice stays reviewable.

### Architecture

**Backend config model.** `repository.ConfigRecord` is the single domain shape mirroring the DynamoDB row. The handler layer owns three explicit DTOs — `ConfigView` for reads, `ConfigCreateRequest` for POST, `ConfigUpdateRequest` for PUT — with mapping functions at the boundary. This closes KI-013 (four redeclared config shapes) as a side effect and sets the pattern for the next retro's backend-model discussion.

**Frontend component split.** `ConfigForm` splits into `EditConfigForm` + `CreateConfigForm`, each consuming its matching DTO, sharing a `ConfigFields` child. Drops four defaulted no-op props in the edit path; each call-site passes only what it actually uses.

**E2E pool isolation.** LocalStack init creates both `puzzle-pool` and `puzzle-pool-e2e`. A new `task e2e:up` starts a second backend on `:5182` with `PUZZLE_TABLE_NAME=puzzle-pool-e2e`. The e2e Playwright project targets that second backend. `task e2e:seed` writes fixtures only to the e2e table. Dev pool is never touched by test runs.

**Fixture source.** A small Go `cmd/genfixtures` tool generates fixture puzzles with fixed seeds and writes them to `frontend/playwright/e2e/fixtures/puzzles/*.json`. The JSONs are committed; running the tool a second time produces byte-identical output. If the output format changes, re-run the tool and commit the diff.

**Playwright layout.** `frontend/e2e/` renames to `frontend/playwright/` with two subfolders: `integration/` (mocked backend, holds today's `grid-interaction.spec.ts`) and `e2e/` (live backend, new). The Playwright config defines two projects (`integration`, `e2e`) instead of one `chromium`. `frontend/playwright/README.md` documents which suite is which and when to run each.

### Terminology

**Testing vocabulary.** `GLOSSARY.md` gains a Testing section with two entries: an **end-to-end test** exercises the full stack (frontend + backend + LocalStack) and prefers real user flows, with database peeks allowed when useful. An **integration test** exercises one side of the system with other services mocked. Playwright is the tool; a Playwright test is either category depending on whether the backend is real or mocked.

## Deferred items

- **Full e2e coverage (R-06B follow-up).** Double 9×9 play-through, serve-lifecycle, pool-empty UI, generation-path tests. Tracked in ROADMAP.md Phase 10+.
- **Backend domain/DTO convention sweep.** The next retro picks up whether the R-06A pattern becomes a project-wide convention (and whether a CLAUDE.md lesson captures it). Retro-prep memo is in auto-memory.
- **KI-009 admin auth.** Out of scope here. The new public `/api/config/modes` endpoint is one of the things that makes the KI-009 auth-gate safer to turn on — free users no longer need admin data.

## Constraints & assumptions

- The backend's `PUZZLE_TABLE_NAME` env already switches the target table; the e2e isolation needs no backend code changes.
- `task dev:up` and `task e2e:up` run separate backend instances simultaneously. Ports `:5181` (dev) and `:5182` (e2e) must both be free.
- The Go fixture generator produces deterministic output against fixed seeds, the same guarantee R-063 / R-067c already rely on for the Step 7 gate.
- R-06B assumes `task e2e:seed` ran before Playwright. Human-facing flow is documented in `frontend/playwright/README.md`; CI is out of scope for this slice.
