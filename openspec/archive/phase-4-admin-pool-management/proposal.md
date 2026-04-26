# Phase 4: Admin Pool Management

## What

Admin UI and API endpoints for viewing pool status and configuring generation settings per size+mode combo. Config items live in the existing `puzzle-pool` DynamoDB table. The replenish handler switches from hardcoded combos to dynamic config-driven discovery.

## Why

Phase 3 hardcodes combo lists, generation parameters, and pool thresholds. Adding or tuning combos requires code changes and redeployment. An admin page lets us adjust generation settings, enable/disable combos (e.g., Double Queens via KI-007), and monitor pool levels without touching code.

## Scope

- **R-050** -- DynamoDB CONFIG items in `puzzle-pool` table (PK=`CONFIG`, SK=`{size}#{mode}`)
- **R-051** -- `GET /admin/pool` endpoint: merged config + ready counts per combo
- **R-052** -- `PUT /admin/config/{size}/{mode}` endpoint: update one combo's config
- **R-053** -- `POST /admin/config` endpoint: create a new combo
- **R-054** -- Refactor replenish handler: read CONFIG items instead of hardcoded list, use per-combo threshold and generation params
- **R-055** -- Replenish filter: optional `?size=X&mode=Y` query params for per-combo replenish
- **R-056** -- Frontend: `/admin` route with pool table (counts per combo), config editing (modal/expandable), replenish controls (global + per-combo)
- **R-057** -- Frontend: admin link in PageShell header
- **R-058** -- LocalStack seed: initial CONFIG items for local dev

## Not in Scope

- DELETE config endpoint (use `enabled=false` instead)
- Auth gate on admin routes (deferred to identity system)
- Rich pool details (status breakdown, generation history) -- future phase
- Verdict endpoint + UI (Phase 5)
- Puzzle replay (Phase 6)

## Implementation Milestones

- **A: Backend -- Config Repository + Endpoints** -- CONFIG item CRUD in repository, GET/PUT/POST handlers, admin pool endpoint
- **B: Backend -- Replenish Refactor** -- Dynamic combo discovery, per-combo config, replenish filter
- **C: Frontend -- Admin Page** -- Admin route, pool table, config editing, replenish controls, header link
- **D: LocalStack Seed + Integration** -- Seed CONFIG items, verify full flow

## References

- ROADMAP.md: R-060 through R-062 (to be updated with refined items)
- design-grill-summary.md (this directory)
- Phase 3 design: openspec/archive/phase-3-puzzle-pool/
