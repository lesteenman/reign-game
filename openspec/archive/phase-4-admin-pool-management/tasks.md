# Phase 4: Implementation Tasks

## Milestones

```
Milestone A (Backend Config)     → Config repository, admin endpoints (GET/PUT/POST)
Milestone B (Replenish Refactor) → Dynamic config-driven replenish
Milestone C (Frontend)           → Admin page, config editing, replenish controls
Milestone D (Seed + Integration) → LocalStack seed, full flow verification
```

## Dependency Layers

```
Layer 0: T-400 (Config repository) + T-404 (LocalStack seed) — parallel
    ↓
Layer 1: T-401 (Admin pool handler) + T-402 (Config handlers) — parallel
    ↓
Layer 2: T-403 (Replenish refactor) + T-405 (Route registration)
    ↓
Layer 3: T-406 (Admin API service) + T-408 (Admin nav link) + T-409 (Route registration) — parallel
    ↓
Layer 4: T-407 (Admin page + config form)
    ↓
Layer 5: T-410 (Integration test)
```

## Status

| Task | Title | Milestone | Status |
|------|-------|-----------|--------|
| T-400 | Config repository methods | A | [x] |
| T-401 | Admin pool handler (GET /admin/pool) | A | [x] |
| T-402 | Admin config handlers (PUT + POST) | A | [x] |
| T-403 | Replenish refactor (dynamic config) | B | [x] |
| T-404 | LocalStack seed: CONFIG items | D | [x] |
| T-405 | Route registration (main.go) | B | [x] |
| T-406 | Frontend admin API service | C | [x] |
| T-407 | Frontend admin page + config form | C | [x] |
| T-408 | Frontend admin nav link (PageShell) | C | [x] |
| T-409 | Frontend route registration (App.tsx) | C | [x] |
| T-410 | Integration test | D | [x] |

## Tasks

### Milestone A: Backend — Config Repository + Endpoints

#### T-400: Config Repository Methods

- **Roadmap:** R-050
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-10)
- **Work:**
  - Add ConfigRecord struct to `internal/repository/puzzle.go`
  - Add GetItem to DynamoDBAPI interface
  - Implement GetAllConfigs, GetConfig, PutConfig, CreateConfig
  - CreateConfig uses `attribute_not_exists(PK)` condition for conflict detection
  - TDD: all methods with mock DynamoDB
- **Acceptance:** BE-10 tests pass.
- **Commit after completion.**

#### T-401: Admin Pool Handler

- **Roadmap:** R-051
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-11)
- **Work:**
  - Create `internal/handler/admin_pool.go`
  - GET /admin/pool: fetch all configs, count ready for enabled combos, merge response
  - Define ConfigAndCountRepo interface
  - TDD: enabled/disabled combos, empty list, error cases
- **Acceptance:** BE-11 tests pass.
- **Depends on:** T-400
- **Commit after completion.**

#### T-402: Admin Config Handlers

- **Roadmap:** R-052, R-053
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-12)
- **Work:**
  - Create `internal/handler/admin_config.go`
  - PUT /admin/config/{size}/{mode}: validate, check exists, update
  - POST /admin/config: validate, create with conflict check
  - Reuse validation rules from ParseGenerateParams
  - TDD: valid/invalid inputs, 404, 409
- **Acceptance:** BE-12 tests pass.
- **Depends on:** T-400
- **Commit after completion.**

### Milestone B: Backend — Replenish Refactor

#### T-403: Replenish Refactor

- **Roadmap:** R-054, R-055
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-13)
- **Work:**
  - Add ConfigReader interface to replenish handler
  - Replace hardcoded combos with GetAllConfigs()
  - Use per-combo threshold and generation params
  - Remove PoolThreshold constant and sizeModeCombos var
  - Keep size/mode filter support
  - TDD: dynamic configs, mixed enabled/disabled, different thresholds
- **Acceptance:** BE-13 tests pass. Existing replenish tests updated.
- **Depends on:** T-400
- **Commit after completion.**

#### T-405: Route Registration

- **Roadmap:** R-051, R-052, R-053
- **Agent:** backend-dev
- **Spec:** specs/backend.md (BE-14)
- **Work:**
  - Update newRouter() in main.go with new admin routes
  - Update ReplenishHandler call to pass ConfigReader
  - Ensure repo satisfies all required interfaces
- **Acceptance:** BE-14 — all routes reachable.
- **Depends on:** T-401, T-402, T-403
- **Commit after completion.**

### Milestone C: Frontend — Admin Page

#### T-406: Frontend Admin API Service

- **Roadmap:** R-056
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-10)
- **Work:**
  - Create `src/services/adminService.ts`
  - Implement fetchPoolStatus, updateConfig, createConfig, triggerReplenish
  - Define TypeScript types for API responses
  - TDD: mock fetch for all endpoints and status codes
- **Acceptance:** FE-10 tests pass.
- **Commit after completion.**

#### T-407: Frontend Admin Page + Config Form

- **Roadmap:** R-056
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-11, FE-12)
- **Work:**
  - Create `src/pages/AdminPage.tsx`
  - Pool table with combo rows, counts, enabled status
  - Config edit form (modal or expandable)
  - Create combo form
  - Replenish All + per-combo replenish buttons
  - Loading state, error handling, refresh after actions
  - TDD: render, interactions, API calls, error states
- **Acceptance:** FE-11, FE-12 tests pass.
- **Depends on:** T-406
- **Commit after completion.**

#### T-408: Frontend Admin Nav Link

- **Roadmap:** R-057
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-13)
- **Work:**
  - Add admin link to PageShell header (before dark mode toggle)
  - React Router Link to /admin
  - TDD: link renders, navigates correctly
- **Acceptance:** FE-13 tests pass.
- **Commit after completion.**

#### T-409: Frontend Route Registration

- **Roadmap:** R-056
- **Agent:** frontend-dev
- **Spec:** specs/frontend.md (FE-14)
- **Work:**
  - Add /admin route to App.tsx
  - Import AdminPage (lazy or direct)
- **Acceptance:** FE-14 — /admin renders AdminPage.
- **Depends on:** T-407
- **Commit after completion.**

### Milestone D: Seed + Integration

#### T-404: LocalStack Seed — CONFIG Items

- **Roadmap:** R-058
- **Agent:** devops-engineer
- **Work:**
  - Update `.localstack/init-aws.sh`
  - Add put-item commands for 5 initial CONFIG items after table creation
  - Verify configs appear with `aws dynamodb scan`
- **Acceptance:** LocalStack starts with CONFIG items seeded.
- **Commit after completion.**

#### T-410: Integration Test

- **Roadmap:** All R-050 through R-058
- **Agent:** tester
- **Work:**
  - Start LocalStack + API server
  - GET /admin/pool — verify config items + counts
  - PUT /admin/config — change a config, verify update
  - POST /admin/config — create new combo, verify 201; duplicate, verify 409
  - POST /admin/replenish — verify uses dynamic config
  - Frontend: navigate to /admin, verify pool table renders
- **Acceptance:** Full admin flow works end-to-end locally.
- **Depends on:** T-405, T-407, T-404
- **Commit after completion.**

## Execution Summary

| Layer | Tasks | Agents | Parallel? |
|-------|-------|--------|-----------|
| 0 | T-400, T-404 | backend-dev, devops-engineer | Yes |
| 1 | T-401, T-402 | backend-dev | Yes |
| 2 | T-403, T-405 | backend-dev | Sequential (T-405 depends on T-401-403) |
| 3 | T-406, T-408, T-409 | frontend-dev | Yes (T-409 depends on T-407 but T-406/T-408 parallel) |
| 4 | T-407 | frontend-dev | — |
| 5 | T-410 | tester | — |
